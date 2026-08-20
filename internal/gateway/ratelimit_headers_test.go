package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/internal/ratelimit"
)

func rlCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, w
}

// TestRateLimitHeadersWritten 覆盖 U7:
// 令牌桶一直在算 remaining,但调用方全部用 `_` 丢弃,全站零限流头。
// openai-python / openai-node 靠这些头做预测性退避。
func TestRateLimitHeadersWritten(t *testing.T) {
	c, w := rlCtx()
	setRPMHeaders(c, ratelimit.Result{Allowed: true, Limit: 60, Remaining: 42})
	setTPMHeaders(c, ratelimit.Result{Allowed: true, Limit: 100000, Remaining: 98000})

	cases := map[string]string{
		"x-ratelimit-limit-requests":     "60",
		"x-ratelimit-remaining-requests": "42",
		"x-ratelimit-limit-tokens":       "100000",
		"x-ratelimit-remaining-tokens":   "98000",
	}
	for k, want := range cases {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestNoHeadersWhenUnlimited(t *testing.T) {
	c, w := rlCtx()
	setRPMHeaders(c, ratelimit.Result{Allowed: true}) // Limit=0 表示不限流

	if got := w.Header().Get("x-ratelimit-limit-requests"); got != "" {
		t.Errorf("不限流时不该写头: %q", got)
	}
}

func TestFormatResetUsesDurationStyle(t *testing.T) {
	cases := map[time.Duration]string{
		0:                       "0s",
		1500 * time.Millisecond: "1.5s",
		90 * time.Second:        "1m30s",
	}
	for d, want := range cases {
		if got := formatReset(d); got != want {
			t.Errorf("formatReset(%v) = %q, want %q", d, got, want)
		}
	}
}

// TestRejectRateLimitedSetsRetryAfter 覆盖 429 缺 Retry-After:
// SDK 只能退回固定退避,拿不到服务端知道的确切等待时间。
func TestRejectRateLimitedSetsRetryAfter(t *testing.T) {
	c, w := rlCtx()
	rejectRateLimited(c, "rate_limit_exceeded", "慢一点",
		ratelimit.Result{Limit: 60, Remaining: 0, ResetAfter: 12 * time.Second})

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "12" {
		t.Errorf("Retry-After = %q, want 12", got)
	}
	e := decodeErr(t, w.Body.Bytes())
	if e["type"] != "rate_limit_error" {
		t.Errorf("type = %v", e["type"])
	}
}

func TestRetryAfterAtLeastOneSecond(t *testing.T) {
	c, w := rlCtx()
	rejectRateLimited(c, "rate_limit_exceeded", "慢一点", ratelimit.Result{ResetAfter: 0})

	if got := w.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1(不能是 0,否则客户端立刻重试打死自己)", got)
	}
}

// TestDegradedIsVisible 覆盖 U27:
// Redis 出错时保持放行是刻意的(限流器不该是网关单点),
// 但必须可见 —— 此前是静默放行,日志里一个字都没有。
func TestDegradedIsVisible(t *testing.T) {
	c, w := rlCtx()
	noteDegraded(c, ratelimit.Result{Allowed: true, Degraded: true}, "rpm")

	if got := w.Header().Get("X-Gateway-Ratelimit-Degraded"); got != "rpm" {
		t.Errorf("降级头 = %q", got)
	}
}

func TestNotDegradedNoHeader(t *testing.T) {
	c, w := rlCtx()
	noteDegraded(c, ratelimit.Result{Allowed: true}, "rpm")

	if got := w.Header().Get("X-Gateway-Ratelimit-Degraded"); got != "" {
		t.Errorf("正常时不该写降级头: %q", got)
	}
}
