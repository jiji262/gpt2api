package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// defaultAllowHeaders 是始终允许的请求头。
//
// 此前这里只有 Authorization / Content-Type / X-Request-Id 三项:
// 浏览器里的 openai-js(Stainless 生成)每个请求都会带一串 x-stainless-*
// 元数据头,预检时它们不在允许列表里,整个请求直接被浏览器拦掉。
var defaultAllowHeaders = []string{
	"Authorization",
	"Content-Type",
	"Accept",
	"X-Request-Id",
	"OpenAI-Organization",
	"OpenAI-Project",
	"OpenAI-Beta",
	"api-key",
	"X-Api-Key",
}

// exposeHeaders 是允许 JS 读取的响应头。限流头必须在这里,
// 否则浏览器端 SDK 拿不到 x-ratelimit-*,预测性退避形同虚设。
var exposeHeaders = strings.Join([]string{
	"X-Request-Id",
	"Retry-After",
	"x-ratelimit-limit-requests",
	"x-ratelimit-remaining-requests",
	"x-ratelimit-reset-requests",
	"x-ratelimit-limit-tokens",
	"x-ratelimit-remaining-tokens",
	"x-ratelimit-reset-tokens",
	"X-Gateway-Ignored-Params",
	"X-Gateway-Ratelimit-Degraded",
}, ", ")

// CORS 简易跨域中间件。
func CORS(origins []string) gin.HandlerFunc {
	allow := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, o := range origins {
		if o == "*" {
			allowAll = true
		}
		allow[strings.TrimRight(o, "/")] = struct{}{}
	}
	baseHeaders := strings.Join(defaultAllowHeaders, ", ")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			_, whitelisted := allow[strings.TrimRight(origin, "/")]
			if allowAll || whitelisted {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
			c.Writer.Header().Set("Vary", "Origin")
			// 只有显式白名单的 origin 才带凭据。配了 "*" 还回显任意 origin
			// 并允许 credentials,等于对全网开放带 Cookie 的跨站请求。
			if whitelisted && !allowAll {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeadersFor(c, baseHeaders))
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// allowHeadersFor 在固定列表之外,回显浏览器在预检里申报的自定义头。
//
// 这是处理 SDK 自带元数据头(x-stainless-*、x-lang-* 等)的标准做法:
// 硬编码一份清单永远跟不上 SDK 的版本迭代。
func allowHeadersFor(c *gin.Context, base string) string {
	requested := strings.TrimSpace(c.GetHeader("Access-Control-Request-Headers"))
	if requested == "" {
		return base
	}
	return base + ", " + requested
}
