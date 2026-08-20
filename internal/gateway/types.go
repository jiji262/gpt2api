package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
)

// ChatCompletionsRequest 对应 OpenAI /v1/chat/completions 请求体。
//
// 字段分三类:
//  1. 真正生效的(model / messages / stream / stream_options / max_tokens / user)
//  2. 上游 chatgpt.com 做不到、但用户明确要了就必须报错的(tools / response_format / n / stop ...)
//  3. 上游做不到、忽略也不影响正确性的(temperature / top_p / store ...)
//
// 第 2、3 类的判定逻辑集中在 params.go 的 validateChatRequest。
// 可选字段一律用指针或 json.RawMessage:必须能区分"没传"和"传了默认值",
// 否则会把客户端无条件带上的 "tools":[] 当成真的要用工具而误拒。
type ChatCompletionsRequest struct {
	Model    string           `json:"model" binding:"required"`
	Messages []RequestMessage `json:"messages" binding:"required"`

	Stream        bool           `json:"stream"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`

	// max_tokens 官方已弃用但存量客户端仍在发;max_completion_tokens 是新名字。
	// 两者都传时以 max_tokens 为准(与官方行为一致)。
	MaxTokens           int  `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

	User string `json:"user,omitempty"`

	// ---- 上游做不到,有实际值就硬拒 ----
	N                *int               `json:"n,omitempty"`
	Stop             json.RawMessage    `json:"stop,omitempty"`
	Seed             *int64             `json:"seed,omitempty"`
	Logprobs         *bool              `json:"logprobs,omitempty"`
	TopLogprobs      *int               `json:"top_logprobs,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	Tools            json.RawMessage    `json:"tools,omitempty"`
	ToolChoice       json.RawMessage    `json:"tool_choice,omitempty"`
	Functions        json.RawMessage    `json:"functions,omitempty"`
	FunctionCall     json.RawMessage    `json:"function_call,omitempty"`
	ResponseFormat   *ResponseFormat    `json:"response_format,omitempty"`
	Modalities       []string           `json:"modalities,omitempty"`
	Audio            json.RawMessage    `json:"audio,omitempty"`
	Prediction       json.RawMessage    `json:"prediction,omitempty"`
	WebSearchOptions json.RawMessage    `json:"web_search_options,omitempty"`

	// ---- 上游做不到,但忽略无害 ----
	Temperature       *float64          `json:"temperature,omitempty"`
	TopP              *float64          `json:"top_p,omitempty"`
	ParallelToolCalls *bool             `json:"parallel_tool_calls,omitempty"`
	ReasoningEffort   string            `json:"reasoning_effort,omitempty"`
	Verbosity         string            `json:"verbosity,omitempty"`
	ServiceTier       string            `json:"service_tier,omitempty"`
	Store             *bool             `json:"store,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	SafetyIdentifier  string            `json:"safety_identifier,omitempty"`
	PromptCacheKey    string            `json:"prompt_cache_key,omitempty"`
}

// StreamOptions 对应 stream_options。
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ResponseFormat 只关心 type:上游没有任何 schema 约束通道,
// json_object / json_schema 一律做不到,text 等于不约束。
type ResponseFormat struct {
	Type string `json:"type"`
}

// RequestMessage 是请求里的一条消息。
type RequestMessage struct {
	Role       string          `json:"role"`
	Content    MessageContent  `json:"content"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
}

// MessageContent 兼容 content 的两种合法形态:字符串,或 content part 数组。
//
// 修复前 content 直接声明成 string,任何多模态请求(openai-python 的
// vision 示例、Cherry Studio 贴图、Open WebUI 传附件)都会在反序列化阶段整体失败。
type MessageContent struct {
	// Text 是所有 text part 按出现顺序换行拼接的结果。
	Text string
	// NonTextParts 记录出现过的非文本 part 类型(去重、保序)。
	// 关闭 vision 时据此明确报错,而不是悄悄只发文字。
	NonTextParts []string
	// ImageSources 是 image_url part 里的 url 值(data: 或 https:)。
	// 只有开启 vision 时才会被消费。
	ImageSources []string
}

type contentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL *struct {
		URL    string `json:"url"`
		Detail string `json:"detail,omitempty"`
	} `json:"image_url,omitempty"`
}

// UnmarshalJSON 接受 string / null / []part 三种形态。
func (m *MessageContent) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	switch b[0] {
	case '"':
		return json.Unmarshal(b, &m.Text)
	case '[':
		var parts []contentPart
		if err := json.Unmarshal(b, &parts); err != nil {
			return err
		}
		var texts []string
		seen := map[string]bool{}
		for _, p := range parts {
			if p.Type == "text" {
				texts = append(texts, p.Text)
				continue
			}
			if p.Type == "image_url" && p.ImageURL != nil && p.ImageURL.URL != "" {
				m.ImageSources = append(m.ImageSources, p.ImageURL.URL)
			}
			if p.Type != "" && !seen[p.Type] {
				seen[p.Type] = true
				m.NonTextParts = append(m.NonTextParts, p.Type)
			}
		}
		m.Text = strings.Join(texts, "\n")
		return nil
	default:
		return errors.New("content 必须是字符串或 content part 数组")
	}
}

// MarshalJSON 让 MessageContent 在日志/回显里仍是普通字符串。
func (m MessageContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Text)
}

// upstreamMessages 把协议层消息转成上游认的 {role, content} 形态。
// developer 是 system 的新名字,上游只认 system。
func (r *ChatCompletionsRequest) upstreamMessages() []chatgpt.ChatMessage {
	out := make([]chatgpt.ChatMessage, 0, len(r.Messages))
	for _, m := range r.Messages {
		role := m.Role
		if role == roleDeveloper {
			role = roleSystem
		}
		out = append(out, chatgpt.ChatMessage{Role: role, Content: m.Content.Text})
	}
	return out
}

// ChatCompletionResponse 非流式响应。
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             *ChatCompletionUsage   `json:"usage"`
	SystemFingerprint string                 `json:"system_fingerprint,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     *struct{}       `json:"logprobs"`
}

// ResponseMessage 是响应里的 assistant 消息。
//
// 单独于上游的 chatgpt.ChatMessage:refusal / annotations 是 OpenAI 协议
// 规定必须存在的键(值可为 null),而上游结构体不该被协议字段污染。
// schema 校验型的中间层会按缺键报错,不是按 null 报错。
type ResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Refusal 恒为 null:上游不区分"拒绝回答"和"正常回答",
	// 被审核拦下时表现为零输出,已由 upstream_empty_output 走错误路径。
	Refusal     *string  `json:"refusal"`
	Annotations []string `json:"annotations"`
}

// assistantMessage 组装一条 assistant 回复。
func assistantMessage(content string) ResponseMessage {
	return ResponseMessage{Role: roleAssistant, Content: content, Annotations: []string{}}
}

type ChatCompletionUsage struct {
	PromptTokens        int                  `json:"prompt_tokens"`
	CompletionTokens    int                  `json:"completion_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	PromptTokensDetails *PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

// PromptTokensDetails 目前只有 cached_tokens,且上游不提供缓存命中信息,恒为 0。
// 保留字段是因为部分成本核算侧车按它做除法,缺键会 panic。
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// ChatCompletionChunk 流式 chunk。
//
// Choices 用 omitempty:stream_options.include_usage 要求最后一个 chunk
// choices 为空数组、usage 为真值,两者在同一结构上表达。
type ChatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
	Usage   *ChatCompletionUsage        `json:"usage,omitempty"`
}

type ChatCompletionChunkChoice struct {
	Index        int      `json:"index"`
	Delta        DeltaMsg `json:"delta"`
	FinishReason *string  `json:"finish_reason"`
}

type DeltaMsg struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
