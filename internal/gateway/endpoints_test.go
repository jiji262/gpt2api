package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func v1Req(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	V1NotFound(c)
	return w
}

// TestV1NotFoundIsStructured 覆盖 U28:
// /v1 下的未知路径此前落到 gin 默认 404(空 body),纯 API 部署下连 NoRoute
// 都没注册。被 LangChain / LobeChat 按模型名硬路由过来的客户端只看到一个空 404。
func TestV1NotFoundIsStructured(t *testing.T) {
	w := v1Req(t, "/v1/nonexistent")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	e := decodeErr(t, w.Body.Bytes())
	if e["code"] != "unsupported_endpoint" {
		t.Errorf("code = %v", e["code"])
	}
	msg, _ := e["message"].(string)
	if !strings.Contains(msg, "/v1/chat/completions") {
		t.Errorf("消息里应列出支持的端点: %q", msg)
	}
}

// TestKnownUnsupportedEndpointsReturn501 覆盖 U37:
// 上游做不到的端点应显式 501 + 说明原因,而不是和拼错路径一样都回 404。
func TestKnownUnsupportedEndpointsReturn501(t *testing.T) {
	for _, p := range []string{
		"/v1/embeddings",
		"/v1/moderations",
		"/v1/audio/speech",
		"/v1/audio/transcriptions",
		"/v1/completions",
		"/v1/files",
		"/v1/batches",
		"/v1/assistants/asst_1",
		"/v1/realtime/sessions",
	} {
		t.Run(p, func(t *testing.T) {
			w := v1Req(t, p)
			if w.Code != http.StatusNotImplemented {
				t.Fatalf("%s status = %d, want 501", p, w.Code)
			}
			e := decodeErr(t, w.Body.Bytes())
			if e["type"] != "server_error" {
				t.Errorf("type = %v", e["type"])
			}
			msg, _ := e["message"].(string)
			if len(msg) < 20 {
				t.Errorf("501 必须解释原因: %q", msg)
			}
		})
	}
}

func TestUnsupportedMatchesLongestPrefix(t *testing.T) {
	// /v1/audio/transcriptions 比 /v1/audio/... 更具体,应命中更长的那条。
	reason, ok := matchUnsupported("/v1/audio/transcriptions")
	if !ok || !strings.Contains(reason, "转写") {
		t.Fatalf("reason = %q ok=%v", reason, ok)
	}
}

func TestSupportedEndpointsListIsHonest(t *testing.T) {
	// 列表里出现的端点必须真的注册了;漏改这里比不写更糟。
	for _, e := range SupportedEndpoints {
		path := e[strings.LastIndex(e, " ")+1:]
		if _, unsupported := matchUnsupported(path); unsupported {
			t.Errorf("%q 同时出现在支持列表和不支持列表里", e)
		}
	}
}
