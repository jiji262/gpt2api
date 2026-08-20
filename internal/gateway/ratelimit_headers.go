package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/jiji262/gpt2api/internal/ratelimit"
	"github.com/jiji262/gpt2api/pkg/logger"
	"github.com/jiji262/gpt2api/pkg/oaierr"
)

// 官方响应头名。openai-python / openai-node 用它们做预测性退避:
// 拿不到就只能一次次撞上 429 再指数退避,对调用方是纯粹的延迟浪费。
const (
	hdrLimitRequests     = "x-ratelimit-limit-requests"
	hdrRemainingRequests = "x-ratelimit-remaining-requests"
	hdrResetRequests     = "x-ratelimit-reset-requests"
	hdrLimitTokens       = "x-ratelimit-limit-tokens"
	hdrRemainingTokens   = "x-ratelimit-remaining-tokens"
	hdrResetTokens       = "x-ratelimit-reset-tokens"
	hdrRetryAfter        = "Retry-After"
	// hdrDegraded 在限流器不可用、本次是兜底放行时置位。
	// 没有它,Redis 故障期间的放行和"确实还有额度"在外部完全无法区分。
	hdrDegraded = "X-Gateway-Ratelimit-Degraded"
)

// setRPMHeaders 写入请求维度的限流头。
func setRPMHeaders(c *gin.Context, r ratelimit.Result) {
	if r.Limit <= 0 {
		return
	}
	c.Header(hdrLimitRequests, strconv.FormatInt(r.Limit, 10))
	c.Header(hdrRemainingRequests, strconv.FormatInt(r.Remaining, 10))
	c.Header(hdrResetRequests, formatReset(r.ResetAfter))
}

// setTPMHeaders 写入 token 维度的限流头。
func setTPMHeaders(c *gin.Context, r ratelimit.Result) {
	if r.Limit <= 0 {
		return
	}
	c.Header(hdrLimitTokens, strconv.FormatInt(r.Limit, 10))
	c.Header(hdrRemainingTokens, strconv.FormatInt(r.Remaining, 10))
	c.Header(hdrResetTokens, formatReset(r.ResetAfter))
}

// formatReset 用官方的 Go duration 风格("6m0s" / "1.5s"),不是裸秒数。
func formatReset(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	return d.Round(time.Millisecond).String()
}

// rejectRateLimited 输出 429,并带上 Retry-After 与限流头。
func rejectRateLimited(c *gin.Context, code, msg string, r ratelimit.Result) {
	retry := int(r.ResetAfter.Seconds())
	if retry < 1 {
		retry = 1
	}
	c.Header(hdrRetryAfter, strconv.Itoa(retry))
	oaierr.WriteTyped(c, http.StatusTooManyRequests, oaierr.TypeRateLimit, code, "", msg)
}

// noteDegraded 在限流器降级时打 Error 日志并置位响应头。
//
// 保持 fail-open 是刻意的:限流器不该成为整个网关的单点故障。
// 但必须让降级可见 —— 此前 Redis 报错时 RPM/TPM 静默全失效,
// 账号池会在几十秒内被打穿,而日志里一个字都没有。
func noteDegraded(c *gin.Context, r ratelimit.Result, dimension string) {
	if !r.Degraded {
		return
	}
	c.Header(hdrDegraded, dimension)
	logger.L().Error("限流器降级,本次请求已放行",
		zap.String("dimension", dimension),
		zap.String("path", c.Request.URL.Path))
}
