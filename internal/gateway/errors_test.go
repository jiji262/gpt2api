package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// decodeErr 解出 {"error":{...}} 的四个字段。缺键与 null 是两回事,
// 所以用 map 而不是结构体解,才能区分。
func decodeErr(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var got map[string]map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (%s)", err, body)
	}
	e, ok := got["error"]
	if !ok {
		t.Fatalf("缺少顶层 error 键: %s", body)
	}
	for _, k := range []string{"message", "type", "param", "code"} {
		if _, ok := e[k]; !ok {
			t.Fatalf("缺少 OpenAI Error schema 的必需键 %q: %s", k, body)
		}
	}
	return e
}

func TestOpenAIErrorDerivesTypeFromStatus(t *testing.T) {
	cases := []struct {
		status   int
		wantType string
	}{
		{http.StatusBadRequest, "invalid_request_error"},
		{http.StatusUnauthorized, "authentication_error"},
		{http.StatusForbidden, "permission_error"},
		{http.StatusNotFound, "not_found_error"},
		{http.StatusTooManyRequests, "rate_limit_error"},
		{http.StatusBadGateway, "server_error"},
		{http.StatusServiceUnavailable, "server_error"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		openAIError(c, tc.status, "some_code", "boom")

		if w.Code != tc.status {
			t.Errorf("status = %d, want %d", w.Code, tc.status)
		}
		e := decodeErr(t, w.Body.Bytes())
		if e["type"] != tc.wantType {
			t.Errorf("status %d: type = %v, want %q", tc.status, e["type"], tc.wantType)
		}
		if e["param"] != nil {
			t.Errorf("status %d: param 应为 null,实际 %v", tc.status, e["param"])
		}
	}
}

func TestOpenAIErrorParamPointsAtField(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	openAIErrorParam(c, http.StatusNotFound, "model_not_found", "model", `模型 "x" 不存在`)

	e := decodeErr(t, w.Body.Bytes())
	if e["param"] != "model" {
		t.Errorf("param = %v, want model", e["param"])
	}
	if e["code"] != "model_not_found" {
		t.Errorf("code = %v", e["code"])
	}
}

func TestOpenAIErrorTypedOverridesType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// 余额不足:状态码保留 402,但 type 用官方字面量,让按 type 分支的中间层认得出。
	openAIErrorTyped(c, http.StatusPaymentRequired, "insufficient_quota",
		"insufficient_quota", "", "积分不足")

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", w.Code)
	}
	e := decodeErr(t, w.Body.Bytes())
	if e["type"] != "insufficient_quota" {
		t.Errorf("type = %v, want insufficient_quota", e["type"])
	}
}

func TestJSONFieldName(t *testing.T) {
	cases := map[string]string{
		"Model":          "model",
		"Messages":       "messages",
		"MaxTokens":      "max_tokens",
		"TopP":           "top_p",
		"ResponseFormat": "response_format",
		"N":              "n",
	}
	for in, want := range cases {
		if got := jsonFieldName(in); got != want {
			t.Errorf("jsonFieldName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBindErrorMessageIsActionable 确认不再把 validator 的英文原文和内部结构体名
// 直接吐给客户端。
func TestBindErrorMessageIsActionable(t *testing.T) {
	var req ChatCompletionsRequest
	body := strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`) // 缺 model

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	c.Request.Header.Set("Content-Type", "application/json")

	err := c.ShouldBindJSON(&req)
	if err == nil {
		t.Fatal("期望校验失败,实际通过")
	}
	param, msg := bindErrorMessage(err)
	if param != "model" {
		t.Errorf("param = %q, want model", param)
	}
	if strings.Contains(msg, "ChatCompletionsRequest") {
		t.Errorf("消息泄漏了内部结构体名: %q", msg)
	}
	if strings.Contains(msg, "validation for") {
		t.Errorf("消息仍是 validator 英文原文: %q", msg)
	}
	if !strings.Contains(msg, "model") {
		t.Errorf("消息没告诉用户是哪个字段: %q", msg)
	}
}

func TestBindErrorMessageFallsBackOnNonValidatorError(t *testing.T) {
	var req ChatCompletionsRequest
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{not json`))
	c.Request.Header.Set("Content-Type", "application/json")

	err := c.ShouldBindJSON(&req)
	if err == nil {
		t.Fatal("期望解析失败")
	}
	param, msg := bindErrorMessage(err)
	if param != "" {
		t.Errorf("语法错误定位不到字段,param 应为空,实际 %q", param)
	}
	if msg == "" {
		t.Error("消息不能为空")
	}
}
