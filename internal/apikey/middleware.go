package apikey

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/pkg/oaierr"
)

const (
	CtxKey      = "apikey"
	CtxKeyOwner = "apikey_user_id"
)

// Middleware 返回一个 gin 中间件,按 OpenAI 规范校验 Bearer Key。
// allowQuery=true 允许 ?api_key= 作为兜底(浏览器直出)。
func Middleware(svc *Service, allowQuery bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractKey(c, allowQuery)
		if raw == "" {
			openAIAuthError(c, "missing_api_key", "缺少 API Key,请在 Authorization 头中传入 Bearer <key>")
			return
		}
		k, err := svc.Verify(c.Request.Context(), raw)
		if err != nil {
			openAIAuthError(c, "invalid_api_key", err.Error())
			return
		}
		ip := c.ClientIP()
		if !k.IPAllowed(ip) {
			// 403 而不是 401:key 本身是有效的,是这个来源不被允许。
			// 回 401 会让 SDK 与中转层判定"key 无效"并把它丢弃/拉黑,
			// 用户拿着一把好 key 却被引向"重新申请 key"的错误方向。
			oaierr.Write(c, http.StatusForbidden, oaierr.CodeIPNotAllowed, "",
				"当前 IP("+ip+")不在该 API Key 的白名单内")
			return
		}
		c.Set(CtxKey, k)
		c.Set(CtxKeyOwner, k.UserID)
		c.Next()
	}
}

func extractKey(c *gin.Context, allowQuery bool) string {
	if h := c.GetHeader("Authorization"); h != "" {
		// RFC 7235 规定 auth-scheme 大小写不敏感。此前只认首字母大写的 "Bearer ",
		// 小写 "bearer xxx" 会被当成裸 key 送去 Verify,报"格式不正确",
		// 把用户引向完全错误的排查方向。
		if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			return strings.TrimSpace(h[7:])
		}
		return strings.TrimSpace(h)
	}
	// Azure OpenAI 风格的 api-key 头,以及部分客户端惯用的 x-api-key。
	// 接受它们不损失任何安全性,却能省掉一类"配置对了但连不上"的支持成本。
	for _, hdr := range []string{"api-key", "X-Api-Key"} {
		if v := strings.TrimSpace(c.GetHeader(hdr)); v != "" {
			return v
		}
	}
	if allowQuery {
		if v := c.Query("api_key"); v != "" {
			return v
		}
	}
	return ""
}

// FromCtx 取回 APIKey 对象。
func FromCtx(c *gin.Context) (*APIKey, bool) {
	v, ok := c.Get(CtxKey)
	if !ok {
		return nil, false
	}
	k, ok := v.(*APIKey)
	return k, ok
}

// openAIAuthError 按 OpenAI 规范返回 401 错误。
//
// 注意 type 必须是 authentication_error 而不是 invalid_request_error:
// LiteLLM / LangChain 按 type 决定"换一把 key"还是"改请求参数"。
func openAIAuthError(c *gin.Context, code, msg string) {
	oaierr.Write(c, http.StatusUnauthorized, code, "", msg)
}
