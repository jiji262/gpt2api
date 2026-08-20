package chatgpt

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChatMessage 是 OpenAI 风格的一条消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Attachments 是随这条消息一起发给上游的已上传文件(vision 输入)。
	// 为空时消息按纯文本 content_type=text 发送,与此前行为完全一致。
	Attachments []*UploadedFile `json:"-"`
}

// ConversationOpts 是 StreamConversation 的参数。
type ConversationOpts struct {
	Model       string        // 上游模型 slug(如 auto / gpt-4o / o4-mini)
	Messages    []ChatMessage // OpenAI 风格消息
	ParentMsgID string        // 可选,为空自动生成
	ConvID      string        // 可选,为空则新会话
	ProofToken  string        // 可选,POW 解出后填入
	ChatToken   string        // 必传(来自 ChatRequirements)
	ReadTimeout time.Duration // SSE 读超时(单次事件间隔),默认 60s
}

// conversationPayload 对齐 chatgpt.com 请求体(文本模式)。
type conversationPayload struct {
	Action                     string                 `json:"action"`
	Messages                   []upstreamMsg          `json:"messages"`
	ParentMessageID            string                 `json:"parent_message_id"`
	ConversationID             string                 `json:"conversation_id,omitempty"`
	Model                      string                 `json:"model"`
	TimezoneOffsetMin          int                    `json:"timezone_offset_min"`
	Suggestions                []string               `json:"suggestions"`
	HistoryAndTrainingDisabled bool                   `json:"history_and_training_disabled"`
	ConversationMode           map[string]interface{} `json:"conversation_mode"`
	ForceParagen               bool                   `json:"force_paragen"`
	ForceParagenModelSlug      string                 `json:"force_paragen_model_slug"`
	ForceNulligen              bool                   `json:"force_nulligen"`
	ForceRateLimit             bool                   `json:"force_rate_limit"`
	WebsocketRequestID         string                 `json:"websocket_request_id"`
	ClientContextualInfo       map[string]interface{} `json:"client_contextual_info,omitempty"`
	PluginIDs                  []string               `json:"plugin_ids,omitempty"`
}

type upstreamMsg struct {
	ID         string          `json:"id"`
	Author     upstreamAuthor  `json:"author"`
	Content    upstreamContent `json:"content"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	CreateTime float64         `json:"create_time,omitempty"`
}

type upstreamAuthor struct {
	Role string `json:"role"`
}

type upstreamContent struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
}

// StreamConversation 向 /backend-api/conversation 发 SSE,返回事件 channel。
// 调用方必须消费完 channel(或 cancel ctx)以释放连接。
func (c *Client) StreamConversation(ctx context.Context, opt ConversationOpts) (<-chan SSEEvent, error) {
	if opt.ChatToken == "" {
		return nil, errors.New("chat_token required")
	}
	if opt.Model == "" {
		opt.Model = "auto"
	}
	if opt.ParentMsgID == "" {
		opt.ParentMsgID = uuid.NewString()
	}
	if opt.ReadTimeout == 0 {
		opt.ReadTimeout = c.opts.SSETimeout
	}

	payload := conversationPayload{
		Action:                     "next",
		Model:                      opt.Model,
		ParentMessageID:            opt.ParentMsgID,
		ConversationID:             opt.ConvID,
		TimezoneOffsetMin:          -480, // UTC+8
		HistoryAndTrainingDisabled: false,
		ConversationMode:           map[string]interface{}{"kind": "primary_assistant"},
		WebsocketRequestID:         uuid.NewString(),
	}
	for _, m := range opt.Messages {
		payload.Messages = append(payload.Messages, upstreamMsg{
			ID:         uuid.NewString(),
			Author:     upstreamAuthor{Role: m.Role},
			Content:    upstreamContent{ContentType: "text", Parts: []string{m.Content}},
			CreateTime: float64(time.Now().Unix()),
		})
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		c.opts.BaseURL+"/backend-api/conversation",
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	c.commonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Openai-Sentinel-Chat-Requirements-Token", opt.ChatToken)
	if opt.ProofToken != "" {
		req.Header.Set("Openai-Sentinel-Proof-Token", opt.ProofToken)
	}

	// 对 SSE 请求取消客户端整体 timeout,改为 per-event 读超时控制。
	localClient := *c.hc
	localClient.Timeout = 0

	res, err := localClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		buf, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, &UpstreamError{Status: res.StatusCode, Message: "conversation failed", Body: string(buf)}
	}

	out := make(chan SSEEvent, 32)
	go parseSSE(ctx, res.Body, out, opt.ReadTimeout)
	return out, nil
}

// parseSSE 读取 SSE 流,把每个 data: 事件推入 channel。
// chatgpt.com 的事件格式:
//
//	event: delta\n
//	data: {"p":"...","o":"append","v":"..."}\n\n
//
//	data: [DONE]\n\n
//
// ctx 必须传请求上下文。消费方在拿到结束标记后会直接 break,不会把 channel 读干;
// 此前 out <- 是无条件阻塞发送,这时生产者会永久卡住,defer r.Close() 永不执行,
// goroutine 和上游连接一起泄漏。加了 max_tokens 软截断后这条路径会被高频命中。
//
// readTimeout 是两次事件之间的最大间隔。此前该参数被直接丢弃,
// 配置项 gateway.sse_read_timeout_sec 是死配置;现在用看门狗关闭底层连接来兑现它。
func parseSSE(ctx context.Context, r io.ReadCloser, out chan<- SSEEvent, readTimeout time.Duration) {
	defer close(out)
	defer r.Close()

	if readTimeout <= 0 {
		readTimeout = 60 * time.Second
	}
	beat := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	// 看门狗:超时或上下文取消时关闭 body,让阻塞中的 ReadString 立刻返回错误。
	// http.Response.Body.Close 可以与 Read 并发调用,这是标准库承诺的解阻塞手段。
	go func() {
		t := time.NewTimer(readTimeout)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				_ = r.Close()
				return
			case <-beat:
				if !t.Stop() {
					select {
					case <-t.C:
					default:
					}
				}
				t.Reset(readTimeout)
			case <-t.C:
				_ = r.Close()
				return
			}
		}
	}()

	rd := bufio.NewReaderSize(r, 32*1024)
	var event string
	var dataBuf strings.Builder

	// send 在上下文取消时放弃发送,避免消费方提前 break 造成的永久阻塞。
	send := func(ev SSEEvent) bool {
		select {
		case out <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}

	flush := func() bool {
		if dataBuf.Len() == 0 {
			event = ""
			return true
		}
		data := strings.TrimRight(dataBuf.String(), "\n")
		dataBuf.Reset()
		ok := send(SSEEvent{Event: event, Data: []byte(data)})
		event = ""
		return ok
	}

	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil {
				// 消费方已经走了,不必再报错。
				return
			}
			if err != io.EOF {
				send(SSEEvent{Err: fmt.Errorf("sse read: %w", err)})
			} else {
				flush()
			}
			return
		}
		select {
		case beat <- struct{}{}:
		default:
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// 事件边界
			if !flush() {
				return
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			// 注释/心跳,忽略
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			s := strings.TrimPrefix(line, "data:")
			if len(s) > 0 && s[0] == ' ' {
				s = s[1:]
			}
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(s)
			continue
		}
		// 其他行忽略
	}
}
