// responses.go —— /v1/responses 的单向转译垫片。
//
// 为什么要做:多个客户端按**模型名**硬路由到 Responses,无视 base_url 配置。
// langchain-openai 对任何含 "codex" 的模型名强制走 Responses;LobeChat 对
// gpt-5.x 的部分档位强制走且用户设置覆盖不了;Vercel AI SDK 的 openai('id')
// 默认就是 Responses。这些请求打过来只会拿到 404,用户完全不知道为什么。
//
// 为什么是"单向"垫片:Responses 的服务端状态(previous_response_id /
// conversation / background)需要一个本地会话状态层,本仓库没有 ——
// 出图完即 PATCH is_visible=false,chat 每次新开会话。所以这些字段一律明确拒绝,
// 而不是假装支持后给出错误结果。
//
// 与 Chat Completions 的关键差异(实现时最容易搞错的三点):
//  1. 流式用**具名事件**(event: response.output_text.delta),不是匿名 data chunk
//  2. **没有 [DONE] 终止符** —— 官方 spec 里 7 处 [DONE] 全在 Assistants 和
//     chat 的 stream_options 描述里,Responses 一处都没有
//  3. usage 字段名是 input_tokens / output_tokens,不是 prompt_tokens / completion_tokens
package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jiji262/gpt2api/internal/apikey"
	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
	"github.com/jiji262/gpt2api/pkg/oaierr"
)

// ResponsesRequest 是 POST /v1/responses 的请求体子集。
type ResponsesRequest struct {
	Model           string            `json:"model" binding:"required"`
	Input           json.RawMessage   `json:"input"`
	Instructions    string            `json:"instructions,omitempty"`
	Stream          bool              `json:"stream,omitempty"`
	MaxOutputTokens *int              `json:"max_output_tokens,omitempty"`
	Temperature     *float64          `json:"temperature,omitempty"`
	TopP            *float64          `json:"top_p,omitempty"`
	Store           *bool             `json:"store,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Text            *responsesText    `json:"text,omitempty"`
	Reasoning       json.RawMessage   `json:"reasoning,omitempty"`
	Tools           json.RawMessage   `json:"tools,omitempty"`
	ToolChoice      json.RawMessage   `json:"tool_choice,omitempty"`
	Include         []string          `json:"include,omitempty"`
	Truncation      string            `json:"truncation,omitempty"`

	// ---- 需要服务端状态,本网关一律拒绝 ----
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
	Conversation       json.RawMessage `json:"conversation,omitempty"`
	Background         *bool           `json:"background,omitempty"`
}

type responsesText struct {
	Format *struct {
		Type string `json:"type"`
	} `json:"format,omitempty"`
	Verbosity string `json:"verbosity,omitempty"`
}

// Responses 是 POST /v1/responses 入口。
func (h *Handler) Responses(c *gin.Context) {
	ak, ok := apikey.FromCtx(c)
	if !ok {
		openAIError(c, http.StatusUnauthorized, "missing_api_key", "缺少 API Key")
		return
	}

	var req ResponsesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		param, msg := bindErrorMessage(err)
		openAIErrorParam(c, http.StatusBadRequest, oaierr.CodeInvalidRequestError, param, msg)
		return
	}

	chatReq, param, why := translateResponsesRequest(&req)
	if param != "" {
		writeUnsupportedParam(c, param, why)
		return
	}
	// 转译后的请求仍要过同一套 chat 参数校验,保证两个端点的拒绝口径一致。
	verdict := validateChatRequestWithVision(chatReq, h.visionEnabled())
	if verdict.RejectParam != "" {
		writeUnsupportedParam(c, verdict.RejectParam, verdict.RejectMessage)
		return
	}
	// Responses 特有的软忽略项:上游不接受截断策略,也不产出 reasoning/logprobs
	// 之类可 include 的附加数据。回显 truncation 而不说明会让调用方
	// 以为超长上下文被自动截断了。
	setIgnoredParamsHeader(c, append(verdict.Ignored, responsesIgnored(&req)...))

	h.runChat(c, ak, chatReq, responsesRenderer{h: h, req: &req})
}

// responsesIgnored 收集 Responses 独有的、传了也不生效的参数。
func responsesIgnored(r *ResponsesRequest) []string {
	var out []string
	if t := strings.TrimSpace(r.Truncation); t != "" && t != "disabled" {
		out = append(out, "truncation")
	}
	if len(r.Include) > 0 {
		out = append(out, "include")
	}
	return out
}

// translateResponsesRequest 把 Responses 请求翻成等价的 Chat 请求。
// 返回非空 param 表示该字段做不到,需要明确拒绝。
func translateResponsesRequest(r *ResponsesRequest) (*ChatCompletionsRequest, string, string) {
	if r.PreviousResponseID != "" {
		return nil, "previous_response_id", "本网关不持有服务端会话状态,请把完整对话历史放进 input"
	}
	if rawPresent(r.Conversation) {
		return nil, "conversation", "本网关不持有服务端会话状态"
	}
	if r.Background != nil && *r.Background {
		return nil, "background", "本网关不支持后台异步任务,请用 stream 或同步等待"
	}
	if r.Store != nil && *r.Store {
		return nil, "store", "本网关不留存响应,无法按 id 回查"
	}

	msgs, err := responsesInputToMessages(r.Input)
	if err != nil {
		return nil, "input", err.Error()
	}
	if r.Instructions != "" {
		// instructions 等价于置顶的 system 消息。
		msgs = append([]RequestMessage{{
			Role:    roleSystem,
			Content: MessageContent{Text: r.Instructions},
		}}, msgs...)
	}
	if len(msgs) == 0 {
		return nil, "input", "不能为空"
	}

	out := &ChatCompletionsRequest{
		Model:       r.Model,
		Messages:    msgs,
		Stream:      r.Stream,
		Temperature: r.Temperature,
		TopP:        r.TopP,
		Metadata:    r.Metadata,
		Tools:       r.Tools,
		ToolChoice:  r.ToolChoice,
	}
	if r.MaxOutputTokens != nil {
		out.MaxTokens = *r.MaxOutputTokens
	}
	if r.Text != nil {
		out.Verbosity = r.Text.Verbosity
		if r.Text.Format != nil && r.Text.Format.Type != "" && r.Text.Format.Type != "text" {
			// 交给 chat 那套校验统一报错,param 名保持 Responses 侧的写法。
			return nil, "text.format", fmt.Sprintf(
				"上游没有 schema 约束通道,%q 无法保证输出格式。请在 instructions 里要求 JSON 并对结果做容错解析",
				r.Text.Format.Type)
		}
	}
	if rawPresent(r.Reasoning) {
		out.ReasoningEffort = "requested" // 只为进软忽略清单,值本身不下发
	}
	return out, "", ""
}

// responsesInputItem 是 input 数组里的一项。
type responsesInputItem struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// responsesInputToMessages 把 input 归一成 chat messages。
// input 可以是裸字符串(等价于单条 user 消息)或 item 数组。
func responsesInputToMessages(raw json.RawMessage) ([]RequestMessage, error) {
	if !rawPresent(raw) {
		return nil, nil
	}
	s := strings.TrimSpace(string(raw))
	if s[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		return []RequestMessage{{Role: roleUser, Content: MessageContent{Text: text}}}, nil
	}
	if s[0] != '[' {
		return nil, fmt.Errorf("input 必须是字符串或数组")
	}

	var items []responsesInputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	out := make([]RequestMessage, 0, len(items))
	for _, it := range items {
		// 非 message 的 item(function_call / reasoning / file_search_call ...)
		// 依赖上游没有的能力,交由 chat 校验层统一拒绝会更晦涩,这里直接说清楚。
		if it.Type != "" && it.Type != "message" {
			return nil, fmt.Errorf("暂不支持 %q 类型的 input item,上游只接受纯文本消息", it.Type)
		}
		// content 可能是字符串,也可能是 Responses 特有的 part 数组
		// (input_text / output_text,而不是 chat 的 text)。
		// 不能直接复用 MessageContent 的解析:它只认 "text",
		// 会把 input_text 当成非文本 part,导致纯文本请求被误拒。
		mc, cerr := responsesMessageContent(it.Content)
		if cerr != nil {
			return nil, cerr
		}
		role := it.Role
		if role == "" {
			role = roleUser
		}
		out = append(out, RequestMessage{Role: role, Content: mc})
	}
	return out, nil
}

// responsesMessageContent 把一条 item 的 content 归一成 MessageContent。
func responsesMessageContent(raw json.RawMessage) (MessageContent, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return MessageContent{}, nil
	}
	if s[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return MessageContent{}, err
		}
		return MessageContent{Text: text}, nil
	}
	text, parts, err := responsesContentParts(raw)
	if err != nil {
		return MessageContent{}, err
	}
	return MessageContent{Text: text, NonTextParts: parts}, nil
}

// responsesContentParts 解析 Responses 特有的 content part 类型。
func responsesContentParts(raw json.RawMessage) (string, []string, error) {
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", nil, fmt.Errorf("content 解析失败: %w", err)
	}
	var texts []string
	var nonText []string
	seen := map[string]bool{}
	for _, p := range parts {
		switch p.Type {
		case "input_text", "output_text", "text", "summary_text":
			texts = append(texts, p.Text)
		default:
			if p.Type != "" && !seen[p.Type] {
				seen[p.Type] = true
				nonText = append(nonText, p.Type)
			}
		}
	}
	return strings.Join(texts, "\n"), nonText, nil
}

// ---------------------------------------------------------------- 响应对象

// ResponsesUsage 注意字段名与 chat 不同:input_tokens / output_tokens。
type ResponsesUsage struct {
	InputTokens         int            `json:"input_tokens"`
	OutputTokens        int            `json:"output_tokens"`
	TotalTokens         int            `json:"total_tokens"`
	InputTokensDetails  map[string]int `json:"input_tokens_details"`
	OutputTokensDetails map[string]int `json:"output_tokens_details"`
}

type responsesContentPart struct {
	Type        string   `json:"type"`
	Text        string   `json:"text"`
	Annotations []string `json:"annotations"`
}

type responsesOutputItem struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id"`
	Status  string                 `json:"status"`
	Role    string                 `json:"role"`
	Content []responsesContentPart `json:"content"`
}

// ResponsesObject 是 /v1/responses 的响应体。
type ResponsesObject struct {
	ID                 string                `json:"id"`
	Object             string                `json:"object"`
	CreatedAt          int64                 `json:"created_at"`
	Status             string                `json:"status"`
	Model              string                `json:"model"`
	Output             []responsesOutputItem `json:"output"`
	Usage              *ResponsesUsage       `json:"usage"`
	Error              *oaierr.Payload       `json:"error"`
	IncompleteDetails  map[string]string     `json:"incomplete_details"`
	Instructions       *string               `json:"instructions"`
	MaxOutputTokens    *int                  `json:"max_output_tokens"`
	PreviousResponseID *string               `json:"previous_response_id"`
	Metadata           map[string]string     `json:"metadata"`
	Temperature        *float64              `json:"temperature"`
	TopP               *float64              `json:"top_p"`
	Truncation         string                `json:"truncation"`
	ParallelToolCalls  bool                  `json:"parallel_tool_calls"`
	Tools              []struct{}            `json:"tools"`
}

// 响应状态。
const (
	responseCompleted  = "completed"
	responseInProgress = "in_progress"
	responseIncomplete = "incomplete"
	responseFailed     = "failed"
)

// newResponsesObject 建一个骨架,回显请求参数。
func newResponsesObject(req *ResponsesRequest, rc respCtx, status string) *ResponsesObject {
	o := &ResponsesObject{
		ID:          "resp_" + strings.TrimPrefix(rc.ID, "chatcmpl-"),
		Object:      "response",
		CreatedAt:   rc.Created,
		Status:      status,
		Model:       rc.Model,
		Output:      []responsesOutputItem{},
		Metadata:    req.Metadata,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		// 回显固定为 disabled:上游没有截断策略通道,
		// 原样回显用户传的值会让人以为它生效了。
		Truncation:        "disabled",
		ParallelToolCalls: true,
		Tools:             []struct{}{},
		MaxOutputTokens:   req.MaxOutputTokens,
	}
	if o.Metadata == nil {
		o.Metadata = map[string]string{}
	}
	if req.Instructions != "" {
		v := req.Instructions
		o.Instructions = &v
	}
	return o
}

func responsesUsageOf(rc respCtx, outputTokens int) *ResponsesUsage {
	return &ResponsesUsage{
		InputTokens:         rc.PromptTokens,
		OutputTokens:        outputTokens,
		TotalTokens:         rc.PromptTokens + outputTokens,
		InputTokensDetails:  map[string]int{"cached_tokens": 0},
		OutputTokensDetails: map[string]int{"reasoning_tokens": 0},
	}
}

// messageItem 组装唯一的 assistant 输出项。
func messageItem(itemID, text, status string) responsesOutputItem {
	return responsesOutputItem{
		Type:   "message",
		ID:     itemID,
		Status: status,
		Role:   roleAssistant,
		Content: []responsesContentPart{{
			Type:        "output_text",
			Text:        text,
			Annotations: []string{},
		}},
	}
}

// ---------------------------------------------------------------- renderer

// responsesRenderer 按 Responses 协议输出。
type responsesRenderer struct {
	h   *Handler
	req *ResponsesRequest
}

func (r responsesRenderer) Render(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome {
	if rc.Stream {
		return r.renderStream(c, rc, stream)
	}
	return r.renderCollect(c, rc, stream)
}

func (r responsesRenderer) renderCollect(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome {
	var sb strings.Builder
	out := consumeUpstream(rc, stream, func(d string) { sb.WriteString(d) })

	if out.Failure != nil {
		oaierr.Write(c, out.Failure.Status, out.Failure.Code, "", out.Failure.Message)
		return out
	}
	status := responseCompleted
	itemStatus := responseCompleted
	obj := newResponsesObject(r.req, rc, status)
	if out.FinishReason == finishLength {
		obj.Status = responseIncomplete
		obj.IncompleteDetails = map[string]string{"reason": "max_output_tokens"}
		itemStatus = responseIncomplete
	}
	obj.Output = []responsesOutputItem{messageItem("msg_"+uuid.NewString(), sb.String(), itemStatus)}
	obj.Usage = responsesUsageOf(rc, out.CompletionTokens)
	c.JSON(http.StatusOK, obj)
	return out
}

func (r responsesRenderer) renderStream(c *gin.Context, rc respCtx, stream <-chan chatgpt.SSEEvent) streamOutcome {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	seq := 0
	emit := func(event string, payload map[string]interface{}) {
		payload["type"] = event
		payload["sequence_number"] = seq
		seq++
		b, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		if flusher != nil {
			flusher.Flush()
		}
	}

	itemID := "msg_" + uuid.NewString()
	inProgress := newResponsesObject(r.req, rc, responseInProgress)
	emit("response.created", map[string]interface{}{"response": inProgress})
	emit("response.in_progress", map[string]interface{}{"response": inProgress})
	emit("response.output_item.added", map[string]interface{}{
		"output_index": 0,
		"item":         messageItem(itemID, "", responseInProgress),
	})
	emit("response.content_part.added", map[string]interface{}{
		"item_id": itemID, "output_index": 0, "content_index": 0,
		"part": responsesContentPart{Type: "output_text", Annotations: []string{}},
	})

	var sb strings.Builder
	out := consumeUpstream(rc, stream, func(d string) {
		sb.WriteString(d)
		emit("response.output_text.delta", map[string]interface{}{
			"item_id": itemID, "output_index": 0, "content_index": 0, "delta": d,
		})
	})

	if out.Failure != nil {
		failed := newResponsesObject(r.req, rc, responseFailed)
		failed.Error = &oaierr.New(out.Failure.Status, "", out.Failure.Code, "", out.Failure.Message).Error
		emit("response.failed", map[string]interface{}{"response": failed})
		// 注意:Responses 流没有 [DONE] 终止符,response.failed 就是终止事件。
		return out
	}

	itemStatus := responseCompleted
	status := responseCompleted
	if out.FinishReason == finishLength {
		itemStatus, status = responseIncomplete, responseIncomplete
	}
	emit("response.output_text.done", map[string]interface{}{
		"item_id": itemID, "output_index": 0, "content_index": 0, "text": sb.String(),
	})
	emit("response.content_part.done", map[string]interface{}{
		"item_id": itemID, "output_index": 0, "content_index": 0,
		"part": responsesContentPart{Type: "output_text", Text: sb.String(), Annotations: []string{}},
	})
	emit("response.output_item.done", map[string]interface{}{
		"output_index": 0, "item": messageItem(itemID, sb.String(), itemStatus),
	})

	final := newResponsesObject(r.req, rc, status)
	final.Output = []responsesOutputItem{messageItem(itemID, sb.String(), itemStatus)}
	final.Usage = responsesUsageOf(rc, out.CompletionTokens)
	if status == responseIncomplete {
		final.IncompleteDetails = map[string]string{"reason": "max_output_tokens"}
		emit("response.incomplete", map[string]interface{}{"response": final})
	} else {
		emit("response.completed", map[string]interface{}{"response": final})
	}
	return out
}

// ResponsesUnsupported 处理 Responses 的有状态子端点
// (GET/DELETE /v1/responses/{id}、/cancel、input_items)。
func (h *Handler) ResponsesUnsupported(c *gin.Context) {
	oaierr.WriteTyped(c, http.StatusNotImplemented, oaierr.TypeServer,
		oaierr.CodeUnsupportedEndpoint, "",
		"本网关的 /v1/responses 是无状态转译垫片,不留存响应,"+
			"因此不支持按 id 回查、取消或列举 input items。请改用流式或同步等待拿到完整结果。")
}
