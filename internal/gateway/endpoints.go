package gateway

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/internal/apikey"
	modelpkg "github.com/jiji262/gpt2api/internal/model"
	"github.com/jiji262/gpt2api/pkg/oaierr"
)

// SupportedEndpoints 是本网关真正实现的 OpenAI 端点,用于 404/501 的提示文案。
var SupportedEndpoints = []string{
	"GET  /v1/models",
	"GET  /v1/models/{model}",
	"POST /v1/chat/completions",
	"POST /v1/responses",
	"POST /v1/images/generations",
	"POST /v1/images/edits",
}

// unsupportedReasons 记录"官方有、本网关上游做不到"的端点及原因。
// 命中这些路径时返回 501 + 明确解释,而不是让客户端对着空 body 的 404 猜。
var unsupportedReasons = map[string]string{
	"/v1/embeddings":           "上游 chatgpt.com 网页版不提供向量化接口",
	"/v1/moderations":          "上游不单独暴露审核接口,审核在生成过程中隐式发生",
	"/v1/audio/speech":         "上游不提供语音合成接口",
	"/v1/audio/transcriptions": "上游不提供语音转写接口",
	"/v1/audio/translations":   "上游不提供语音翻译接口",
	"/v1/completions":          "官方已弃用的 legacy 补全端点,请改用 /v1/chat/completions",
	"/v1/files":                "上游的文件通道仅服务于对话内附件,不构成独立的文件存储",
	"/v1/batches":              "批处理依赖官方 Platform 侧的异步作业系统",
	"/v1/vector_stores":        "向量存储依赖官方 Platform 侧能力",
	"/v1/realtime":             "实时语音会话依赖官方 Realtime 协议",
	"/v1/assistants":           "Assistants API 官方已于 2026-08-26 停用",
	"/v1/conversations":        "本网关不持有服务端会话状态",
	"/v1/organization":         "组织管理属于官方 Platform 账户体系",
}

// V1NotFound 处理落到 /v1/ 下但没有匹配路由的请求。
//
// 此前这类请求要么被 gin 默认 404(空 body),要么在纯 API 部署下连 NoRoute
// 都没注册。被 LangChain / LobeChat 之类按模型名硬路由到 /v1/responses 的
// 客户端只会看到一个空 404,完全不知道发生了什么。
func V1NotFound(c *gin.Context) {
	p := c.Request.URL.Path
	if reason, ok := matchUnsupported(p); ok {
		oaierr.WriteTyped(c, http.StatusNotImplemented, oaierr.TypeServer,
			oaierr.CodeUnsupportedEndpoint, "",
			"本网关不支持 "+p+":"+reason+"。当前支持的端点:"+strings.Join(SupportedEndpoints, " / "))
		return
	}
	oaierr.Write(c, http.StatusNotFound, oaierr.CodeUnsupportedEndpoint, "",
		"未知端点 "+p+"。当前支持的端点:"+strings.Join(SupportedEndpoints, " / "))
}

// matchUnsupported 按最长前缀匹配已知的不支持端点。
func matchUnsupported(path string) (string, bool) {
	best, bestLen := "", 0
	for prefix, reason := range unsupportedReasons {
		if strings.HasPrefix(path, prefix) && len(prefix) > bestLen {
			best, bestLen = reason, len(prefix)
		}
	}
	return best, bestLen > 0
}

// modelObject 是 /v1/models 的单条记录。
//
// 官方 Model schema 只有 id/object/created/owned_by 四个字段;
// context_window 等是本网关的扩展,便于客户端选型时不必硬编码。
// 值未知时留 0 并省略,不编造 —— 上游的真实上下文窗口取决于账号档位。
type modelObject struct {
	ID              string `json:"id"`
	Object          string `json:"object"`
	Created         int64  `json:"created"`
	OwnedBy         string `json:"owned_by"`
	ContextWindow   int    `json:"context_window,omitempty"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
	Type            string `json:"type,omitempty"`
}

func toModelObject(m *modelpkg.Model) modelObject {
	return modelObject{
		ID:              m.Slug,
		Object:          "model",
		Created:         m.CreatedAt.Unix(),
		OwnedBy:         "chatgpt",
		ContextWindow:   m.ContextWindow,
		MaxOutputTokens: m.MaxOutputTokens,
		Type:            m.Type,
	}
}

// ListModels GET /v1/models。
//
// 两处修正:
//   - 按 API Key 的模型白名单过滤。此前列出全部启用模型,客户端下拉框里
//     会出现点了就 403 的条目。
//   - 按 id 排序。Registry 缓存命中时遍历的是 map,Go 的 map 迭代顺序随机,
//     同一把 key 连续请求两次拿到的顺序不一样。
func (h *Handler) ListModels(c *gin.Context) {
	list, err := h.visibleModels(c)
	if err != nil {
		openAIError(c, http.StatusInternalServerError, "list_models_error", "获取模型列表失败:"+err.Error())
		return
	}
	data := make([]modelObject, 0, len(list))
	for _, m := range list {
		data = append(data, toModelObject(m))
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

// RetrieveModel GET /v1/models/:model。
//
// 官方有这个端点,不少 SDK 与中转层会在连接时用它探测模型可用性;
// 此前落到裸 404(纯 API 部署下甚至是空 body)。
func (h *Handler) RetrieveModel(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("model"))
	if slug == "" {
		openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeInvalidRequestError, "model", "缺少模型 id")
		return
	}
	m, err := h.Models.BySlug(c.Request.Context(), slug)
	if err != nil || m == nil || !m.Enabled {
		openAIErrorParam(c, http.StatusNotFound, oaierr.CodeModelNotFound, "model",
			"模型 "+slug+" 不存在或已下架")
		return
	}
	if ak, ok := apikey.FromCtx(c); ok && !ak.ModelAllowed(slug) {
		openAIErrorParam(c, http.StatusForbidden, oaierr.CodeModelNotAllowed, "model",
			"当前 API Key 无权调用模型 "+slug)
		return
	}
	c.JSON(http.StatusOK, toModelObject(m))
}

// visibleModels 返回当前 API Key 实际可用的模型,按 id 稳定排序。
func (h *Handler) visibleModels(c *gin.Context) ([]*modelpkg.Model, error) {
	list, err := h.Models.ListEnabled(c.Request.Context())
	if err != nil {
		return nil, err
	}
	ak, hasKey := apikey.FromCtx(c)
	out := make([]*modelpkg.Model, 0, len(list))
	for _, m := range list {
		if hasKey && !ak.ModelAllowed(m.Slug) {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}
