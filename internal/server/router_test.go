package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/internal/config"
	"github.com/jiji262/gpt2api/internal/gateway"
)

func init() { gin.SetMode(gin.TestMode) }

// newTestServer 起一个**真实的** HTTP 服务,路由表与生产完全一致。
//
// 不注入任何 DB / Redis 依赖:本文件测的是路由挂载、中间件链、错误信封,
// 这些在真实请求路径上才能验证 —— 单元测试直接调 handler 是绕过了
// 路由匹配与中间件顺序的,而 U28(NoRoute 只在有 web/dist 时才注册)
// 恰恰就是这一层的 bug。
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.App.Env = "test"
	cfg.Security.CORSOrigins = []string{"*"}

	r := New(&Deps{
		Config:   cfg,
		GatewayH: &gateway.Handler{},
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, method, path string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func decodeErrBody(t *testing.T, res *http.Response) map[string]interface{} {
	t.Helper()
	var got map[string]map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("响应不是 OpenAI 错误信封: %v", err)
	}
	e, ok := got["error"]
	if !ok {
		t.Fatalf("缺少 error 键")
	}
	for _, k := range []string{"message", "type", "param", "code"} {
		if _, ok := e[k]; !ok {
			t.Errorf("缺少必需键 %q", k)
		}
	}
	return e
}

func TestHealthEndpoints(t *testing.T) {
	srv := newTestServer(t)
	for _, p := range []string{"/healthz", "/readyz"} {
		if res := get(t, srv, http.MethodGet, p, nil); res.StatusCode != http.StatusOK {
			t.Errorf("%s = %d", p, res.StatusCode)
		}
	}
}

// TestV1RequiresAuth 走完整中间件链验证鉴权拒绝的形状。
func TestV1RequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	res := get(t, srv, http.MethodPost, "/v1/chat/completions", nil)

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
	e := decodeErrBody(t, res)
	if e["type"] != "authentication_error" {
		t.Errorf("type = %v", e["type"])
	}
}

// TestUnknownV1PathIsStructured 是 U28 的端到端验证:
// 此前 NoRoute 只在 mountSPA 成功(存在 web/dist)时注册,
// 纯 API 部署下这些路径落到 gin 默认的空 body 404。
// 本测试没有 web/dist,正是那个场景。
func TestUnknownV1PathIsStructured(t *testing.T) {
	srv := newTestServer(t)
	res := get(t, srv, http.MethodPost, "/v1/nosuchthing", nil)

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	e := decodeErrBody(t, res)
	msg, _ := e["message"].(string)
	if !strings.Contains(msg, "/v1/chat/completions") {
		t.Errorf("应列出支持的端点: %q", msg)
	}
}

// TestKnownUnsupportedEndpointsAre501 端到端验证 U37。
// 注意这些路径没有注册路由,走的是 NoRoute —— 因此不经过 /v1 组的鉴权中间件,
// 无需 API Key 也能拿到明确答复,这正是我们想要的行为。
func TestKnownUnsupportedEndpointsAre501(t *testing.T) {
	srv := newTestServer(t)
	for _, p := range []string{
		"/v1/embeddings", "/v1/moderations", "/v1/audio/speech",
		"/v1/files", "/v1/batches", "/v1/completions",
	} {
		t.Run(p, func(t *testing.T) {
			res := get(t, srv, http.MethodPost, p, nil)
			if res.StatusCode != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501", res.StatusCode)
			}
			e := decodeErrBody(t, res)
			if msg, _ := e["message"].(string); len(msg) < 20 {
				t.Errorf("501 必须解释原因: %q", msg)
			}
		})
	}
}

// TestResponsesRoutesRegistered 验证新增路由真的挂上了 ——
// 单元测试直接调 handler 是发现不了"忘了注册"的。
// 有鉴权中间件,所以期望 401 而不是 404:401 就证明路由匹配成功了。
func TestResponsesRoutesRegistered(t *testing.T) {
	srv := newTestServer(t)
	cases := []struct{ method, path string }{
		{http.MethodPost, "/v1/responses"},
		{http.MethodGet, "/v1/responses/resp_1"},
		{http.MethodDelete, "/v1/responses/resp_1"},
		{http.MethodPost, "/v1/responses/resp_1/cancel"},
		{http.MethodGet, "/v1/responses/resp_1/input_items"},
		{http.MethodGet, "/v1/models/gpt-5"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			res := get(t, srv, tc.method, tc.path, nil)
			if res.StatusCode == http.StatusNotFound {
				t.Fatalf("路由未注册(拿到 404)")
			}
			if res.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401(路由已挂、被鉴权拦下)", res.StatusCode)
			}
		})
	}
}

// TestCORSPreflightForBrowserSDK 端到端验证 U10:
// 浏览器里的 openai-js 每个请求都带一串 x-stainless-* 头,
// 此前允许列表只有三项,预检直接被浏览器拦掉。
func TestCORSPreflightForBrowserSDK(t *testing.T) {
	srv := newTestServer(t)
	res := get(t, srv, http.MethodOptions, "/v1/chat/completions", map[string]string{
		"Origin":                         "https://app.example.com",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "authorization,content-type,x-stainless-lang,x-stainless-retry-count",
	})

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("预检 status = %d, want 204", res.StatusCode)
	}
	allow := res.Header.Get("Access-Control-Allow-Headers")
	for _, want := range []string{"x-stainless-lang", "x-stainless-retry-count"} {
		if !strings.Contains(allow, want) {
			t.Errorf("Allow-Headers 缺 %q: %q", want, allow)
		}
	}
	expose := res.Header.Get("Access-Control-Expose-Headers")
	if !strings.Contains(expose, "x-ratelimit-remaining-requests") {
		t.Errorf("限流头必须可读: %q", expose)
	}
	// 通配符 origin 下不得下发 credentials。
	if got := res.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("通配符下不该允许 credentials: %q", got)
	}
}

// TestRequestIDHeaderPresent 确认可观测性头在真实链路上生效。
func TestRequestIDHeaderPresent(t *testing.T) {
	srv := newTestServer(t)
	res := get(t, srv, http.MethodGet, "/healthz", nil)
	if res.Header.Get("X-Request-Id") == "" {
		t.Error("缺少 X-Request-Id")
	}
}

// TestNonV1UnknownPathStaysBare 确认没有把 /v1 的错误信封漏给整站。
func TestNonV1UnknownPathStaysBare(t *testing.T) {
	srv := newTestServer(t)
	res := get(t, srv, http.MethodGet, "/totally/unknown", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var body map[string]interface{}
	if json.NewDecoder(res.Body).Decode(&body) == nil {
		if _, hasErr := body["error"]; hasErr {
			t.Error("非 /v1 路径不该套 OpenAI 错误信封")
		}
	}
}
