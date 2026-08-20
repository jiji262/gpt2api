// vision.go —— chat 的图片输入(vision)。
//
// 上游 chatgpt.com 的附件通道本身是通的:图生图已经在用同一套
// UploadFile → asset_pointer → metadata.attachments 的流程(见 internal/gateway/images.go)。
// 这里把它接到 text 通路上,让 /v1/chat/completions 的 image_url part 真正生效。
//
// ⚠ 默认关闭。text 通路 + attachments 这个组合**没有 HAR 抓包实证** ——
// 它按两条已验证通路的交集推导而来。上游对"客户端类型"的判定非常敏感,
// 形状不对会触发 silent rejection(表现为"有事件但正文为空")。
// 关闭时 image_url part 仍然走 Batch 1 的明确 400,不会静默丢图。
// 开启方式与验证步骤见 docs/spec/vision.md。
package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
	"github.com/jiji262/gpt2api/pkg/logger"
	"github.com/jiji262/gpt2api/pkg/safefetch"
)

// 单张输入图上限与单请求张数上限,与图生图路径保持一致。
const (
	maxVisionImageBytes = 20 * 1024 * 1024
	maxVisionImages     = 4
)

// visionEnabled 读取 gateway.vision_enabled 设置。未注入 Settings 时视为关闭。
func (h *Handler) visionEnabled() bool {
	if h == nil || h.Settings == nil {
		return false
	}
	s, ok := h.Settings.(interface{ GatewayVisionEnabled() bool })
	return ok && s.GatewayVisionEnabled()
}

// attachImages 把请求里的 image_url part 上传到上游,并挂到对应消息上。
//
// 返回的 messages 与入参 upstreamMsgs 一一对应(顺序不变),
// 只是带图的那几条多了 Attachments。任何一张图失败都直接报错:
// 少一张图的回答是错的回答,静默降级比失败更糟。
func (h *Handler) attachImages(
	ctx context.Context, cli *chatgpt.Client,
	reqMsgs []RequestMessage, upstreamMsgs []chatgpt.ChatMessage,
) error {
	total := 0
	for _, m := range reqMsgs {
		total += len(m.Content.ImageSources)
	}
	if total == 0 {
		return nil
	}
	if total > maxVisionImages {
		return fmt.Errorf("单次请求最多 %d 张图片,收到 %d 张", maxVisionImages, total)
	}

	for i := range reqMsgs {
		srcs := reqMsgs[i].Content.ImageSources
		if len(srcs) == 0 {
			continue
		}
		atts := make([]*chatgpt.UploadedFile, 0, len(srcs))
		for j, src := range srcs {
			data, name, err := decodeImageSource(ctx, src)
			if err != nil {
				return fmt.Errorf("messages[%d] 第 %d 张图: %w", i, j+1, err)
			}
			up, err := cli.UploadFile(ctx, data, name)
			if err != nil {
				return fmt.Errorf("messages[%d] 第 %d 张图上传失败: %w", i, j+1, err)
			}
			atts = append(atts, up)
		}
		upstreamMsgs[i].Attachments = atts
		logger.L().Info("vision 附件已挂载",
			zap.Int("message_index", i), zap.Int("count", len(atts)))
	}
	return nil
}

// decodeImageSource 把一个 image_url 的 url 值解成字节。
// 支持 data:<mime>;base64,xxx 与 http(s):// 两种形态。
func decodeImageSource(ctx context.Context, src string) ([]byte, string, error) {
	src = strings.TrimSpace(src)
	switch {
	case strings.HasPrefix(src, "data:"):
		return decodeDataURL(src)
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		return fetchRemoteImageBytes(ctx, src)
	case src == "":
		return nil, "", errors.New("url 为空")
	default:
		// 裸 base64 是常见的手写错误,给一句能直接照做的提示。
		return nil, "", errors.New(`url 必须是 data:image/...;base64,... 或 http(s):// 形式`)
	}
}

func decodeDataURL(src string) ([]byte, string, error) {
	i := strings.Index(src, ",")
	if i < 0 {
		return nil, "", errors.New("data URL 缺少逗号分隔符")
	}
	header, payload := src[5:i], src[i+1:]
	if !strings.Contains(header, "base64") {
		return nil, "", errors.New("data URL 必须是 base64 编码")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", fmt.Errorf("base64 解码失败: %w", err)
	}
	if len(data) == 0 {
		return nil, "", errors.New("图片内容为空")
	}
	if len(data) > maxVisionImageBytes {
		return nil, "", fmt.Errorf("图片超过 %dMB 上限", maxVisionImageBytes/1024/1024)
	}
	mime := strings.SplitN(header, ";", 2)[0]
	return data, "image" + extFromMime(mime), nil
}

// fetchRemoteImageBytes 拉取远程图片。
//
// 走 safefetch 而不是 http.DefaultClient:这是"按调用方给的 URL 取图",
// 不该带任何账号凭据、不该走账号代理,更不该能打到内网 ——
// 裸 DefaultClient 会跟随重定向,一个正常的 https 外链可以 302 到
// 169.254.169.254,而错误文案原样回传就成了内网探测 oracle。
func fetchRemoteImageBytes(ctx context.Context, url string) ([]byte, string, error) {
	data, ct, err := safefetch.Get(ctx, url, maxVisionImageBytes, 30*time.Second)
	if err != nil {
		return nil, "", err
	}
	return data, "image" + extFromMime(ct), nil
}

func extFromMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0])) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}
