package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func preflight(origins []string, origin, requestHeaders string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	if origin != "" {
		c.Request.Header.Set("Origin", origin)
	}
	if requestHeaders != "" {
		c.Request.Header.Set("Access-Control-Request-Headers", requestHeaders)
	}
	CORS(origins)(c)
	return w
}

// TestPreflightEchoesRequestedHeaders 覆盖 U10:
// 浏览器里的 openai-js(Stainless 生成)每个请求都带一串 x-stainless-* 头,
// 此前允许列表只有三项,预检直接被浏览器拦掉。
func TestPreflightEchoesRequestedHeaders(t *testing.T) {
	w := preflight([]string{"*"}, "https://app.example.com",
		"authorization, content-type, x-stainless-lang, x-stainless-package-version")

	got := w.Header().Get("Access-Control-Allow-Headers")
	for _, want := range []string{"x-stainless-lang", "x-stainless-package-version", "Authorization"} {
		if !strings.Contains(got, want) {
			t.Errorf("Allow-Headers 缺 %q: %q", want, got)
		}
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("预检应回 204,实际 %d", w.Code)
	}
}

func TestPreflightWithoutRequestedHeadersUsesDefaults(t *testing.T) {
	w := preflight([]string{"*"}, "https://app.example.com", "")

	got := w.Header().Get("Access-Control-Allow-Headers")
	for _, want := range []string{"Authorization", "Content-Type", "OpenAI-Beta", "api-key"} {
		if !strings.Contains(got, want) {
			t.Errorf("默认列表缺 %q: %q", want, got)
		}
	}
}

// TestWildcardOriginDoesNotAllowCredentials 是一条安全约束:
// 配了 "*" 还回显任意 origin 并允许 credentials,
// 等于对全网开放带 Cookie 的跨站请求。
func TestWildcardOriginDoesNotAllowCredentials(t *testing.T) {
	w := preflight([]string{"*"}, "https://evil.example.com", "")

	if w.Header().Get("Access-Control-Allow-Origin") != "https://evil.example.com" {
		t.Errorf("通配符下应回显 origin: %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("通配符下不得允许 credentials,实际 %q", got)
	}
}

func TestWhitelistedOriginAllowsCredentials(t *testing.T) {
	w := preflight([]string{"https://app.example.com"}, "https://app.example.com", "")

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("白名单 origin 应允许 credentials,实际 %q", got)
	}
}

func TestNonWhitelistedOriginGetsNoAllowOrigin(t *testing.T) {
	w := preflight([]string{"https://app.example.com"}, "https://evil.example.com", "")

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("非白名单 origin 不该拿到 Allow-Origin: %q", got)
	}
}

// TestRateLimitHeadersAreExposed 保证浏览器端 SDK 真能读到限流头,
// 否则预测性退避形同虚设。
func TestRateLimitHeadersAreExposed(t *testing.T) {
	w := preflight([]string{"*"}, "https://app.example.com", "")

	got := w.Header().Get("Access-Control-Expose-Headers")
	for _, want := range []string{
		"x-ratelimit-remaining-requests",
		"x-ratelimit-remaining-tokens",
		"Retry-After",
		"X-Gateway-Ignored-Params",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Expose-Headers 缺 %q: %q", want, got)
		}
	}
}

func TestNoOriginNoCORSHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	CORS([]string{"*"})(c)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("非跨域请求不该带 CORS 头")
	}
}

// TestRecoverUsesOpenAIEnvelopeUnderV1 覆盖冒烟测试实测到的问题:
// /v1 下的 panic 此前回内部信封 {"code":50000,...},
// openai-python 会抛 JSONDecodeError 而不是 APIStatusError ——
// 调用方看到的错误与真实原因毫无关系。
func TestRecoverUsesOpenAIEnvelopeUnderV1(t *testing.T) {
	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)
	engine.Use(Recover())
	engine.POST("/v1/chat/completions", func(*gin.Context) { panic("boom") })
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	engine.ServeHTTP(w, c.Request)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	var got map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w.Body.String())
	}
	e, ok := got["error"]
	if !ok {
		t.Fatalf("缺少 error 键: %s", w.Body.String())
	}
	if e["type"] != "server_error" {
		t.Errorf("type = %v", e["type"])
	}
	for _, k := range []string{"message", "type", "param", "code"} {
		if _, ok := e[k]; !ok {
			t.Errorf("缺键 %q", k)
		}
	}
}

func TestRecoverKeepsInternalShapeOutsideV1(t *testing.T) {
	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)
	engine.Use(Recover())
	engine.POST("/api/admin/models", func(*gin.Context) { panic("boom") })
	c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/models", nil)
	engine.ServeHTTP(w, c.Request)

	var got map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if _, hasErr := got["error"]; hasErr {
		t.Error("管理端不该套 OpenAI 信封")
	}
	if got["code"] == nil {
		t.Errorf("管理端应保持内部信封: %s", w.Body.String())
	}
}
