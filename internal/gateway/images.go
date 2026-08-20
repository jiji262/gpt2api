package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jiji262/gpt2api/internal/apikey"
	"github.com/jiji262/gpt2api/internal/billing"
	"github.com/jiji262/gpt2api/internal/image"
	modelpkg "github.com/jiji262/gpt2api/internal/model"
	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
	"github.com/jiji262/gpt2api/internal/usage"
	"github.com/jiji262/gpt2api/pkg/logger"
	"github.com/jiji262/gpt2api/pkg/oaierr"
	"github.com/jiji262/gpt2api/pkg/safefetch"
)

// 单张参考图的硬上限(字节)。chatgpt.com 的 /backend-api/files 实测上限大致 20MB。
const maxReferenceImageBytes = 20 * 1024 * 1024

// 同一次请求最多携带的参考图数量。
const maxReferenceImages = 4

// chatMsg 是 OpenAI chat message 的本地别名,便于 handleChatAsImage 内部表达。
type chatMsg = chatgpt.ChatMessage

// ImagesHandler 挂载在 /v1/images/* 下的处理器。
//
// 复用 Handler 的依赖(鉴权/模型/计费/限流/usage)加上专属的 image.Runner 和 DAO。
// 路由:
//
//	POST /v1/images/generations       同步生图(默认)
//	GET  /v1/images/tasks/:id         查询历史任务(按 task_id)
type ImagesHandler struct {
	*Handler
	Runner *image.Runner
	DAO    *image.DAO
	// ImageAccResolver 可选:代理下载上游图片时用于解出账号 AT/cookies/proxy。
	// 未注入时 /p/img 路径会返回 503。
	ImageAccResolver ImageAccountResolver
}

// ImageGenRequest OpenAI 兼容入参。
//
// 对 reference_images 的扩展:OpenAI 的 /images/generations 规范没有这个字段;
// 这里加一项非标准扩展,便于 Playground / Web UI 发起"图生图"走同一条 generations 路径。
// 每一项可以是:
//   - https:// URL       直接 HTTP GET
//   - data:<mime>;base64,xxxx   dataURL
//   - 纯 base64 字符串            兼容
type ImageGenRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size"`
	Quality        string `json:"quality,omitempty"`
	Style          string `json:"style,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // url | b64_json
	User           string `json:"user,omitempty"`

	// 以下是官方 Images API 有、上游 chatgpt.com 网页版做不到的参数。
	// 声明出来不是为了实现,是为了在用户显式指定时能明确报错 ——
	// 此前它们连字段都没有,静默丢弃,用户以为生效了。
	Background        string `json:"background,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	PartialImages     *int   `json:"partial_images,omitempty"`
	Stream            bool   `json:"stream,omitempty"`
	InputFidelity     string `json:"input_fidelity,omitempty"`

	ReferenceImages []string `json:"reference_images,omitempty"` // 非标准扩展,见注释
	// Upscale 非标准扩展:控制"本服务对原图做本地高清放大"的目标档位。
	// 可选值:""(原图直出,默认)/ "2k"(长边 2560) / "4k"(长边 3840)。
	// 算法:golang.org/x/image/draw.CatmullRom(传统插值,不是 AI 超分)。
	// 生效时机:图片代理 URL 首次请求时做一次 decode+放大+PNG 编码,之后进程内
	// LRU 缓存命中毫秒级返回。仅影响 /v1/images/proxy/... 的出口字节,不改原图。
	Upscale string `json:"upscale,omitempty"`
}

// ImageGenData 单张图响应。b64_json 与 url 二选一,由 response_format 决定。
type ImageGenData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	FileID        string `json:"file_id,omitempty"` // chatgpt.com 侧原始 id(用于对账)
}

// ImageGenResponse OpenAI 兼容返回。
//
// size / quality / background / output_format 是官方响应里的回显字段,
// 客户端用它们确认"我要的和拿到的是不是一回事"。
// 刻意不带 usage:上游不返回 token 计数,编数字比缺字段更糟。
type ImageGenResponse struct {
	Created      int64          `json:"created"`
	Data         []ImageGenData `json:"data"`
	Size         string         `json:"size,omitempty"`
	Quality      string         `json:"quality,omitempty"`
	Background   string         `json:"background,omitempty"`
	OutputFormat string         `json:"output_format,omitempty"`
	TaskID       string         `json:"task_id,omitempty"`
}

// ImageGenerations POST /v1/images/generations。
func (h *ImagesHandler) ImageGenerations(c *gin.Context) {
	startAt := time.Now()
	ak, ok := apikey.FromCtx(c)
	if !ok {
		openAIError(c, http.StatusUnauthorized, "missing_api_key", "缺少 API Key")
		return
	}

	var req ImageGenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		openAIError(c, http.StatusBadRequest, "invalid_request_error", "请求参数错误:"+err.Error())
		return
	}
	if p, why := validateImageRequest(&req); p != "" {
		writeUnsupportedParam(c, p, why)
		return
	}
	applyImageDefaults(&req)
	req.Upscale = image.ValidateUpscale(req.Upscale)

	refID := uuid.NewString()
	rec := &usage.Log{
		UserID:    ak.UserID,
		KeyID:     ak.ID,
		RequestID: refID,
		Type:      usage.TypeImage,
		IP:        c.ClientIP(),
		UA:        c.Request.UserAgent(),
	}
	defer func() {
		rec.DurationMs = int(time.Since(startAt).Milliseconds())
		if rec.Status == "" {
			rec.Status = usage.StatusFailed
		}
		if h.Usage != nil {
			h.Usage.Write(rec)
		}
	}()
	fail := func(code string) { rec.Status = usage.StatusFailed; rec.ErrorCode = code }

	// 1) 模型白名单
	if !ak.ModelAllowed(req.Model) {
		fail("model_not_allowed")
		openAIError(c, http.StatusForbidden, "model_not_allowed",
			fmt.Sprintf("当前 API Key 无权调用模型 %q", req.Model))
		return
	}
	m, err := h.Models.BySlug(c.Request.Context(), req.Model)
	if err != nil || m == nil || !m.Enabled {
		fail("model_not_found")
		openAIErrorParam(c, http.StatusNotFound, oaierr.CodeModelNotFound, "model",
			fmt.Sprintf("模型 %q 不存在或已下架", req.Model))
		return
	}
	if m.Type != modelpkg.TypeImage {
		fail("model_type_mismatch")
		openAIErrorParam(c, http.StatusBadRequest, "model_type_mismatch", "model",
			fmt.Sprintf("模型 %q 不是图像模型,不能用于 /v1/images/generations", req.Model))
		return
	}
	rec.ModelID = m.ID

	// 2) 分组倍率 + RPM 限流(图像不走 TPM)
	ratio := 1.0
	rpmCap := ak.RPM
	rpmFromGroup := false
	if h.Groups != nil {
		if g, err := h.Groups.OfUser(c.Request.Context(), ak.UserID); err == nil && g != nil {
			ratio = g.Ratio
			if rpmCap == 0 {
				rpmFromGroup = g.RPMLimit > 0
				rpmCap = g.RPMLimit
			}
		}
	}
	if h.Limiter != nil {
		r := h.Limiter.AllowRPM(c.Request.Context(), rateScope(ak, rpmFromGroup), rpmCap)
		noteDegraded(c, r, "rpm")
		setRPMHeaders(c, r)
		if !r.Allowed {
			fail("rate_limit_rpm")
			rejectRateLimited(c, oaierr.CodeRateLimitExceeded,
				"触发每分钟请求数限制 (RPM),请稍后再试", r)
			return
		}
	}

	// 3) 预扣(图像按定价,est = actual)
	cost := billing.ComputeImageCost(m, req.N, ratio)
	if cost > 0 {
		if err := h.Billing.PreDeduct(c.Request.Context(), ak.UserID, ak.ID, cost, refID, "image prepay"); err != nil {
			if errors.Is(err, billing.ErrInsufficient) {
				fail("insufficient_balance")
				openAIErrorTyped(c, http.StatusPaymentRequired, "insufficient_quota",
					oaierr.CodeInsufficientQuota, "", "积分不足,请前往「账单与充值」充值后再试")
				return
			}
			fail("billing_error")
			openAIError(c, http.StatusInternalServerError, "billing_error", "计费异常:"+err.Error())
			return
		}
	}
	refunded := false
	refund := func(code string) {
		fail(code)
		if refunded || cost == 0 {
			return
		}
		refunded = true
		_ = h.Billing.Refund(context.Background(), ak.UserID, ak.ID, cost, refID, "image refund")
	}

	// 4) 落任务
	taskID := image.GenerateTaskID()
	task := &image.Task{
		TaskID:          taskID,
		UserID:          ak.UserID,
		KeyID:           ak.ID,
		ModelID:         m.ID,
		Prompt:          req.Prompt,
		N:               req.N,
		Size:            req.Size,
		Upscale:         req.Upscale,
		Status:          image.StatusDispatched,
		EstimatedCredit: cost,
	}
	if h.DAO != nil {
		if err := h.DAO.Create(c.Request.Context(), task); err != nil {
			refund("billing_error")
			openAIError(c, http.StatusInternalServerError, "internal_error", "创建任务失败:"+err.Error())
			return
		}
	}

	// 4.5) 解析 reference_images(图生图 / 图像编辑入口都走到这里)
	refs, err := decodeReferenceInputs(c.Request.Context(), req.ReferenceImages)
	if err != nil {
		refund("invalid_request_error")
		openAIError(c, http.StatusBadRequest, "invalid_reference_image", "参考图解析失败:"+err.Error())
		return
	}

	// 5) 执行(同步阻塞)
	//
	// 单请求硬上限 3 分钟:Runner 默认 per-attempt 2 分钟,留出 1 分钟给
	// 账号调度 + 签名 URL 换取等周边耗时。IMG2 已正式上线,不再做 preview_only 重试。
	runCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()

	// 带参考图时,多轮重试没什么意义(反而会重复上传参考图),只留 1 次尝试。
	maxAttempts := 2
	if len(refs) > 0 {
		maxAttempts = 1
	}

	res := h.Runner.Run(runCtx, image.RunOptions{
		TaskID:        taskID,
		UserID:        ak.UserID,
		KeyID:         ak.ID,
		ModelID:       m.ID,
		UpstreamModel: resolveUpstreamSlug(m.UpstreamModelSlug),
		Prompt:        maybeAppendClaritySuffix(req.Prompt),
		N:             req.N,
		MaxAttempts:   maxAttempts,
		References:    refs,
	})
	rec.AccountID = res.AccountID

	if res.Status != image.StatusSuccess {
		refund(ifEmpty(res.ErrorCode, "upstream_error"))
		httpStatus := http.StatusBadGateway
		if res.ErrorCode == image.ErrNoAccount {
			httpStatus = http.StatusServiceUnavailable
		}
		if res.ErrorCode == image.ErrRateLimited {
			httpStatus = http.StatusServiceUnavailable
		}
		openAIError(c, httpStatus, ifEmpty(res.ErrorCode, "upstream_error"),
			localizeImageErr(res.ErrorCode, res.ErrorMessage))
		return
	}

	// 6) 结算
	if cost > 0 {
		if err := h.Billing.Settle(context.Background(), ak.UserID, ak.ID, cost, cost, refID, "image settle"); err != nil {
			logger.L().Error("billing settle image", zap.Error(err), zap.String("ref", refID))
		}
	}
	_ = h.Keys.DAO().TouchUsage(context.Background(), ak.ID, c.ClientIP(), cost)

	// 7) usage
	rec.Status = usage.StatusSuccess
	rec.CreditCost = cost

	// 8) DAO 回写 credit_cost(Runner 已经 MarkSuccess,这里只补 credit_cost)
	if h.DAO != nil {
		_ = h.DAO.UpdateCost(c.Request.Context(), taskID, cost)
	}

	// 9) 响应:URL 统一走自家代理,防止 chatgpt.com estuary/content 防盗链
	out := newImageGenResponse(&req, time.Now().Unix(), taskID)
	for i := range res.SignedURLs {
		d := ImageGenData{}
		if i < len(res.FileIDs) {
			d.FileID = strings.TrimPrefix(res.FileIDs[i], "sed:")
		}
		if req.ResponseFormat == "b64_json" {
			body, err := h.loadImageBytes(c.Request.Context(), taskID, i)
			if err != nil {
				logger.L().Warn("b64_json 取字节失败",
					zap.Error(err), zap.String("task_id", taskID), zap.Int("idx", i))
				openAIError(c, http.StatusBadGateway, "image_fetch_failed",
					"图片已生成但读取字节失败,可改用 response_format=url 重试:"+err.Error())
				return
			}
			d.B64JSON = base64.StdEncoding.EncodeToString(body)
		} else {
			d.URL = h.imageURL(c, taskID, i)
		}
		out.Data = append(out.Data, d)
	}
	c.JSON(http.StatusOK, out)
}

// ImageTask GET /v1/images/tasks/:id。
func (h *ImagesHandler) ImageTask(c *gin.Context) {
	ak, ok := apikey.FromCtx(c)
	if !ok {
		openAIError(c, http.StatusUnauthorized, "missing_api_key", "缺少 API Key")
		return
	}
	id := c.Param("id")
	if id == "" {
		openAIError(c, http.StatusBadRequest, "invalid_request_error", "task id 不能为空")
		return
	}
	if h.DAO == nil {
		openAIError(c, http.StatusInternalServerError, "not_configured", "图片任务存储未初始化,请联系管理员")
		return
	}
	t, err := h.DAO.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, image.ErrNotFound) {
			openAIError(c, http.StatusNotFound, "not_found", "任务不存在")
			return
		}
		openAIError(c, http.StatusInternalServerError, "internal_error", "查询任务失败:"+err.Error())
		return
	}
	if t.UserID != ak.UserID {
		openAIError(c, http.StatusNotFound, "not_found", "任务不存在")
		return
	}

	urls := t.DecodeResultURLs()
	data := make([]ImageGenData, 0, len(urls))
	fileIDs := t.DecodeFileIDs()
	for i := range urls {
		d := ImageGenData{URL: h.imageURL(c, t.TaskID, i)}
		if i < len(fileIDs) {
			d.FileID = strings.TrimPrefix(fileIDs[i], "sed:")
		}
		data = append(data, d)
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":         t.TaskID,
		"status":          t.Status,
		"conversation_id": t.ConversationID,
		"created":         t.CreatedAt.Unix(),
		"finished_at":     nullableUnix(t.FinishedAt),
		"error":           t.Error,
		"credit_cost":     t.CreditCost,
		"data":            data,
	})
}

// handleChatAsImage 是 /v1/chat/completions 发现 model.type=image 时的转派点。
// 行为:
//   - 取最后一条 user message 作为 prompt
//   - 走完整图像链路(同 /v1/images/generations)
//   - 以 assistant message(含 markdown 图片链接)的 OpenAI chat 响应返回
//
// 这样前端只要调用一个端点 /v1/chat/completions,切换 model=gpt-image-2 就能出图。
func (h *ImagesHandler) handleChatAsImage(c *gin.Context, rec *usage.Log, ak *apikey.APIKey,
	m *modelpkg.Model, req *ChatCompletionsRequest, startAt time.Time) {
	rec.ModelID = m.ID
	rec.Type = usage.TypeImage

	prompt := extractLastUserPrompt(req.upstreamMessages())
	if strings.TrimSpace(prompt) == "" {
		rec.Status = usage.StatusFailed
		rec.ErrorCode = "invalid_request_error"
		openAIError(c, http.StatusBadRequest, "invalid_request_error",
			"图像模型需要用户消息作为 prompt,请检查 messages 内容")
		return
	}

	refID := uuid.NewString()

	// 倍率 + RPM
	ratio := 1.0
	rpmCap := ak.RPM
	rpmFromGroup := false
	if h.Groups != nil {
		if g, err := h.Groups.OfUser(c.Request.Context(), ak.UserID); err == nil && g != nil {
			ratio = g.Ratio
			if rpmCap == 0 {
				rpmFromGroup = g.RPMLimit > 0
				rpmCap = g.RPMLimit
			}
		}
	}
	if h.Limiter != nil {
		r := h.Limiter.AllowRPM(c.Request.Context(), rateScope(ak, rpmFromGroup), rpmCap)
		noteDegraded(c, r, "rpm")
		setRPMHeaders(c, r)
		if !r.Allowed {
			rec.Status = usage.StatusFailed
			rec.ErrorCode = "rate_limit_rpm"
			rejectRateLimited(c, oaierr.CodeRateLimitExceeded,
				"触发每分钟请求数限制 (RPM),请稍后再试", r)
			return
		}
	}

	// 预扣
	cost := billing.ComputeImageCost(m, 1, ratio)
	if cost > 0 {
		if err := h.Billing.PreDeduct(c.Request.Context(), ak.UserID, ak.ID, cost, refID, "chat->image prepay"); err != nil {
			rec.Status = usage.StatusFailed
			if errors.Is(err, billing.ErrInsufficient) {
				rec.ErrorCode = "insufficient_balance"
				openAIErrorTyped(c, http.StatusPaymentRequired, "insufficient_quota",
					oaierr.CodeInsufficientQuota, "", "积分不足,请前往「账单与充值」充值后再试")
				return
			}
			rec.ErrorCode = "billing_error"
			openAIError(c, http.StatusInternalServerError, "billing_error", "计费异常:"+err.Error())
			return
		}
	}
	refunded := false
	refund := func(code string) {
		rec.Status = usage.StatusFailed
		rec.ErrorCode = code
		if refunded || cost == 0 {
			return
		}
		refunded = true
		_ = h.Billing.Refund(context.Background(), ak.UserID, ak.ID, cost, refID, "chat->image refund")
	}

	taskID := image.GenerateTaskID()
	if h.DAO != nil {
		_ = h.DAO.Create(c.Request.Context(), &image.Task{
			TaskID:          taskID,
			UserID:          ak.UserID,
			KeyID:           ak.ID,
			ModelID:         m.ID,
			Prompt:          prompt,
			N:               1,
			Size:            "1024x1024",
			Status:          image.StatusDispatched,
			EstimatedCredit: cost,
		})
	}

	runCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()

	res := h.Runner.Run(runCtx, image.RunOptions{
		TaskID:        taskID,
		UserID:        ak.UserID,
		KeyID:         ak.ID,
		ModelID:       m.ID,
		UpstreamModel: resolveUpstreamSlug(m.UpstreamModelSlug),
		Prompt:        maybeAppendClaritySuffix(prompt),
		N:             1,
		MaxAttempts:   2,
	})
	rec.AccountID = res.AccountID

	if res.Status != image.StatusSuccess {
		refund(ifEmpty(res.ErrorCode, "upstream_error"))
		httpStatus := http.StatusBadGateway
		if res.ErrorCode == image.ErrNoAccount || res.ErrorCode == image.ErrRateLimited {
			httpStatus = http.StatusServiceUnavailable
		}
		openAIError(c, httpStatus, ifEmpty(res.ErrorCode, "upstream_error"),
			localizeImageErr(res.ErrorCode, res.ErrorMessage))
		return
	}

	if cost > 0 {
		_ = h.Billing.Settle(context.Background(), ak.UserID, ak.ID, cost, cost, refID, "chat->image settle")
	}
	_ = h.Keys.DAO().TouchUsage(context.Background(), ak.ID, c.ClientIP(), cost)
	if h.DAO != nil {
		_ = h.DAO.UpdateCost(c.Request.Context(), taskID, cost)
	}

	rec.Status = usage.StatusSuccess
	rec.CreditCost = cost
	rec.DurationMs = int(time.Since(startAt).Milliseconds())

	// 以 chat 响应返回(content 里内嵌 markdown 图片)。
	var sb strings.Builder
	for i := range res.SignedURLs {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("![generated](%s)", h.imageURL(c, taskID, i)))
	}

	rc := respCtx{
		ID:           "chatcmpl-" + uuid.NewString(),
		Model:        m.Slug,
		Created:      time.Now().Unix(),
		PromptTokens: roughEstimateTokens(req.upstreamMessages()),
		IncludeUsage: req.StreamOptions != nil && req.StreamOptions.IncludeUsage,
	}
	// 此前无视 req.Stream 一律返回非流式 JSON:客户端已经按 SSE 解析响应体,
	// 拿到一坨 JSON 直接解析失败 —— 钱扣了、图生成了、用户什么都没拿到。
	if req.Stream {
		writeImageAsChatStream(c, rc, sb.String())
		return
	}

	c.JSON(http.StatusOK, ChatCompletionResponse{
		ID:      rc.ID,
		Object:  "chat.completion",
		Created: rc.Created,
		Model:   rc.Model,
		Choices: []ChatCompletionChoice{{
			Index:        0,
			Message:      assistantMessage(sb.String()),
			FinishReason: finishStop,
		}},
		Usage: usageOf(rc, (len(sb.String())+3)/4),
	})
}

// writeImageAsChatStream 把已经生成好的图片 markdown 以 SSE 形式一次性发出。
//
// 图片是同步生成的,没有真正的增量可言;这里只是把结果包成客户端预期的
// chunk 序列,保证 stream:true 的调用方能正常收到内容而不是解析失败。
func writeImageAsChatStream(c *gin.Context, rc respCtx, content string) {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	writeChunk(w, flusher, rc, DeltaMsg{Role: "assistant"}, nil)
	writeChunk(w, flusher, rc, DeltaMsg{Content: content}, nil)
	stop := finishStop
	writeChunk(w, flusher, rc, DeltaMsg{}, &stop)
	if rc.IncludeUsage {
		writeUsageChunk(w, flusher, rc, (len(content)+3)/4)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

// extractLastUserPrompt 从 messages 中拿最后一条 user 消息的 content。
func extractLastUserPrompt(msgs []chatMsg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && strings.TrimSpace(msgs[i].Content) != "" {
			return msgs[i].Content
		}
	}
	return ""
}

// --- helpers ---

func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// localizeImageErr 把 runner 返回的英文错误码 + 原始 err.Error() 压成一段中文提示,
// 方便前端 / SDK 用户直接看懂。原始英文 message 作为后缀保留以便排障。
func localizeImageErr(code, raw string) string {
	var zh string
	switch code {
	case image.ErrNoAccount:
		zh = "账号池暂无可用账号,请稍后重试"
	case image.ErrRateLimited:
		zh = "上游风控,请稍后再试"
	case image.ErrUnknown, "":
		zh = "图片生成失败"
	case "upstream_error":
		zh = "上游返回错误"
	default:
		zh = "图片生成失败(" + code + ")"
	}
	if raw != "" && raw != code {
		return zh + ":" + raw
	}
	return zh
}

func nullableUnix(t *time.Time) int64 {
	if t == nil || t.IsZero() {
		return 0
	}
	return t.Unix()
}

// 含这些关键字时,追加中英双约束让上游出字更清楚(迁移自 gen_image.py)。
var textHintKeywords = []string{
	"文字", "对话", "台词", "旁白", "标语", "字幕", "标题", "文案",
	"招牌", "横幅", "海报文字", "弹幕", "气泡", "字体",
	"text:", "caption", "subtitle", "title:", "label", "banner", "poster text",
}

const claritySuffix = "\n\nclean readable Chinese text, prioritize text clarity over image details"

// ImageEdits 实现 POST /v1/images/edits,严格按 OpenAI 规范接 multipart/form-data。
//
// 表单字段(与 OpenAI 官方一致):
//
//	image            (file)      单张主图,必填
//	image[]          (file)      多张,可重复(2025 起官方支持)
//	mask             (file)      不支持,显式 400(上游没有 inpainting 通道)
//	prompt           (string)    必填
//	model            (string)    模型 slug,默认 gpt-image-2
//	n                (int)       默认 1
//	size             (string)    默认 1024x1024
//	response_format  (string)    url | b64_json
//	user             (string)
//
// 实际走的上游协议和 /v1/images/generations + reference_images 完全相同。
// 行为等价于"把 multipart 文件读成字节 + prompt,交给 ImageGenerations 的主流程"。
func (h *ImagesHandler) ImageEdits(c *gin.Context) {
	startAt := time.Now()
	ak, ok := apikey.FromCtx(c)
	if !ok {
		openAIError(c, http.StatusUnauthorized, "missing_api_key", "缺少 API Key")
		return
	}

	// multipart 上限:单文件 20MB * 最多 4 张 + 冗余。
	if err := c.Request.ParseMultipartForm(int64(maxReferenceImageBytes) * int64(maxReferenceImages+1)); err != nil {
		openAIError(c, http.StatusBadRequest, "invalid_request_error", "解析 multipart 失败:"+err.Error())
		return
	}

	// multipart 的字段归一成同一个请求结构,复用 generations 那套三档校验:
	// 两条路径此前各写一遍参数处理,是 size/quality 行为不一致的根源。
	req := ImageGenRequest{
		Model:          c.Request.FormValue("model"),
		Prompt:         strings.TrimSpace(c.Request.FormValue("prompt")),
		Size:           c.Request.FormValue("size"),
		Quality:        c.Request.FormValue("quality"),
		Style:          c.Request.FormValue("style"),
		ResponseFormat: c.Request.FormValue("response_format"),
		User:           c.Request.FormValue("user"),
		Background:     c.Request.FormValue("background"),
		OutputFormat:   c.Request.FormValue("output_format"),
		Moderation:     c.Request.FormValue("moderation"),
		InputFidelity:  c.Request.FormValue("input_fidelity"),
	}
	if v := c.Request.FormValue("n"); v != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeInvalidRequestError, "n",
				"n 必须是整数")
			return
		}
		req.N = parsed
	}
	if p, why := validateEditExtras(hasMaskField(c.Request.MultipartForm), req.InputFidelity); p != "" {
		writeUnsupportedParam(c, p, why)
		return
	}
	if p, why := validateImageRequest(&req); p != "" {
		writeUnsupportedParam(c, p, why)
		return
	}
	applyImageDefaults(&req)
	prompt, model, n, size := req.Prompt, req.Model, req.N, req.Size
	upscale := image.ValidateUpscale(c.Request.FormValue("upscale"))

	// 主图 + 可能的多张
	files, err := collectEditFiles(c.Request.MultipartForm)
	if err != nil {
		openAIError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if len(files) == 0 {
		openAIError(c, http.StatusBadRequest, "invalid_request_error", "至少需要上传一张 image 作为参考图")
		return
	}
	if len(files) > maxReferenceImages {
		openAIError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("最多支持 %d 张参考图", maxReferenceImages))
		return
	}
	refs := make([]image.ReferenceImage, 0, len(files))
	for _, fh := range files {
		data, err := readMultipart(fh)
		if err != nil {
			openAIError(c, http.StatusBadRequest, "invalid_reference_image",
				fmt.Sprintf("读取 %q 失败:%s", fh.Filename, err.Error()))
			return
		}
		if len(data) == 0 {
			openAIError(c, http.StatusBadRequest, "invalid_reference_image",
				fmt.Sprintf("参考图 %q 为空", fh.Filename))
			return
		}
		if len(data) > maxReferenceImageBytes {
			openAIError(c, http.StatusBadRequest, "invalid_reference_image",
				fmt.Sprintf("参考图 %q 超过 %dMB 上限", fh.Filename, maxReferenceImageBytes/1024/1024))
			return
		}
		refs = append(refs, image.ReferenceImage{Data: data, FileName: filepath.Base(fh.Filename)})
	}

	// usage 记录
	refID := uuid.NewString()
	rec := &usage.Log{
		UserID:    ak.UserID,
		KeyID:     ak.ID,
		RequestID: refID,
		Type:      usage.TypeImage,
		IP:        c.ClientIP(),
		UA:        c.Request.UserAgent(),
	}
	defer func() {
		rec.DurationMs = int(time.Since(startAt).Milliseconds())
		if rec.Status == "" {
			rec.Status = usage.StatusFailed
		}
		if h.Usage != nil {
			h.Usage.Write(rec)
		}
	}()
	fail := func(code string) { rec.Status = usage.StatusFailed; rec.ErrorCode = code }

	if !ak.ModelAllowed(model) {
		fail("model_not_allowed")
		openAIError(c, http.StatusForbidden, "model_not_allowed",
			fmt.Sprintf("当前 API Key 无权调用模型 %q", model))
		return
	}
	m, err := h.Models.BySlug(c.Request.Context(), model)
	if err != nil || m == nil || !m.Enabled {
		fail("model_not_found")
		openAIErrorParam(c, http.StatusNotFound, oaierr.CodeModelNotFound, "model",
			fmt.Sprintf("模型 %q 不存在或已下架", model))
		return
	}
	if m.Type != modelpkg.TypeImage {
		fail("model_type_mismatch")
		openAIErrorParam(c, http.StatusBadRequest, "model_type_mismatch", "model",
			fmt.Sprintf("模型 %q 不是图像模型,不能用于 /v1/images/edits", model))
		return
	}
	rec.ModelID = m.ID

	ratio := 1.0
	rpmCap := ak.RPM
	rpmFromGroup := false
	if h.Groups != nil {
		if g, err := h.Groups.OfUser(c.Request.Context(), ak.UserID); err == nil && g != nil {
			ratio = g.Ratio
			if rpmCap == 0 {
				rpmFromGroup = g.RPMLimit > 0
				rpmCap = g.RPMLimit
			}
		}
	}
	if h.Limiter != nil {
		r := h.Limiter.AllowRPM(c.Request.Context(), rateScope(ak, rpmFromGroup), rpmCap)
		noteDegraded(c, r, "rpm")
		setRPMHeaders(c, r)
		if !r.Allowed {
			fail("rate_limit_rpm")
			rejectRateLimited(c, oaierr.CodeRateLimitExceeded,
				"触发每分钟请求数限制 (RPM),请稍后再试", r)
			return
		}
	}

	cost := billing.ComputeImageCost(m, n, ratio)
	if cost > 0 {
		if err := h.Billing.PreDeduct(c.Request.Context(), ak.UserID, ak.ID, cost, refID, "image-edit prepay"); err != nil {
			if errors.Is(err, billing.ErrInsufficient) {
				fail("insufficient_balance")
				openAIErrorTyped(c, http.StatusPaymentRequired, "insufficient_quota",
					oaierr.CodeInsufficientQuota, "", "积分不足,请前往「账单与充值」充值后再试")
				return
			}
			fail("billing_error")
			openAIError(c, http.StatusInternalServerError, "billing_error", "计费异常:"+err.Error())
			return
		}
	}
	refunded := false
	refund := func(code string) {
		fail(code)
		if refunded || cost == 0 {
			return
		}
		refunded = true
		_ = h.Billing.Refund(context.Background(), ak.UserID, ak.ID, cost, refID, "image-edit refund")
	}

	taskID := image.GenerateTaskID()
	if h.DAO != nil {
		_ = h.DAO.Create(c.Request.Context(), &image.Task{
			TaskID:          taskID,
			UserID:          ak.UserID,
			KeyID:           ak.ID,
			ModelID:         m.ID,
			Prompt:          prompt,
			N:               n,
			Size:            size,
			Upscale:         upscale,
			Status:          image.StatusDispatched,
			EstimatedCredit: cost,
		})
	}

	runCtx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Minute)
	defer cancel()

	res := h.Runner.Run(runCtx, image.RunOptions{
		TaskID:        taskID,
		UserID:        ak.UserID,
		KeyID:         ak.ID,
		ModelID:       m.ID,
		UpstreamModel: resolveUpstreamSlug(m.UpstreamModelSlug),
		Prompt:        maybeAppendClaritySuffix(prompt),
		N:             n,
		MaxAttempts:   1, // 带参考图时只跑一次,避免重复上传
		References:    refs,
	})
	rec.AccountID = res.AccountID

	if res.Status != image.StatusSuccess {
		refund(ifEmpty(res.ErrorCode, "upstream_error"))
		httpStatus := http.StatusBadGateway
		if res.ErrorCode == image.ErrNoAccount || res.ErrorCode == image.ErrRateLimited {
			httpStatus = http.StatusServiceUnavailable
		}
		openAIError(c, httpStatus, ifEmpty(res.ErrorCode, "upstream_error"),
			localizeImageErr(res.ErrorCode, res.ErrorMessage))
		return
	}

	if cost > 0 {
		if err := h.Billing.Settle(context.Background(), ak.UserID, ak.ID, cost, cost, refID, "image-edit settle"); err != nil {
			logger.L().Error("billing settle image-edit", zap.Error(err), zap.String("ref", refID))
		}
	}
	_ = h.Keys.DAO().TouchUsage(context.Background(), ak.ID, c.ClientIP(), cost)

	rec.Status = usage.StatusSuccess
	rec.CreditCost = cost
	if h.DAO != nil {
		_ = h.DAO.UpdateCost(c.Request.Context(), taskID, cost)
	}

	out := newImageGenResponse(&req, time.Now().Unix(), taskID)
	for i := range res.SignedURLs {
		d := ImageGenData{}
		if i < len(res.FileIDs) {
			d.FileID = strings.TrimPrefix(res.FileIDs[i], "sed:")
		}
		if req.ResponseFormat == "b64_json" {
			body, err := h.loadImageBytes(c.Request.Context(), taskID, i)
			if err != nil {
				logger.L().Warn("b64_json 取字节失败",
					zap.Error(err), zap.String("task_id", taskID), zap.Int("idx", i))
				openAIError(c, http.StatusBadGateway, "image_fetch_failed",
					"图片已生成但读取字节失败,可改用 response_format=url 重试:"+err.Error())
				return
			}
			d.B64JSON = base64.StdEncoding.EncodeToString(body)
		} else {
			d.URL = h.imageURL(c, taskID, i)
		}
		out.Data = append(out.Data, d)
	}
	c.JSON(http.StatusOK, out)
}

// hasMaskField 判断 multipart 里是否带了 mask 文件。
func hasMaskField(form *multipart.Form) bool {
	return form != nil && len(form.File["mask"]) > 0
}

// collectEditFiles 把 multipart 里"可能作为参考图"的字段一次性收拢。
// 兼容 OpenAI 的几种写法:
//   - image      : 单文件
//   - image[]    : 多文件
//
// mask 不在收拢范围内:上游没有 inpainting 通道,把它当普通参考图会丢掉
// "只改遮罩区域"的语义还白占一个配额。入口处已用 validateEditExtras 直接 400。
func collectEditFiles(form *multipart.Form) ([]*multipart.FileHeader, error) {
	if form == nil {
		return nil, errors.New("empty multipart form")
	}
	var out []*multipart.FileHeader
	seen := map[string]bool{}
	add := func(fhs []*multipart.FileHeader) {
		for _, fh := range fhs {
			if fh == nil {
				continue
			}
			key := fh.Filename + "|" + fmt.Sprint(fh.Size)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, fh)
		}
	}
	for _, key := range []string{"image", "image[]", "images", "images[]"} {
		if fhs := form.File[key]; len(fhs) > 0 {
			add(fhs)
		}
	}
	// 也兼容 image_1 / image_2 / ... 的写法
	for k, fhs := range form.File {
		if strings.HasPrefix(k, "image_") {
			add(fhs)
		}
	}
	return out, nil
}

func readMultipart(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// decodeReferenceInputs 把 JSON 里 reference_images(url/data-url/base64 混合)下载/解码成字节。
// 超出条数上限直接报错;单张尺寸上限 maxReferenceImageBytes。
func decodeReferenceInputs(ctx context.Context, inputs []string) ([]image.ReferenceImage, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) > maxReferenceImages {
		return nil, fmt.Errorf("最多支持 %d 张参考图", maxReferenceImages)
	}
	out := make([]image.ReferenceImage, 0, len(inputs))
	for i, s := range inputs {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, fmt.Errorf("第 %d 张参考图为空", i+1)
		}
		data, name, err := fetchReferenceBytes(ctx, s)
		if err != nil {
			return nil, fmt.Errorf("第 %d 张参考图:%w", i+1, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("第 %d 张参考图解码后为空", i+1)
		}
		if len(data) > maxReferenceImageBytes {
			return nil, fmt.Errorf("第 %d 张参考图超过 %dMB 上限", i+1, maxReferenceImageBytes/1024/1024)
		}
		out = append(out, image.ReferenceImage{Data: data, FileName: name})
	}
	return out, nil
}

// fetchReferenceBytes 支持 http(s) / data URL / 裸 base64 三种输入。
func fetchReferenceBytes(ctx context.Context, s string) ([]byte, string, error) {
	low := strings.ToLower(s)
	switch {
	case strings.HasPrefix(low, "data:"):
		// data:<mime>[;base64],<payload>
		comma := strings.IndexByte(s, ',')
		if comma < 0 {
			return nil, "", errors.New("无效 data URL")
		}
		meta := s[5:comma]
		payload := s[comma+1:]
		if strings.Contains(strings.ToLower(meta), ";base64") {
			b, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				// 兼容 URL-safe
				if b2, err2 := base64.URLEncoding.DecodeString(payload); err2 == nil {
					b = b2
				} else {
					return nil, "", fmt.Errorf("base64 解码失败:%w", err)
				}
			}
			return b, "", nil
		}
		return []byte(payload), "", nil
	case strings.HasPrefix(low, "http://"), strings.HasPrefix(low, "https://"):
		// 走 safefetch:这是按调用方给的 URL 取图,必须挡住内网地址与
		// "外链 302 到内网"的绕过,错误文案也要脱敏(否则是内网探测 oracle)。
		// 15s 基本能覆盖 OSS / CDN / presigned URL。
		body, _, err := safefetch.Get(ctx, s, int64(maxReferenceImageBytes), 15*time.Second)
		if err != nil {
			return nil, "", err
		}
		name := ""
		if u, uerr := neturl.Parse(s); uerr == nil {
			name = filepath.Base(u.Path)
		}
		return body, name, nil
	default:
		// 当成裸 base64 处理
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			if b2, err2 := base64.URLEncoding.DecodeString(s); err2 == nil {
				return b2, "", nil
			}
			return nil, "", fmt.Errorf("既非 URL 也非可解析的 base64:%w", err)
		}
		return b, "", nil
	}
}

func parseIntClamp(s string, min, max int) (int, error) {
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0, err
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v, nil
}

func maybeAppendClaritySuffix(prompt string) string {
	lower := strings.ToLower(prompt)
	need := false
	for _, kw := range textHintKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			need = true
			break
		}
	}
	if !need {
		// 检测中文/英文引号内容 ≥ 2 个字
		for _, pair := range [][2]string{
			{"\"", "\""}, {"'", "'"},
			{"“", "”"}, {"‘", "’"},
			{"「", "」"}, {"『", "』"},
		} {
			if idx := strings.Index(prompt, pair[0]); idx >= 0 {
				rest := prompt[idx+len(pair[0]):]
				if end := strings.Index(rest, pair[1]); end >= 2 {
					need = true
					break
				}
			}
		}
	}
	if need && !strings.Contains(prompt, strings.TrimSpace(claritySuffix)) {
		return prompt + claritySuffix
	}
	return prompt
}
