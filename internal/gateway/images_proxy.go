// images_proxy.go —— 图片返回防盗链代理。
//
// 背景:chatgpt.com 返回给我们的图片下载 URL 有两种:
//  1. file-service 直出:https://files.oaiusercontent.com/...(签名直链,15 分钟有效,浏览器可直接访问)
//  2. sediment / estuary:https://chatgpt.com/backend-api/estuary/content?...
//     这种 URL **必须带 Authorization: Bearer <AT>** 才能下载,
//     直接把它塞进 <img src> 返回给前端,浏览器 100% 403。
//
// 方案:后端不再把上游 URL 直接暴露给客户端,改成生成一个自家签名 URL:
//
//	GET /p/img/<task_id>/<idx>?exp=<unix_ms>&sig=<hex>
//
// 请求到达时,后端:
//  1. 校验 exp 未过期 + sig 匹配(HMAC-SHA256,进程级随机 secret);
//  2. 用 DAO 按 task_id 查任务,找到 file_ids[idx] / account_id / conversation_id;
//  3. 用账号 AT + deviceID + proxy 构造一个 chatgpt.Client;
//  4. 调 ImageDownloadURL 拿当前有效的上游签名 URL;
//  5. 调 FetchImage 把字节拉下来,按 Content-Type 原样写回给浏览器。
//
// 这样前端只看见自家 host 的 URL,不再受防盗链 / Authorization 困扰;
// 上游签名 URL 过期也不怕,每次访问都现取。
package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/jiji262/gpt2api/internal/image"
	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
	"github.com/jiji262/gpt2api/pkg/logger"
)

// imageUpscaleCache 进程级单例 LRU,用于缓存「原图 → 4K/2K PNG」的放大结果。
// 首次请求某张图的 4K 会花费一次 decode + Catmull-Rom + png encode(约 0.5~1.5s),
// 之后同一条代理 URL 的请求毫秒级命中,不会重复计算。
//
// 放大不会写回 image_tasks / file system —— 所有放大字节都只存在于当前进程的
// LRU 里,服务重启即销毁,保证磁盘占用为 0。
var imageUpscaleCache = image.NewUpscaleCache(0, 0)

// ImageAccountResolver 按账号 ID 解出构造 chatgpt client 所需的敏感字段。
// 由 main.go 注入。接口里不直接依赖 account 包,保持本层解耦。
type ImageAccountResolver interface {
	AuthToken(ctx context.Context, accountID uint64) (at, deviceID, cookies string, err error)
	ProxyURL(ctx context.Context, accountID uint64) string
}

// imageProxySecret 进程级随机密钥,用于 HMAC 签名图片 URL。
// 进程重启后旧的签名 URL 全部失效,这是故意的(防止长期有效的 URL 泄漏)。
var imageProxySecret []byte

func init() {
	imageProxySecret = make([]byte, 32)
	if _, err := rand.Read(imageProxySecret); err != nil {
		for i := range imageProxySecret {
			imageProxySecret[i] = byte(i*31 + 7)
		}
	}
}

// ImageProxyTTL 单条签名 URL 的默认有效期(24h,够前端离线展示一段时间)。
const ImageProxyTTL = 24 * time.Hour

// BuildImageProxyURL 生成代理 URL。返回绝对 path(不含 host),调用方可以直接拼或交给前端同 origin 使用。
//
// 默认 ttl=24h。前端展示一张历史图片,最多走一次上游获取 bytes,之后浏览器缓存即可。
func BuildImageProxyURL(taskID string, idx int, ttl time.Duration) string {
	if ttl <= 0 {
		ttl = ImageProxyTTL
	}
	expMs := time.Now().Add(ttl).UnixMilli()
	sig := computeImgSig(taskID, idx, expMs)
	return fmt.Sprintf("/p/img/%s/%d?exp=%d&sig=%s", taskID, idx, expMs, sig)
}

func computeImgSig(taskID string, idx int, expMs int64) string {
	mac := hmac.New(sha256.New, imageProxySecret)
	fmt.Fprintf(mac, "%s|%d|%d", taskID, idx, expMs)
	return hex.EncodeToString(mac.Sum(nil))[:24]
}

func verifyImgSig(taskID string, idx int, expMs int64, sig string) bool {
	if expMs < time.Now().UnixMilli() {
		return false
	}
	want := computeImgSig(taskID, idx, expMs)
	return hmac.Equal([]byte(sig), []byte(want))
}

// ImageProxy 按签名代理下载上游图片。无需 API Key,只靠 URL 签名校验。
func (h *ImagesHandler) ImageProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	idxStr := c.Param("idx")
	expStr := c.Query("exp")
	sig := c.Query("sig")

	if taskID == "" || idxStr == "" || expStr == "" || sig == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 || idx > 64 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	expMs, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !verifyImgSig(taskID, idx, expMs, sig) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if h.DAO == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	t, err := h.DAO.Get(c.Request.Context(), taskID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	fids := t.DecodeFileIDs()
	if idx >= len(fids) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// 按需放大:若 task 上打了 upscale 标记,先走进程内 LRU,命中则直接返回。
	// 无论走本地还是回源,命中 cache 的情况统一处理。未命中再拉原图。
	scale := image.ValidateUpscale(t.Upscale)
	cacheKey := ""
	if scale != "" {
		cacheKey = fmt.Sprintf("%s|%d|%s", taskID, idx, scale)
		if data, ctCache, ok := imageUpscaleCache.Get(cacheKey); ok {
			c.Header("Cache-Control", "private, max-age=3600")
			c.Header("X-Upscale", scale+";cache=hit")
			c.Data(http.StatusOK, ctCache, data)
			return
		}
	}

	// ═══ 快路径:生图成功时字节已落盘,直接读本地文件 ═══
	// 完全不回源 chatgpt,对话被 hide/删都无所谓。
	if t.LocalStored {
		body, ct, ok := readLocalImage(taskID, idx, t.LocalContentType)
		if ok {
			h.serveImageBytes(c, body, ct, scale, cacheKey)
			return
		}
		// DB 标记 local_stored=1 但文件读不到 —— 磁盘丢文件的罕见情况,
		// 降级到回源(此时 conversation 可能已 hide,回源多半也 404,但至少不 panic)。
		logger.L().Warn("image proxy local_stored=1 but file missing, falling back to remote",
			zap.String("task_id", taskID), zap.Int("idx", idx))
	}

	// ═══ 慢路径:回源 chatgpt 取签名 URL + 拉字节 ═══
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	body, ct, err := h.fetchRemoteImage(ctx, t.AccountID, t.ConversationID, fids[idx])
	if err != nil {
		logger.L().Warn("image proxy remote fetch",
			zap.Error(err), zap.String("task_id", taskID))
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}

	h.serveImageBytes(c, body, ct, scale, cacheKey)
}

// fetchRemoteImage 回源 chatgpt.com 取一张图的字节。
// ref 可能是 "sed:xxxx" 或裸 file id,由上游 ImageDownloadURL 自行区分。
func (h *ImagesHandler) fetchRemoteImage(
	ctx context.Context, accountID uint64, convID, ref string,
) ([]byte, string, error) {
	if accountID == 0 || h.ImageAccResolver == nil {
		return nil, "", errors.New("图片账号解析器未注入,无法回源")
	}
	at, deviceID, cookies, err := h.ImageAccResolver.AuthToken(ctx, accountID)
	if err != nil {
		return nil, "", fmt.Errorf("解析账号: %w", err)
	}
	cli, err := chatgpt.New(chatgpt.Options{
		AuthToken: at,
		DeviceID:  deviceID,
		ProxyURL:  h.ImageAccResolver.ProxyURL(ctx, accountID),
		Cookies:   cookies,
		Timeout:   h.upstreamTimeout(),
	})
	if err != nil {
		return nil, "", fmt.Errorf("构造上游 client: %w", err)
	}
	signedURL, err := cli.ImageDownloadURL(ctx, convID, ref)
	if err != nil {
		return nil, "", fmt.Errorf("取下载 URL: %w", err)
	}
	body, ct, err := cli.FetchImage(ctx, signedURL, 16*1024*1024)
	if err != nil {
		return nil, "", fmt.Errorf("拉取字节: %w", err)
	}
	if ct == "" {
		ct = "image/png"
	}
	return body, ct, nil
}

// loadImageBytes 取一张已生成图片的原始字节。本地落盘优先,读不到再回源。
// b64_json 响应用它,不必再让客户端多跑一次 HTTP。
func (h *ImagesHandler) loadImageBytes(ctx context.Context, taskID string, idx int) ([]byte, error) {
	if h.DAO == nil {
		return nil, errors.New("图片任务存储未初始化")
	}
	t, err := h.DAO.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	fids := t.DecodeFileIDs()
	if idx >= len(fids) {
		return nil, fmt.Errorf("图片下标 %d 越界", idx)
	}
	if t.LocalStored {
		if body, _, ok := readLocalImage(taskID, idx, t.LocalContentType); ok {
			return body, nil
		}
	}
	body, _, err := h.fetchRemoteImage(ctx, t.AccountID, t.ConversationID, fids[idx])
	return body, err
}

// readLocalImage 读取本地已落盘的生图字节。
// 先按 DB 里存的 LocalContentType 拼路径;读不到再用 ExistsAny 扫所有扩展名兜底
// (历史任务或 CT 丢失时有用)。返回 (body, content_type, ok)。
func readLocalImage(taskID string, idx int, storedCT string) ([]byte, string, bool) {
	p := image.LocalPath(image.DefaultLocalRoot, taskID, idx, storedCT)
	data, err := os.ReadFile(p)
	if err == nil {
		ct := storedCT
		if ct == "" {
			ct = http.DetectContentType(data)
		}
		return data, ct, true
	}
	if p2, ok := image.ExistsAny(image.DefaultLocalRoot, taskID, idx); ok {
		if d2, err2 := os.ReadFile(p2); err2 == nil {
			return d2, http.DetectContentType(d2), true
		}
	}
	return nil, "", false
}

// serveImageBytes 统一处理"拿到 body 之后的返回流程":
// upscale 命中 LRU → 返回;没命中就 decode/放大/写 LRU;无放大则原样返回。
// 这样本地和远端两条路径末尾代码一致,避免重复。
func (h *ImagesHandler) serveImageBytes(
	c *gin.Context, body []byte, ct, scale, cacheKey string,
) {
	if scale == "" {
		c.Header("Cache-Control", "private, max-age=1800")
		c.Data(http.StatusOK, ct, body)
		return
	}
	// 并发闸:避免 4K 请求风暴把 CPU 打满影响生图主流程
	imageUpscaleCache.Acquire()
	upBytes, upCT, err := image.DoUpscale(body, scale)
	imageUpscaleCache.Release()
	if err != nil {
		logger.L().Warn("image proxy upscale",
			zap.Error(err), zap.String("scale", scale))
		c.Header("Cache-Control", "private, max-age=1800")
		c.Header("X-Upscale", scale+";err")
		c.Data(http.StatusOK, ct, body)
		return
	}
	if upCT != "" {
		ct = upCT
	}
	if len(upBytes) > 0 {
		body = upBytes
		imageUpscaleCache.Put(cacheKey, body, ct)
		c.Header("X-Upscale", scale+";cache=miss")
	} else {
		// DoUpscale 也可能因"原图长边已足够大"而直接返回原字节
		c.Header("X-Upscale", scale+";noop")
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, ct, body)
}
