// Package oaierr 产出符合 OpenAI 规范的错误信封。
//
// openai-openapi 的 Error schema 把 code / message / param / type 四个键全部列为
// required —— 客户端(LiteLLM Router、LangChain error handler、one-api 聚合层)按
// error.type 分支决定"换 key / 换渠道 / 退避重试",按 error.param 定位出错字段。
// 键缺失和值为 null 是两回事,所以 Param/Code 用指针,空值序列化成 null 而不是省略。
//
// 本包被 internal/gateway 与 internal/apikey 共用,放在 pkg/ 下避免包循环。
package oaierr

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 错误类型。官方 spec 里 type 是裸 string 无 enum,全站只出现过
// invalid_request_error / server_error / insufficient_quota 三个字面量;
// 其余取值取自 LiteLLM / LangChain 实际分支使用的约定值。
const (
	TypeInvalidRequest = "invalid_request_error"
	TypeAuthentication = "authentication_error"
	TypePermission     = "permission_error"
	TypeNotFound       = "not_found_error"
	TypeRateLimit      = "rate_limit_error"
	TypeServer         = "server_error"
)

// 错误码。前半段沿用本网关既有取值(保持向后兼容),后半段是新增的规范码。
const (
	CodeMissingAPIKey        = "missing_api_key"
	CodeInvalidAPIKey        = "invalid_api_key"
	CodeIPNotAllowed         = "ip_not_allowed"
	CodeModelNotFound        = "model_not_found"
	CodeModelNotAllowed      = "model_not_allowed"
	CodeInsufficientQuota    = "insufficient_quota"
	CodeRateLimitExceeded    = "rate_limit_exceeded"
	CodeUpstreamError        = "upstream_error"
	CodeNoAccountAvailable   = "no_account_available"
	CodeUnsupportedParameter = "unsupported_parameter"
	CodeUnsupportedValue     = "unsupported_value"
	CodeUnsupportedEndpoint  = "unsupported_endpoint"
	CodeInvalidRequestError  = "invalid_request_error"
	CodeContextLengthExceed  = "context_length_exceeded"
	CodeUpstreamEmptyOutput  = "upstream_empty_output"
	CodeUpstreamInterrupted  = "upstream_interrupted"
)

// Payload 是 error 对象本身。四个字段全部输出,空值为 null。
type Payload struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

// Envelope 是完整响应体 {"error": {...}}。
type Envelope struct {
	Error Payload `json:"error"`
}

// TypeForStatus 由 HTTP 状态码推导 error.type。
func TypeForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return TypeAuthentication
	case status == http.StatusForbidden:
		return TypePermission
	case status == http.StatusNotFound:
		return TypeNotFound
	case status == http.StatusTooManyRequests:
		return TypeRateLimit
	case status >= 500:
		return TypeServer
	default:
		return TypeInvalidRequest
	}
}

// StatusForUpstream 把上游 chatgpt.com 的状态码翻译成本网关对外的状态码。
//
// 关键点:上游的 401/403 是"我们的账号池出问题了",不是"调用方的 key 错了"。
// 原样透传会让客户端把自己的 key 当成无效 key 丢弃(LiteLLM 会永久拉黑渠道),
// 所以一律压成 502。而 429 / 503 / 504 是调用方值得重试的信号,保留语义。
func StatusForUpstream(upstreamStatus int) int {
	switch upstreamStatus {
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests
	case http.StatusServiceUnavailable:
		return http.StatusServiceUnavailable
	case http.StatusGatewayTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}

// New 构造信封。typ 为空时由 status 推导。
func New(status int, typ, code, param, message string) *Envelope {
	if typ == "" {
		typ = TypeForStatus(status)
	}
	e := &Envelope{Error: Payload{Message: message, Type: typ}}
	if code != "" {
		e.Error.Code = &code
	}
	if param != "" {
		e.Error.Param = &param
	}
	return e
}

// Write 输出错误并 Abort。type 由状态码推导,适合大多数调用点。
func Write(c *gin.Context, status int, code, param, message string) {
	WriteTyped(c, status, "", code, param, message)
}

// WriteTyped 在需要覆盖 type 时使用(例如 402 余额不足要报 insufficient_quota)。
func WriteTyped(c *gin.Context, status int, typ, code, param, message string) {
	c.AbortWithStatusJSON(status, New(status, typ, code, param, message))
}

// SSEErrorLine 返回一条可直接写入流的 SSE 错误事件(含结尾空行)。
//
// 用于"流已经开始、响应头已经 200 发出去"之后上游才出错的场景:此时改状态码已经
// 来不及,唯一能让客户端知道出事的办法就是在流里送一个 error 事件。
func SSEErrorLine(status int, code, param, message string) string {
	b, err := json.Marshal(New(status, "", code, param, message))
	if err != nil {
		// Payload 全是 string/指针,理论上不可能失败;兜底成静态串保证流不中断。
		return "data: {\"error\":{\"message\":\"internal marshal error\",\"type\":\"server_error\",\"param\":null,\"code\":null}}\n\n"
	}
	return "data: " + string(b) + "\n\n"
}
