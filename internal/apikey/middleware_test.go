package apikey

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/pkg/oaierr"
)

// oaierrWriteForbidden 复刻 Middleware 里 IP 白名单拒绝分支的输出,
// 避免为了测一个错误形状去搭一整套 DB 依赖。
func oaierrWriteForbidden(c *gin.Context) {
	oaierr.Write(c, http.StatusForbidden, oaierr.CodeIPNotAllowed, "",
		"当前 IP(1.2.3.4)不在该 API Key 的白名单内")
}

func init() { gin.SetMode(gin.TestMode) }

func TestExtractKey(t *testing.T) {
	cases := []struct {
		name       string
		headers    map[string]string
		query      string
		allowQuery bool
		want       string
	}{
		{"标准 Bearer", map[string]string{"Authorization": "Bearer sk-abc"}, "", false, "sk-abc"},
		// RFC 7235: auth-scheme 大小写不敏感。修复前小写会被当成裸 key。
		{"小写 bearer", map[string]string{"Authorization": "bearer sk-abc"}, "", false, "sk-abc"},
		{"大写 BEARER", map[string]string{"Authorization": "BEARER sk-abc"}, "", false, "sk-abc"},
		{"混合 BeArEr", map[string]string{"Authorization": "BeArEr sk-abc"}, "", false, "sk-abc"},
		{"裸 key 兼容", map[string]string{"Authorization": "sk-abc"}, "", false, "sk-abc"},
		{"Bearer 后多余空格", map[string]string{"Authorization": "Bearer   sk-abc  "}, "", false, "sk-abc"},
		{"api-key 头", map[string]string{"api-key": "sk-azure"}, "", false, "sk-azure"},
		{"X-Api-Key 头", map[string]string{"X-Api-Key": "sk-x"}, "", false, "sk-x"},
		{"Authorization 优先于 api-key", map[string]string{"Authorization": "Bearer sk-a", "api-key": "sk-b"}, "", false, "sk-a"},
		{"query 兜底关闭", nil, "sk-q", false, ""},
		{"query 兜底开启", nil, "sk-q", true, "sk-q"},
		{"全空", nil, "", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			url := "/v1/chat/completions"
			if tc.query != "" {
				url += "?api_key=" + tc.query
			}
			c.Request = httptest.NewRequest(http.MethodPost, url, nil)
			for k, v := range tc.headers {
				c.Request.Header.Set(k, v)
			}
			if got := extractKey(c, tc.allowQuery); got != tc.want {
				t.Errorf("extractKey = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestMissingKeyReturnsAuthenticationError 覆盖 F1:401 的 type 必须是
// authentication_error,LiteLLM 与 LangChain 按它决定"换一把 key"而不是"改参数"。
func TestMissingKeyReturnsAuthenticationError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// svc 传 nil 是安全的:没有 key 时在 Verify 之前就已经 Abort。
	Middleware(nil, false)(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !c.IsAborted() {
		t.Fatal("期望 Abort")
	}
	var got map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w.Body.String())
	}
	e := got["error"]
	for _, k := range []string{"message", "type", "param", "code"} {
		if _, ok := e[k]; !ok {
			t.Errorf("缺少必需键 %q: %s", k, w.Body.String())
		}
	}
	if e["type"] != "authentication_error" {
		t.Errorf("type = %v, want authentication_error", e["type"])
	}
	if e["code"] != "missing_api_key" {
		t.Errorf("code = %v, want missing_api_key", e["code"])
	}
	if e["param"] != nil {
		t.Errorf("param 应为 null,实际 %v", e["param"])
	}
}

// TestIPNotAllowedIsForbidden 覆盖 U6:
// key 有效但来源不被允许,是权限问题(403)不是身份问题(401)。
// 回 401 会让 SDK 与中转层判定"key 无效"并丢弃/拉黑它。
func TestIPNotAllowedIsForbidden(t *testing.T) {
	// 直接验证错误形状:构造完整 Service 需要 DB,这里只测状态码与信封语义。
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	oaierrWriteForbidden(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	var got map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["error"]["type"] != "permission_error" {
		t.Errorf("type = %v, want permission_error", got["error"]["type"])
	}
	if got["error"]["code"] != "ip_not_allowed" {
		t.Errorf("code = %v", got["error"]["code"])
	}
}
