// Package gateway 实现 OpenAI 兼容的 /v1/* 入口。
//
// 职责:
//  1. 鉴权(API Key,IP/模型白名单)
//  2. 查模型 → 预扣积分
//  3. 通过调度器拿账号 Lease
//  4. 转译请求体 → 调用 chatgpt.com 上游
//  5. 转译响应(流式 or 聚合) → OpenAI 协议
//  6. 结算(真实 tokens) / 失败退款 / 释放账号锁 / 更新风控状态
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jiji262/gpt2api/internal/apikey"
	"github.com/jiji262/gpt2api/internal/billing"
	modelpkg "github.com/jiji262/gpt2api/internal/model"
	"github.com/jiji262/gpt2api/internal/ratelimit"
	"github.com/jiji262/gpt2api/internal/scheduler"
	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
	"github.com/jiji262/gpt2api/internal/usage"
	"github.com/jiji262/gpt2api/internal/user"
	"github.com/jiji262/gpt2api/pkg/logger"
	"github.com/jiji262/gpt2api/pkg/oaierr"
)

// Handler 聚合网关需要的所有依赖。
type Handler struct {
	Models    *modelpkg.Registry
	Keys      *apikey.Service
	Billing   *billing.Engine
	Scheduler *scheduler.Scheduler
	Groups    *user.GroupCache
	Limiter   *ratelimit.Limiter
	Usage     *usage.Logger
	AccSvc    interface {
		DecryptCookies(ctx context.Context, accountID uint64) (string, error)
	}
	// Images 可选:若挂载,chat/completions 里指定图像模型会自动转派。
	Images *ImagesHandler

	// Settings 可选:若注入则在构造上游 client 时应用动态超时。
	Settings interface {
		GatewayUpstreamTimeoutSec() int
		GatewaySSEReadTimeoutSec() int
	}
}

// sseReadTimeout 返回两次 SSE 事件之间的最大间隔。未注入时回退 120s。
//
// 此前 Settings 接口声明了这个方法却没有任何调用点,
// gateway.sse_read_timeout_sec 是纯死配置:parseSSE 侧的看门狗已经实现了,
// 但网关从没把配置值传下去,后台改这个设置项毫无效果。
func (h *Handler) sseReadTimeout() time.Duration {
	if h.Settings != nil {
		if n := h.Settings.GatewaySSEReadTimeoutSec(); n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 120 * time.Second
}

// upstreamTimeout 返回当前应使用的上游非流式超时。未注入时回退 60s。
func (h *Handler) upstreamTimeout() time.Duration {
	if h.Settings != nil {
		if n := h.Settings.GatewayUpstreamTimeoutSec(); n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 60 * time.Second
}

// upstreamSlugFallbacks 是"品牌名 → 已抓包实证的灰度 slug"的兜底映射。
//
// 背景:chatgpt.com 的 /f/conversation payload 里 model 字段不是品牌名
// (gpt-5 / gpt-4o),而是灰度构建版本号(例如 gpt-5-3)。浏览器打开页面时从
// /backend-api/models 拿到真实 slug,发请求时原样回传。后台直接发 "gpt-5"
// 会被上游判为非标准客户端,下发一条 is_visually_hidden_from_conversation=true
// 的空 system message 静默拒绝 —— 表现出来就是"没输出"。
//
// 真源是 models.upstream_model_slug 这一列,本表只在该列仍是裸品牌名时兜底。
// 新增条目前必须有 HAR 抓包实证,不要凭猜测往里加。
var upstreamSlugFallbacks = map[string]string{
	// HAR 抓包(2026-04,paid 账号,Edge 143)实证
	"gpt-5": "gpt-5-3",
}

// resolveUpstreamSlug 解析一个模型最终发给上游的 slug。
//
// chat 与 image 两条路径此前不一致:chat 会过这层映射,image 直接用
// m.UpstreamModelSlug —— 同一个后台配置项在两条路径上行为不同,
// 排查"某个模型只有生图不出内容"时会完全找错方向。现在统一走这里。
func resolveUpstreamSlug(configured string) string {
	if v, ok := upstreamSlugFallbacks[configured]; ok {
		return v
	}
	return configured
}

// rateScope 决定限流按 key 还是按 user 分桶。
//
// 必须与"额度从哪来"一致:ak.RPM/ak.TPM 为 0 时额度落到用户分组上,
// 此时仍按 key 分桶的话,同一用户建 N 把 key 就拿到 N 倍分组限额。
//
// ⚠ RPM 与 TPM 必须各算各的 fromGroup。取两者的或会有两种错法:
//   - RPM 来自 key、TPM 来自分组时,chat 走 u: 桶而 images 走 k: 桶,
//     同一把 key 拿到两个独立的 RPM 桶,实际额度翻倍
//   - capacity 是逐请求传进 Lua 的,同用户的小 key 会把共享桶钳到自己的容量,
//     把大 key 一起打成 429
func rateScope(ak *apikey.APIKey, fromGroup bool) ratelimit.Scope {
	return ratelimit.Scope{KeyID: ak.ID, UserID: ak.UserID, ByUser: fromGroup}
}

// roughEstimateTokens 估算 messages prompt tokens(无 tiktoken,简单 len/4)。
func roughEstimateTokens(msgs []chatgpt.ChatMessage) int {
	n := 0
	for _, m := range msgs {
		n += (len(m.Content) + 3) / 4
		n += 4 // role/overhead
	}
	return n + 2
}

// renderer 决定"拿到上游流之后按哪套协议输出"。
//
// /v1/chat/completions 与 /v1/responses 共用鉴权、模型解析、计费、调度、
// 上游调用的全部逻辑,只有最后的报文形状不同 —— 抽出这层是为了避免
// 把整条管线复制一遍(那样两边的计费和退款迟早会走偏)。
type renderer interface {
	// Render 输出响应并给出结论。rc.Stream 为真时输出 SSE。
	//
	// 只有这一层需要分叉:两套协议的请求级错误都用同一个 OpenAI 错误信封,
	// 管线里的 openAIError* 调用点无需区分。
	Render(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome
}

// chatRenderer 输出 OpenAI Chat Completions 协议。
type chatRenderer struct{ h *Handler }

func (r chatRenderer) Render(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome {
	if rc.Stream {
		return r.h.streamOpenAI(c, rc, stream)
	}
	return r.h.collectOpenAI(c, rc, stream)
}

// ChatCompletions 是 POST /v1/chat/completions 入口。
func (h *Handler) ChatCompletions(c *gin.Context) {
	ak, ok := apikey.FromCtx(c)
	if !ok {
		openAIError(c, http.StatusUnauthorized, "missing_api_key", "缺少 API Key")
		return
	}

	var req ChatCompletionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		param, msg := bindErrorMessage(err)
		openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeInvalidRequestError, param, msg)
		return
	}
	// binding:"required" 对空 slice 不生效,messages:[] 会一路跑到上游才以 502 收场,
	// 白白消耗 PoW / 账号 lease,还会触发 SDK 的两次重试(U17)。
	if len(req.Messages) == 0 {
		openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeInvalidRequestError, "messages",
			"messages 不能为空数组,至少需要一条消息")
		return
	}

	// 参数三档分流:上游做不到又被明确要求的直接 400(而不是静默吞掉,
	// 让调用方误以为模型不会用工具/不会输出 JSON);无害的记进响应头。
	verdict := validateChatRequestWithVision(&req, h.visionEnabled())
	if verdict.RejectParam != "" {
		writeUnsupportedParam(c, verdict.RejectParam, verdict.RejectMessage)
		return
	}
	setIgnoredParamsHeader(c, verdict.Ignored)

	h.runChat(c, ak, &req, chatRenderer{h})
}

// runChat 是 chat 与 responses 共用的主管线:模型解析 → 限流 → 预扣 →
// 调度账号 → 调上游 → 交给 renderer 输出 → 结算或退款。
func (h *Handler) runChat(c *gin.Context, ak *apikey.APIKey, req *ChatCompletionsRequest, rd renderer) {
	startAt := time.Now()
	refID := uuid.NewString()

	// 请求全生命周期的上下文,用于最终写 usage_logs。
	rec := &usage.Log{
		UserID:    ak.UserID,
		KeyID:     ak.ID,
		RequestID: refID,
		Type:      usage.TypeChat,
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

	// 1) 白名单 + 模型
	if !ak.ModelAllowed(req.Model) {
		fail("model_not_allowed")
		openAIError(c, http.StatusForbidden, "model_not_allowed",
			fmt.Sprintf("当前 API Key 无权调用模型 %q", req.Model))
		return
	}
	m, err := h.Models.BySlug(c.Request.Context(), req.Model)
	if err != nil || !m.Enabled {
		fail("model_not_found")
		// 官方对未知模型返回 404 + not_found_error,不是 400。SDK 与中转层按此分支
		// 决定"换渠道"而不是"改参数"。
		openAIErrorParam(c, http.StatusNotFound, oaierr.CodeModelNotFound, "model",
			fmt.Sprintf("模型 %q 不存在或已下架", req.Model))
		return
	}
	// Chat 入口收到图像模型时,转派给图像分支(便于客户端只用 /v1/chat/completions)。
	if m.Type == modelpkg.TypeImage {
		// 图像分支只会输出 Chat Completions 报文。走 /v1/responses 进来时
		// 必须在扣费前拦住,否则客户端会收到 object=chat.completion 的响应
		// (或带 [DONE] 的 chat chunk 流),Responses 解析器直接报错 ——
		// 而钱已经扣了、图已经生成了。
		if _, isChat := rd.(chatRenderer); !isChat {
			fail("model_not_supported_here")
			openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeUnsupportedValue, "model",
				fmt.Sprintf("模型 %q 是图像模型,/v1/responses 不支持图像生成。"+
					"请改用 /v1/images/generations,或在 /v1/chat/completions 上使用该模型", req.Model))
			return
		}
		if h.Images == nil {
			fail("image_not_wired")
			openAIError(c, http.StatusNotImplemented, "image_not_wired",
				"图片生成能力未开启,请联系管理员")
			return
		}
		// 借用当前已鉴权/白名单通过的 ak + 模型,走图像流程并以 OpenAI chat 响应格式返回。
		h.Images.handleChatAsImage(c, rec, ak, m, req, startAt)
		return
	}
	rec.ModelID = m.ID

	// 2) 分组倍率 + RPM/TPM
	ratio := 1.0
	rpmCap, tpmCap := ak.RPM, ak.TPM
	rpmFromGroup, tpmFromGroup := false, false
	if h.Groups != nil {
		if g, err := h.Groups.OfUser(c.Request.Context(), ak.UserID); err == nil && g != nil {
			ratio = g.Ratio
			if rpmCap == 0 {
				rpmCap, rpmFromGroup = g.RPMLimit, g.RPMLimit > 0
			}
			if tpmCap == 0 {
				tpmCap, tpmFromGroup = g.TPMLimit, g.TPMLimit > 0
			}
		}
	}
	rpmScope := rateScope(ak, rpmFromGroup)
	tpmScope := rateScope(ak, tpmFromGroup)

	// 2a) RPM
	if h.Limiter != nil {
		r := h.Limiter.AllowRPM(c.Request.Context(), rpmScope, rpmCap)
		noteDegraded(c, r, "rpm")
		setRPMHeaders(c, r)
		if !r.Allowed {
			fail("rate_limit_rpm")
			rejectRateLimited(c, oaierr.CodeRateLimitExceeded,
				"触发每分钟请求数限制 (RPM),请稍后再试", r)
			return
		}
	}

	// 3) 预扣(按 max_tokens 或 2048 估算)
	upstreamMsgs := req.upstreamMessages()
	promptTokens := roughEstimateTokens(upstreamMsgs)
	estTokens := req.MaxTokens
	if estTokens <= 0 {
		estTokens = 2048
	}
	estCost := billing.EstimateChat(m, promptTokens, req.MaxTokens, ratio)

	// 2b) TPM(按估算 tokens 预扣,结算时按差额 adjust)
	tpmAllowed := false
	if h.Limiter != nil {
		r := h.Limiter.AllowTPM(c.Request.Context(), tpmScope, tpmCap, int64(promptTokens+estTokens))
		noteDegraded(c, r, "tpm")
		setTPMHeaders(c, r)
		// Degraded 表示 Redis 出错、本次是兜底放行,桶里并没有真的扣掉。
		tpmAllowed = r.Allowed && !r.Degraded && r.Limit > 0
		if !r.Allowed {
			fail("rate_limit_tpm")
			rejectRateLimited(c, oaierr.CodeRateLimitExceeded,
				"触发每分钟 Token 限制 (TPM),请稍后再试", r)
			return
		}
	}

	if err := h.Billing.PreDeduct(c.Request.Context(), ak.UserID, ak.ID, estCost, refID, "chat prepay"); err != nil {
		if errors.Is(err, billing.ErrInsufficient) {
			fail("insufficient_balance")
			// 状态码保留 402:官方用 429+insufficient_quota,但 429 会让 SDK 反复退避重试
			// 一个永远不会自愈的状态。type 用官方字面量 insufficient_quota,
			// 让按 type 分支的中间层仍能正确识别。
			openAIErrorTyped(c, http.StatusPaymentRequired, "insufficient_quota",
				oaierr.CodeInsufficientQuota, "", "积分不足,请前往「账单与充值」充值后再试")
			return
		}
		fail("billing_error")
		openAIError(c, http.StatusInternalServerError, "billing_error", "计费异常:"+err.Error())
		return
	}

	// tpmHeld 是本次实际预扣掉的 TPM 额度。Redis 降级时并没有真扣,
	// 此时补差会凭空修改恢复后新建的桶,所以要区分。
	tpmHeld := int64(0)
	if tpmAllowed {
		tpmHeld = int64(promptTokens + estTokens)
	}

	refunded := false
	refund := func(code string) {
		fail(code)
		if refunded {
			return
		}
		refunded = true
		_ = h.Billing.Refund(context.Background(), ak.UserID, ak.ID, estCost, refID, "chat refund")
		// TPM 也要还。入口按 max_tokens(或默认 2048)预扣,失败路径只退积分
		// 不还 token 的话,一分钟内连续几次"账号池空"就能把用户的 TPM 烧干,
		// 而这期间真实产出是 0。
		if h.Limiter != nil && tpmHeld > 0 {
			h.Limiter.AdjustTPM(context.Background(), tpmScope, tpmCap, -tpmHeld)
		}
	}

	// 4) 调度账号
	lease, err := h.Scheduler.Dispatch(c.Request.Context(), modelpkg.TypeChat)
	if err != nil {
		refund("no_account_available")
		openAIError(c, http.StatusServiceUnavailable, "no_account_available", "账号池暂无可用账号,请稍后重试")
		return
	}
	rec.AccountID = lease.Account.ID
	defer func() { _ = lease.Release(context.Background()) }()

	// 5) 构造上游 client
	cookies, _ := h.AccSvc.DecryptCookies(c.Request.Context(), lease.Account.ID)
	cli, err := chatgpt.New(chatgpt.Options{
		AuthToken: lease.AuthToken,
		DeviceID:  lease.DeviceID,
		SessionID: lease.SessionID,
		ProxyURL:  lease.ProxyURL,
		Cookies:   cookies,
		Timeout:   h.upstreamTimeout(),
	})
	if err != nil {
		refund("upstream_init_error")
		openAIError(c, http.StatusInternalServerError, "upstream_init_error", "上游客户端初始化失败:"+err.Error())
		return
	}

	upstreamModel := m.UpstreamModelSlug
	if upstreamModel == "" {
		upstreamModel = "auto"
	}
	// Model slug 兜底映射:chatgpt.com 后端识别的是"灰度构建版本号",不是
	// 通用品牌名。HAR 抓包(2026-04 paid 账号)显示浏览器实际发送的 slug:
	//   品牌名 "GPT-5"   →  真实 slug "gpt-5-3"
	//   品牌名 "GPT-5 Thinking" → "gpt-5-t-3" (待证实)
	// 直接发裸的 "gpt-5" 会被上游识别为非标准客户端,下发一条
	// is_visually_hidden_from_conversation=true 的空 system message(silent
	// rejection)。这里做一次自动改写,避免运维每次灰度版本号变动都要改表。
	// 管理员若在 models.upstream_model_slug 直接填了带版本号的 slug(如
	// "gpt-5-3"),本映射是 no-op。
	upstreamModel = resolveUpstreamSlug(upstreamModel)

	// 对齐 Python 参考实现(gen_image.py,已验证可用)的真实顺序:
	//   (a) Bootstrap GET /                  —— 拿 __cf_bm / oai-did / _cfuvid cookie
	//   (b) sentinel/chat-requirements       —— 拿 chat_token + proofofwork 描述
	//   (c) f/conversation/prepare           —— 带 chat_token(!) + proof_token,拿 conduit_token
	//   (d) f/conversation                   —— 带 chat_token + proof_token + conduit_token 发 SSE
	//
	// Python 参考实现 gen_image.py 的 prepare_fconversation 明确要 chat_token,
	// 且不带 sentinel header 会让 prepare 返回空 conduit_token。

	// (a) Bootstrap
	bootCtx, cancelBoot := context.WithTimeout(c.Request.Context(), 15*time.Second)
	_ = cli.Bootstrap(bootCtx)
	cancelBoot()

	// (b) chat-requirements —— 优先走新两步协议(prepare + finalize),solver 未配置
	// 或失败时会自动回退到单步老接口(V2 内部实现)。
	reqCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cr, err := cli.ChatRequirementsV2(reqCtx)
	if err != nil {
		h.handleUpstreamErr(c, lease, err, func() { refund("upstream_error") })
		return
	}

	// POW(异步,5s 超时)
	var proofToken string
	if cr.Proofofwork.Required {
		proofCtx, cancelProof := context.WithTimeout(c.Request.Context(), 5*time.Second)
		proofCh := make(chan string, 1)
		go func() { proofCh <- cr.SolveProof("") }()
		select {
		case <-proofCtx.Done():
			cancelProof()
			h.Scheduler.MarkWarned(c.Request.Context(), lease.Account.ID)
			refund("pow_timeout")
			openAIError(c, http.StatusServiceUnavailable, "pow_timeout",
				"上游风控(PoW)未在规定时间内完成,请重试")
			return
		case proofToken = <-proofCh:
			cancelProof()
		}
		if proofToken == "" {
			h.Scheduler.MarkWarned(c.Request.Context(), lease.Account.ID)
			refund("pow_failed")
			openAIError(c, http.StatusServiceUnavailable, "pow_failed",
				"上游风控(PoW)校验失败,请稍后重试")
			return
		}
	}
	// Turnstile 在新账号 / 新 device 场景几乎必现,但它实际上是"建议",
	// 大多数情况下直接继续发 /conversation 也能被上游接受。这里只打 warn
	// 日志,不阻断(参考 gen_image.py / chat2api 的通用做法)。
	if cr.Turnstile.Required {
		logger.L().Warn("chat turnstile required, continue anyway",
			zap.Uint64("account_id", lease.Account.ID))
	}

	// 免费账号(persona=chatgpt-freeaccount)对高级模型(如 gpt-5)会静默不生成,
	// SSE 只会下发一条 hidden system preamble 就结束。chatgpt.com 浏览器端对免费账号
	// 实际发的 model 就是 "auto",由上游自己选。我们强制降级,避免"哑巴失败"。
	if cr.IsFreeAccount() && upstreamModel != "auto" {
		logger.L().Warn("free account requesting premium model, downgrade to auto",
			zap.Uint64("account_id", lease.Account.ID),
			zap.String("requested_model", upstreamModel))
		upstreamModel = "auto"
	}

	// vision:把 image_url part 上传成上游附件。关闭时这里是 no-op,
	// image_url 在入口就已经被明确 400 掉了。
	if h.visionEnabled() {
		upCtx, cancelUp := context.WithTimeout(c.Request.Context(), 90*time.Second)
		err := h.attachImages(upCtx, cli, req.Messages, upstreamMsgs)
		cancelUp()
		if err != nil {
			refund("vision_upload_failed")
			openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeInvalidRequestError,
				"messages", "图片输入处理失败:"+err.Error())
			return
		}
	}

	chatOpt := chatgpt.FChatOpts{
		UpstreamModel: upstreamModel,
		Messages:      upstreamMsgs,
		ChatToken:     cr.Token,
		ProofToken:    proofToken,
		SSETimeout:    h.sseReadTimeout(),
	}

	// (c) f/conversation/prepare(必须在 chat-requirements 之后,且带 sentinel header)
	prepCtx, cancelPrep := context.WithTimeout(c.Request.Context(), 30*time.Second)
	conduit, err := cli.PrepareFChat(prepCtx, chatOpt)
	cancelPrep()
	if err != nil {
		logger.L().Warn("f/conversation/prepare failed, continue without conduit",
			zap.Uint64("account_id", lease.Account.ID),
			zap.String("upstream_model", upstreamModel),
			zap.Error(err))
		conduit = ""
	}
	chatOpt.ConduitToken = conduit

	logger.L().Info("chat f/conversation send",
		zap.Uint64("account_id", lease.Account.ID),
		zap.String("upstream_model", upstreamModel),
		zap.Int("chat_token_len", len(cr.Token)),
		zap.Int("proof_token_len", len(proofToken)),
		zap.Int("conduit_len", len(conduit)),
		zap.Bool("turnstile_required", cr.Turnstile.Required),
		zap.String("persona", cr.Persona),
	)

	// (d) f/conversation SSE
	stream, err := cli.StreamFChat(c.Request.Context(), chatOpt)
	if err != nil {
		h.handleUpstreamErr(c, lease, err, func() { refund("upstream_error") })
		return
	}

	// 8) 转发响应
	id := "chatcmpl-" + uuid.NewString()
	rc := respCtx{
		ID:           id,
		Model:        req.Model,
		Created:      time.Now().Unix(),
		PromptTokens: promptTokens,
		MaxTokens:    req.MaxTokens,
		IncludeUsage: req.StreamOptions != nil && req.StreamOptions.IncludeUsage,
		Stream:       req.Stream,
		FreeAccount:  cr.IsFreeAccount(),
	}
	outcome := rd.Render(c, rc, stream)

	// 8b) 上游没能正常产出:全额退款,usage_logs 记失败。
	// 此前这条路径也走"成功结算",用户为一条错误提示付了钱。
	if outcome.Failure != nil {
		refund(outcome.Failure.Code)
		return
	}

	// 9) 结算
	completionTokens := outcome.CompletionTokens
	actual := billing.ComputeChatCost(m, promptTokens, completionTokens, ratio)
	if err := h.Billing.Settle(context.Background(), ak.UserID, ak.ID, estCost, actual, refID, "chat settle"); err != nil {
		logger.L().Error("billing settle", zap.Error(err), zap.String("ref", refID))
	}
	_ = h.Keys.DAO().TouchUsage(context.Background(), ak.ID, c.ClientIP(), actual)

	// 10) TPM 差额补偿。真实 tokens 通常远低于按 max_tokens(或默认 2048)
	// 做的预扣,不还回去等于长期按最坏情况限流。
	// tpmHeld 为 0 说明这次根本没扣(不限流或 Redis 降级),补差会凭空改桶。
	if h.Limiter != nil && tpmHeld > 0 {
		h.Limiter.AdjustTPM(context.Background(), tpmScope, tpmCap,
			int64(promptTokens+completionTokens)-tpmHeld)
	}

	// 11) usage 记录
	rec.Status = usage.StatusSuccess
	rec.InputTokens = promptTokens
	rec.OutputTokens = completionTokens
	rec.CreditCost = actual
}

// respCtx 承载生成响应所需的每请求上下文。
//
// 单独成结构体而不是继续加位置参数:usage / finish_reason / 流中断处理都要用到
// 同一批值,散成 6 个参数会让调用点无法阅读。
type respCtx struct {
	ID    string
	Model string
	// Created 是整条响应的时间戳,所有 chunk 必须一致。
	// 此前每帧现调 time.Now(),同一条响应的 created 会漂移,
	// 按 (id, created) 去重的侧车会把一条响应记成多条。
	Created int64
	// PromptTokens 是入口估算出的输入 token 数,响应里的 usage 必须用它,
	// 否则 prompt_tokens 恒为 0,所有按 usage 记账的侧车都会算错。
	PromptTokens int
	// MaxTokens > 0 时在网关侧软截断。上游没有长度上限入口,
	// 不截断的话 finish_reason 永远不会是 length,调用方无法感知超长。
	MaxTokens int
	// IncludeUsage 对应 stream_options.include_usage。
	IncludeUsage bool
	// Stream 决定 renderer 输出 SSE 还是一次性 JSON。
	Stream bool
	// FreeAccount 标记上游 persona 是 chatgpt-freeaccount,用于在"没拿到任何内容"
	// 的兜底分支给出更精准的错误消息(免费账号会被上游静默拒绝)。
	FreeAccount bool
}

// streamFailure 描述一次"响应没能正常产出"的失败。
type streamFailure struct {
	Status  int
	Code    string
	Message string
}

// streamOutcome 是转发的结论,调用方据此决定退款与 usage_logs 状态。
type streamOutcome struct {
	CompletionTokens int
	FinishReason     string // stop / length,失败时为空
	Failure          *streamFailure
}

// finishStop / finishLength 是本网关会产出的两种正常结束原因。
const (
	finishStop   = "stop"
	finishLength = "length"
)

// consumeUpstream 消费上游 SSE,把增量交给 onDelta,并给出结论。
//
// 流式与非流式的差别只在"拿到增量之后怎么办",消费逻辑完全一致,
// 所以抽出来共用:两边各写一遍是此前 finish_reason 与空输出判定不一致的根源。
func consumeUpstream(rc respCtx, stream <-chan chatgpt.SSEEvent, onDelta func(string)) streamOutcome {
	var extr deltaExtractor
	var total strings.Builder

	evCount := 0
	silentlyRejected := false
	sawFinal := false
	truncated := false
	var readErr error

	// maxChars 把 max_tokens 换算成字符数。与 roughEstimateTokens 用同一套
	// len/4 口径,保证"预扣"和"截断"看到的是同一个尺子。
	maxChars := 0
	if rc.MaxTokens > 0 {
		maxChars = rc.MaxTokens * 4
	}

	for ev := range stream {
		if ev.Err != nil {
			readErr = ev.Err
			logger.L().Warn("upstream stream err", zap.Error(ev.Err))
			break
		}
		if len(ev.Data) == 0 {
			continue
		}
		evCount++
		// 对前若干帧开 Info 级别日志,方便线上快速定位 "SSE 有事件但正文为空" 的协议级问题。
		if evCount <= 16 {
			logger.L().Info("chat sse raw", zap.Int("n", evCount),
				zap.String("event", ev.Event),
				zap.String("data", truncate(string(ev.Data), 2048)))
		}
		if !silentlyRejected && isSilentRejection(ev.Data) {
			silentlyRejected = true
		}
		delta, final, err := extr.Extract(ev.Data)
		if err != nil {
			continue
		}
		if delta != "" {
			if maxChars > 0 && total.Len()+len(delta) >= maxChars {
				if cut := maxChars - total.Len(); cut > 0 {
					delta = safeCut(delta, cut)
					total.WriteString(delta)
					onDelta(delta)
				}
				truncated = true
				break
			}
			total.WriteString(delta)
			onDelta(delta)
		}
		if final {
			sawFinal = true
			break
		}
	}
	logger.L().Info("chat sse done", zap.Int("events", evCount),
		zap.Int("content_len", total.Len()),
		zap.Bool("silently_rejected", silentlyRejected),
		zap.Bool("truncated", truncated))

	out := streamOutcome{CompletionTokens: (total.Len() + 3) / 4}
	switch {
	case readErr != nil:
		out.Failure = &streamFailure{
			Status:  http.StatusBadGateway,
			Code:    oaierr.CodeUpstreamInterrupted,
			Message: "上游连接中途中断,本次回答不完整:" + readErr.Error(),
		}
	case truncated:
		out.FinishReason = finishLength
	case total.Len() == 0:
		// 上游接受了请求却没产出任何可见文本。此前把运维提示当 assistant 正文
		// 返回 HTTP 200 并照常计费,调用方完全无法区分"模型这么回答"和"根本没答"。
		out.Failure = &streamFailure{
			Status:  http.StatusBadGateway,
			Code:    oaierr.CodeUpstreamEmptyOutput,
			Message: emptyReplyMessage(rc.FreeAccount, silentlyRejected),
		}
	case !sawFinal:
		out.Failure = &streamFailure{
			Status:  http.StatusBadGateway,
			Code:    oaierr.CodeUpstreamInterrupted,
			Message: "上游未发送结束标记,本次回答可能不完整,请重试",
		}
	default:
		out.FinishReason = finishStop
	}
	return out
}

// safeCut 在不切坏 UTF-8 字符的前提下截取前 n 字节。
func safeCut(s string, n int) string {
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// streamOpenAI 将上游 SSE 事件转为 OpenAI 风格流式响应。
func (h *Handler) streamOpenAI(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	// 先发一个 role=assistant 的空 delta(OpenAI 规范起始)
	writeChunk(w, flusher, rc, DeltaMsg{Role: "assistant"}, nil)

	out := consumeUpstream(rc, stream, func(delta string) {
		writeChunk(w, flusher, rc, DeltaMsg{Content: delta}, nil)
	})

	if out.Failure != nil {
		// 响应头已经 200 发出去了,改状态码来不及。唯一能让客户端知道出事的
		// 办法是在流里送一个 error 事件 —— 绝不能再补一个 finish_reason:"stop"
		// 把半截回答伪装成完整答案。
		fmt.Fprint(w, oaierr.SSEErrorLine(out.Failure.Status, out.Failure.Code, "", out.Failure.Message))
	} else {
		reason := out.FinishReason
		writeChunk(w, flusher, rc, DeltaMsg{}, &reason)
		if rc.IncludeUsage {
			writeUsageChunk(w, flusher, rc, out.CompletionTokens)
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}

	return out
}

// collectOpenAI 聚合上游 SSE 后一次性返回非流式响应。
func (h *Handler) collectOpenAI(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome {
	var content strings.Builder
	out := consumeUpstream(rc, stream, func(delta string) { content.WriteString(delta) })
	if out.Failure != nil {
		oaierr.Write(c, out.Failure.Status, out.Failure.Code, "", out.Failure.Message)
		return out
	}

	c.JSON(http.StatusOK, ChatCompletionResponse{
		ID:      rc.ID,
		Object:  "chat.completion",
		Created: rc.Created,
		Model:   rc.Model,
		Choices: []ChatCompletionChoice{{
			Index:        0,
			Message:      assistantMessage(content.String()),
			FinishReason: out.FinishReason,
		}},
		Usage: usageOf(rc, out.CompletionTokens),
	})
	return out
}

// usageOf 组装 usage。prompt_tokens 用入口的估算值,
// cached_tokens 恒为 0(上游不提供缓存命中信息),但字段必须在。
func usageOf(rc respCtx, completionTokens int) *ChatCompletionUsage {
	return &ChatCompletionUsage{
		PromptTokens:        rc.PromptTokens,
		CompletionTokens:    completionTokens,
		TotalTokens:         rc.PromptTokens + completionTokens,
		PromptTokensDetails: &PromptTokensDetails{CachedTokens: 0},
	}
}

// handleUpstreamErr 根据上游错误降级账号并回传 OpenAI 错误。
func (h *Handler) handleUpstreamErr(c *gin.Context, lease *scheduler.Lease, err error, refund func()) {
	var ue *chatgpt.UpstreamError
	if errors.As(err, &ue) {
		switch {
		case ue.IsRateLimited():
			h.Scheduler.MarkRateLimited(c.Request.Context(), lease.Account.ID)
		case ue.IsUnauthorized():
			h.Scheduler.MarkDead(c.Request.Context(), lease.Account.ID)
		}
		refund()
		logger.L().Error("chat upstream error",
			zap.Int("status", ue.Status),
			zap.Uint64("account_id", lease.Account.ID),
			zap.String("body", truncate(ue.Body, 1500)))
		// 上游的 401/403 是"账号池出问题了",不是调用方 key 有问题:
		// 原样透传会让 LiteLLM 之类把渠道永久拉黑。而 429/503/504 值得
		// 让调用方重试,这部分语义要保留。
		status := oaierr.StatusForUpstream(ue.Status)
		code := oaierr.CodeUpstreamError
		if status == http.StatusTooManyRequests {
			code = oaierr.CodeRateLimitExceeded
			c.Header(hdrRetryAfter, "30")
		}
		oaierr.Write(c, status, code, "",
			fmt.Sprintf("上游返回错误(HTTP %d):%s", ue.Status, truncate(ue.Body, 200)))
		return
	}
	refund()
	openAIError(c, http.StatusBadGateway, "upstream_error", "上游请求失败:"+err.Error())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// isSilentRejection 识别 ChatGPT 对免费账号/高限流账号的"静默拒绝"特征:
// 上游下发一条 author=system + is_visually_hidden_from_conversation=true
// + end_turn=true + parts=[""] 的 delta 事件,让前端看起来"什么都没发生"就终止。
// 这种 pattern 和 payload 是否合规完全无关,是上游策略层的硬门槛。
func isSilentRejection(data []byte) bool {
	s := string(data)
	// 用字符串快速判定,避免每帧都做完整 JSON 反序列化。
	// 三个字段同时出现才算,防止误判正常 assistant 消息。
	return strings.Contains(s, `"is_visually_hidden_from_conversation": true`) &&
		strings.Contains(s, `"role": "system"`) &&
		strings.Contains(s, `"end_turn": true`)
}

// emptyReplyMessage 根据账号类型和上游信号,返回给最终用户看的兜底文案。
func emptyReplyMessage(freeAccount, silentlyRejected bool) string {
	switch {
	case silentlyRejected && freeAccount:
		return "上游检测到当前账号为免费版(chatgpt-freeaccount),已静默拒绝本次请求。" +
			"请联系管理员更换 ChatGPT Plus / Team 账号后再试。"
	case silentlyRejected:
		return "上游已接受请求但静默终止对话(常见于账号被限流或触发内容审核)," +
			"请稍后重试,若仍失败请更换模型或账号。"
	case freeAccount:
		return "当前账号为 ChatGPT 免费版,上游未产出内容。请更换 Plus/Team 账号后再试。"
	default:
		return "上游未产出回答内容,可能触发了内容审核或账号被临时限流,请稍后重试。"
	}
}

func writeChunk(w io.Writer, f http.Flusher, rc respCtx, delta DeltaMsg, finish *string) {
	writeSSEChunk(w, f, ChatCompletionChunk{
		ID:      rc.ID,
		Object:  "chat.completion.chunk",
		Created: rc.Created,
		Model:   rc.Model,
		Choices: []ChatCompletionChunkChoice{{Index: 0, Delta: delta, FinishReason: finish}},
	})
}

// writeUsageChunk 发 stream_options.include_usage 约定的最后一个 chunk:
// choices 为空数组、usage 为真值。空数组不能省略,客户端按 choices 长度分支。
func writeUsageChunk(w io.Writer, f http.Flusher, rc respCtx, completionTokens int) {
	writeSSEChunk(w, f, ChatCompletionChunk{
		ID:      rc.ID,
		Object:  "chat.completion.chunk",
		Created: rc.Created,
		Model:   rc.Model,
		Choices: []ChatCompletionChunkChoice{},
		Usage:   usageOf(rc, completionTokens),
	})
}

func writeSSEChunk(w io.Writer, f http.Flusher, chunk ChatCompletionChunk) {
	b, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", b)
	if f != nil {
		f.Flush()
	}
}

// openAIError 按 OpenAI 规范返回错误。type 由状态码推导,param 为 null。
//
// 需要指向具体出错字段时用 openAIErrorParam;需要覆盖 type(例如余额不足要报
// insufficient_quota)时用 openAIErrorTyped。
func openAIError(c *gin.Context, httpStatus int, code, msg string) {
	oaierr.Write(c, httpStatus, code, "", msg)
}

// openAIErrorParam 在错误可归因到某个请求字段时使用,param 会指向该字段名。
func openAIErrorParam(c *gin.Context, httpStatus int, code, param, msg string) {
	oaierr.Write(c, httpStatus, code, param, msg)
}

// openAIErrorTyped 覆盖 error.type。
func openAIErrorTyped(c *gin.Context, httpStatus int, typ, code, param, msg string) {
	oaierr.WriteTyped(c, httpStatus, typ, code, param, msg)
}

// bindErrorMessage 把 go-playground/validator 的英文原文翻成可操作的中文,
// 并返回出错字段名(填进 error.param)。
//
// 原文形如:
//
//	Key: 'ChatCompletionsRequest.Model' Error:Field validation for 'Model' failed on the 'required' tag
//
// 直接吐给客户端既暴露内部结构体名,又不告诉用户该怎么改。
func bindErrorMessage(err error) (param, msg string) {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		return "", "请求体解析失败:" + err.Error()
	}
	f := ve[0]
	param = jsonFieldName(f.Field())
	switch f.Tag() {
	case "required":
		return param, fmt.Sprintf("缺少必填参数 %q", param)
	default:
		return param, fmt.Sprintf("参数 %q 不合法(未通过 %s 校验)", param, f.Tag())
	}
}

// jsonFieldName 把结构体字段名转成 OpenAI 协议里的 snake_case 参数名。
func jsonFieldName(structField string) string {
	var b strings.Builder
	for i, r := range structField {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
