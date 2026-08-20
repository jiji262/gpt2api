package gateway

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// publicBaseURL 返回对外可访问的根地址(scheme://host,无尾斜杠)。
//
// 优先用管理员配置的 site.api_base_url;留空时从请求头推导。
// 部署在 Caddy / Nginx 后面时,c.Request.Host 是内网地址、TLS 是 nil,
// 只有 X-Forwarded-* 才反映用户真正访问的地址。
func publicBaseURL(c *gin.Context, configured string) string {
	if s := strings.TrimRight(strings.TrimSpace(configured), "/"); s != "" {
		return s
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if v := firstForwarded(c.GetHeader("X-Forwarded-Proto")); v != "" {
		scheme = v
	}

	host := c.Request.Host
	if v := firstForwarded(c.GetHeader("X-Forwarded-Host")); v != "" {
		host = v
	}
	return scheme + "://" + host
}

// firstForwarded 取 "a, b, c" 形式头部的第一段。多级代理会逐跳追加,
// 第一段才是最靠近用户的那一跳。
func firstForwarded(v string) string {
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// absoluteURL 把站内绝对路径补成完整 URL。已经是完整 URL 的原样返回。
func absoluteURL(c *gin.Context, configured, pathOrURL string) string {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL
	}
	return publicBaseURL(c, configured) + pathOrURL
}

// siteAPIBaseURL 从注入的 Settings 里读配置值,未注入时返回空串(退回请求头推导)。
func (h *Handler) siteAPIBaseURL() string {
	if h == nil || h.Settings == nil {
		return ""
	}
	if s, ok := h.Settings.(interface{ SiteAPIBaseURL() string }); ok {
		return s.SiteAPIBaseURL()
	}
	return ""
}

// imageURL 生成一条对外可用的图片绝对 URL。
func (h *ImagesHandler) imageURL(c *gin.Context, taskID string, idx int) string {
	return absoluteURL(c, h.siteAPIBaseURL(), BuildImageProxyURL(taskID, idx, ImageProxyTTL))
}
