package oaierr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// TestEnvelopeAlwaysHasFourKeys 是本包存在的理由:
// openai-openapi 的 Error schema 把 code/message/param/type 四个键全部列为 required,
// 客户端可以按 null 判断,不能按"键缺失"判断。
func TestEnvelopeAlwaysHasFourKeys(t *testing.T) {
	cases := []struct {
		name      string
		code      string
		param     string
		wantCode  interface{}
		wantParam interface{}
	}{
		{"both empty", "", "", nil, nil},
		{"code only", "model_not_found", "", "model_not_found", nil},
		{"both set", "unsupported_parameter", "tools", "unsupported_parameter", "tools"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(New(http.StatusBadRequest, TypeInvalidRequest, tc.code, tc.param, "boom"))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]map[string]interface{}
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			e, ok := got["error"]
			if !ok {
				t.Fatalf("缺少顶层 error 键: %s", b)
			}
			for _, k := range []string{"message", "type", "param", "code"} {
				if _, ok := e[k]; !ok {
					t.Errorf("缺少必需键 %q: %s", k, b)
				}
			}
			if e["code"] != tc.wantCode {
				t.Errorf("code = %#v, want %#v", e["code"], tc.wantCode)
			}
			if e["param"] != tc.wantParam {
				t.Errorf("param = %#v, want %#v", e["param"], tc.wantParam)
			}
			if e["message"] != "boom" {
				t.Errorf("message = %#v", e["message"])
			}
		})
	}
}

func TestTypeForStatus(t *testing.T) {
	cases := map[int]string{
		http.StatusBadRequest:          TypeInvalidRequest,
		http.StatusUnauthorized:        TypeAuthentication,
		http.StatusPaymentRequired:     TypeInvalidRequest,
		http.StatusForbidden:           TypePermission,
		http.StatusNotFound:            TypeNotFound,
		http.StatusTooManyRequests:     TypeRateLimit,
		http.StatusInternalServerError: TypeServer,
		http.StatusBadGateway:          TypeServer,
		http.StatusServiceUnavailable:  TypeServer,
		http.StatusNotImplemented:      TypeServer,
	}
	for status, want := range cases {
		if got := TypeForStatus(status); got != want {
			t.Errorf("TypeForStatus(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestWriteSetsStatusAndAborts(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	Write(c, http.StatusTooManyRequests, CodeRateLimitExceeded, "", "慢一点")

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if !c.IsAborted() {
		t.Error("期望 Abort,实际没有")
	}
	if ct := w.Header().Get("Content-Type"); ct == "" {
		t.Error("缺少 Content-Type")
	}
	var got Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, w.Body.String())
	}
	// Write 不传 type,应由状态码推导。
	if got.Error.Type != TypeRateLimit {
		t.Errorf("type = %q, want %q", got.Error.Type, TypeRateLimit)
	}
	if got.Error.Code == nil || *got.Error.Code != CodeRateLimitExceeded {
		t.Errorf("code = %v", got.Error.Code)
	}
	if got.Error.Param != nil {
		t.Errorf("param 应为 null,实际 %v", *got.Error.Param)
	}
}

func TestWriteParamPointsAtField(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	WriteTyped(c, http.StatusBadRequest, TypeInvalidRequest, CodeUnsupportedParameter, "tools", "不支持")

	var got Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error.Param == nil || *got.Error.Param != "tools" {
		t.Fatalf("param = %v, want tools", got.Error.Param)
	}
}

// TestSSEErrorEventShape 覆盖 F9:流已经开始后上游出错,
// 必须以 SSE 事件把 OpenAI 错误信封送出去,而不是静默断连。
func TestSSEErrorEventShape(t *testing.T) {
	line := SSEErrorLine(http.StatusBadGateway, CodeUpstreamError, "", "上游断了")
	const prefix = "data: "
	if len(line) < len(prefix) || line[:len(prefix)] != prefix {
		t.Fatalf("SSE 行必须以 %q 开头: %q", prefix, line)
	}
	if suffix := line[len(line)-2:]; suffix != "\n\n" {
		t.Fatalf("SSE 行必须以空行结尾: %q", line)
	}
	var got Envelope
	if err := json.Unmarshal([]byte(line[len(prefix):len(line)-2]), &got); err != nil {
		t.Fatalf("SSE 载荷不是合法 JSON: %v", err)
	}
	if got.Error.Type != TypeServer {
		t.Errorf("type = %q, want %q", got.Error.Type, TypeServer)
	}
	if got.Error.Code == nil || *got.Error.Code != CodeUpstreamError {
		t.Errorf("code = %v", got.Error.Code)
	}
}

func TestStatusForUpstream(t *testing.T) {
	cases := map[int]int{
		http.StatusUnauthorized:        http.StatusBadGateway,
		http.StatusForbidden:           http.StatusBadGateway,
		http.StatusTooManyRequests:     http.StatusTooManyRequests,
		http.StatusBadGateway:          http.StatusBadGateway,
		http.StatusServiceUnavailable:  http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:      http.StatusGatewayTimeout,
		http.StatusInternalServerError: http.StatusBadGateway,
		499:                            http.StatusBadGateway,
	}
	for upstream, want := range cases {
		if got := StatusForUpstream(upstream); got != want {
			t.Errorf("StatusForUpstream(%d) = %d, want %d", upstream, got, want)
		}
	}
}
