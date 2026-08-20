package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/internal/gateway"
)

// mountSPA 把前端 Vite 产物(web/dist)挂到 `/` 上,并实现 SPA 回退(deep link 刷新)。
//
// 路径选择优先级:
//  1. 环境变量 GPT2API_WEB_DIR
//  2. 容器默认:/app/web/dist
//  3. 源码工作目录:./web/dist
//  4. 都不存在则什么都不挂(退化为纯 API)
//
// 注意:
//   - 只有 GET/HEAD 的 NoRoute 请求才会被 fallback 到 index.html。其它方法保持 404。
//   - 明确排除 /api/、/v1/、/healthz、/readyz 等 API 前缀,避免打包问题把接口 404 掩盖成 index.html。
func mountSPA(r *gin.Engine) gin.HandlerFunc {
	dir := resolveWebDir()
	if dir == "" {
		return nil
	}
	indexPath := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return nil
	}

	// 自动挂载 dist/ 根下的所有子目录为静态资源(/assets、/screenshots、未来可能的 /img 等),
	// 避免"加了 public/xxx 但后端忘注册 → 请求被 NoRoute 吞成 index.html"的坑。
	// 同时把所有根层普通文件(favicon、robots.txt、manifest 等)也一并注册。
	// 注意:index.html 不走 StaticFile,单独由 r.GET("/") 处理。
	entries, _ := os.ReadDir(dir)
	for _, ent := range entries {
		name := ent.Name()
		if name == "" || name == "." || name == ".." {
			continue
		}
		full := filepath.Join(dir, name)
		if ent.IsDir() {
			r.Static("/"+name, full)
		} else if name != "index.html" {
			r.StaticFile("/"+name, full)
		}
	}

	// 根路径直接返回 index.html,而不是 404。
	r.GET("/", func(c *gin.Context) { c.File(indexPath) })

	// 返回 deep link 兜底处理器,由 router.New 统一注册进 NoRoute。
	//
	// 此前这里直接调 r.NoRoute:纯 API 部署(没有 web/dist)时整个 NoRoute
	// 都不会注册,/v1 下的未知路径落到 gin 默认 404 —— 空 body,
	// 被硬路由过来的客户端完全看不出发生了什么。
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		p := c.Request.URL.Path
		for _, prefix := range apiPrefixes {
			if strings.HasPrefix(p, prefix) {
				c.Status(http.StatusNotFound)
				return
			}
		}
		c.File(indexPath)
	}
}

// notFoundHandler 统一 NoRoute:/v1 下的未知路径一律回 OpenAI 错误信封,
// 其余交给 SPA(没有 SPA 就退化成裸 404)。
func notFoundHandler(spa gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			gateway.V1NotFound(c)
			return
		}
		if spa != nil {
			spa(c)
			return
		}
		c.Status(http.StatusNotFound)
	}
}

// API 前缀白名单:凡是命中这里的请求不做 SPA fallback。
var apiPrefixes = []string{
	"/api/",
	"/v1/",
	"/healthz",
	"/readyz",
	"/assets/",
}

func resolveWebDir() string {
	if d := os.Getenv("GPT2API_WEB_DIR"); d != "" {
		if isDir(d) {
			return d
		}
	}
	candidates := []string{
		"/app/web/dist",
		"./web/dist",
	}
	for _, d := range candidates {
		if isDir(d) {
			abs, _ := filepath.Abs(d)
			return abs
		}
	}
	return ""
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	if err != nil {
		return false
	}
	return st.IsDir()
}
