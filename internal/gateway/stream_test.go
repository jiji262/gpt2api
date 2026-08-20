package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jiji262/gpt2api/internal/upstream/chatgpt"
)

var errUpstreamTest = errors.New("sse read: connection reset by peer")

// ---------------------------------------------------------------- 夹具

// sseFrom 把一串上游 data 载荷灌进 channel 并关闭。
func sseFrom(payloads ...string) <-chan chatgpt.SSEEvent {
	ch := make(chan chatgpt.SSEEvent, len(payloads))
	for _, p := range payloads {
		ch <- chatgpt.SSEEvent{Data: []byte(p)}
	}
	close(ch)
	return ch
}

// sseThenErr 先发若干正常帧,再发一个错误帧(模拟上游中途断开)。
func sseThenErr(err error, payloads ...string) <-chan chatgpt.SSEEvent {
	ch := make(chan chatgpt.SSEEvent, len(payloads)+1)
	for _, p := range payloads {
		ch <- chatgpt.SSEEvent{Data: []byte(p)}
	}
	ch <- chatgpt.SSEEvent{Err: err}
	close(ch)
	return ch
}

const (
	frameHello  = `{"v":"hello","p":"/message/content/parts/0"}`
	frameWorld  = `{"v":" world","p":"/message/content/parts/0"}`
	frameFinish = `{"v":"finished_successfully","p":"/message/status"}`
	frameNoText = `{"v":{"message":{"recipient":"all","content":{"parts":[""]}}}}`
)

func newStreamCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, w
}

// sseLines 把响应体切成一条条 SSE 载荷(去掉 "data: " 前缀)。
func sseLines(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	for _, blk := range strings.Split(body, "\n\n") {
		blk = strings.TrimSpace(blk)
		if blk == "" {
			continue
		}
		if !strings.HasPrefix(blk, "data: ") {
			t.Fatalf("非法 SSE 块: %q", blk)
		}
		out = append(out, strings.TrimPrefix(blk, "data: "))
	}
	return out
}

func decodeChunk(t *testing.T, s string) ChatCompletionChunk {
	t.Helper()
	var ck ChatCompletionChunk
	if err := json.Unmarshal([]byte(s), &ck); err != nil {
		t.Fatalf("chunk 解析失败: %v (%s)", err, s)
	}
	return ck
}

func baseRC() respCtx {
	return respCtx{ID: "chatcmpl-test", Model: "gpt-5", PromptTokens: 11, Created: 1700000000}
}

// ---------------------------------------------------------------- 流式正常路径

func TestStreamHappyPath(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).streamOpenAI(c, baseRC(), sseFrom(frameHello, frameWorld, frameFinish))

	lines := sseLines(t, w.Body.String())
	if lines[len(lines)-1] != "[DONE]" {
		t.Fatalf("最后一行必须是 [DONE],实际 %q", lines[len(lines)-1])
	}
	first := decodeChunk(t, lines[0])
	if first.Object != "chat.completion.chunk" {
		t.Errorf("object = %q", first.Object)
	}
	if first.Choices[0].Delta.Role != "assistant" {
		t.Errorf("首 chunk 应带 role=assistant,实际 %#v", first.Choices[0].Delta)
	}

	var text strings.Builder
	var finish string
	for _, l := range lines[:len(lines)-1] {
		ck := decodeChunk(t, l)
		if len(ck.Choices) == 0 {
			continue
		}
		text.WriteString(ck.Choices[0].Delta.Content)
		if ck.Choices[0].FinishReason != nil {
			finish = *ck.Choices[0].FinishReason
		}
	}
	if text.String() != "hello world" {
		t.Errorf("正文 = %q", text.String())
	}
	if finish != "stop" {
		t.Errorf("finish_reason = %q, want stop", finish)
	}
	if out.FinishReason != "stop" || out.Failure != nil {
		t.Errorf("outcome = %#v", out)
	}
}

// TestStreamCreatedIsStable 覆盖 U35:created 此前每帧调一次 time.Now,
// 同一条响应的时间戳会漂移,按 (id, created) 去重的侧车会重复计数。
func TestStreamCreatedIsStable(t *testing.T) {
	c, w := newStreamCtx()
	(&Handler{}).streamOpenAI(c, baseRC(), sseFrom(frameHello, frameWorld, frameFinish))

	lines := sseLines(t, w.Body.String())
	for _, l := range lines[:len(lines)-1] {
		if got := decodeChunk(t, l).Created; got != 1700000000 {
			t.Fatalf("created 漂移: %d", got)
		}
	}
}

func TestStreamSetsSSEHeaders(t *testing.T) {
	c, w := newStreamCtx()
	(&Handler{}).streamOpenAI(c, baseRC(), sseFrom(frameHello, frameFinish))

	want := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	}
	for k, v := range want {
		if got := w.Header().Get(k); !strings.Contains(got, v) {
			t.Errorf("header %s = %q, want 含 %q", k, got, v)
		}
	}
}

// ---------------------------------------------------------------- include_usage

func TestStreamIncludeUsageEmitsFinalUsageChunk(t *testing.T) {
	rc := baseRC()
	rc.IncludeUsage = true
	c, w := newStreamCtx()
	(&Handler{}).streamOpenAI(c, rc, sseFrom(frameHello, frameWorld, frameFinish))

	lines := sseLines(t, w.Body.String())
	if lines[len(lines)-1] != "[DONE]" {
		t.Fatalf("最后应是 [DONE]")
	}
	last := decodeChunk(t, lines[len(lines)-2])
	if len(last.Choices) != 0 {
		t.Errorf("usage chunk 的 choices 必须是空数组,实际 %#v", last.Choices)
	}
	if last.Usage == nil {
		t.Fatal("usage chunk 缺 usage")
	}
	if last.Usage.PromptTokens != 11 {
		t.Errorf("prompt_tokens = %d, want 11", last.Usage.PromptTokens)
	}
	if last.Usage.CompletionTokens <= 0 {
		t.Errorf("completion_tokens = %d, 应大于 0", last.Usage.CompletionTokens)
	}
	if last.Usage.TotalTokens != last.Usage.PromptTokens+last.Usage.CompletionTokens {
		t.Errorf("total 不等于两者之和: %#v", last.Usage)
	}
	// 中间的内容 chunk 不应带 usage。
	for _, l := range lines[:len(lines)-2] {
		if decodeChunk(t, l).Usage != nil {
			t.Errorf("非末尾 chunk 不该带 usage: %s", l)
		}
	}
}

func TestStreamWithoutIncludeUsageHasNoUsageChunk(t *testing.T) {
	c, w := newStreamCtx()
	(&Handler{}).streamOpenAI(c, baseRC(), sseFrom(frameHello, frameFinish))

	for _, l := range sseLines(t, w.Body.String()) {
		if l == "[DONE]" {
			continue
		}
		if decodeChunk(t, l).Usage != nil {
			t.Errorf("没要 include_usage 就不该发 usage: %s", l)
		}
	}
}

// ---------------------------------------------------------------- 中断与空输出

// TestStreamUpstreamErrorEmitsErrorEvent 覆盖 F9:
// 此前上游中途断开也照发 finish_reason:"stop" + [DONE],客户端把半截回答当完整答案。
func TestStreamUpstreamErrorEmitsErrorEvent(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).streamOpenAI(c, baseRC(),
		sseThenErr(errUpstreamTest, frameHello))

	lines := sseLines(t, w.Body.String())
	body := w.Body.String()
	if strings.Contains(body, `"finish_reason":"stop"`) {
		t.Error("上游中断不得伪装成正常结束")
	}
	var sawError bool
	for _, l := range lines {
		if strings.Contains(l, `"error"`) {
			sawError = true
			var env struct {
				Error struct {
					Message string  `json:"message"`
					Type    string  `json:"type"`
					Code    *string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(l), &env); err != nil {
				t.Fatalf("error 事件不是合法信封: %v", err)
			}
			if env.Error.Type != "server_error" {
				t.Errorf("type = %q", env.Error.Type)
			}
		}
	}
	if !sawError {
		t.Fatalf("必须下发 error 事件,实际:\n%s", body)
	}
	if out.Failure == nil {
		t.Error("outcome 应标记失败,供上层退款")
	}
}

// TestStreamClosedWithoutFinalIsInterrupted 覆盖"上游连接被静默掐断"的场景:
// channel 正常关闭但从没收到结束标记。
func TestStreamClosedWithoutFinalIsInterrupted(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).streamOpenAI(c, baseRC(), sseFrom(frameHello))

	if strings.Contains(w.Body.String(), `"finish_reason":"stop"`) {
		t.Error("没收到结束标记就不能报 stop")
	}
	if out.Failure == nil {
		t.Fatal("应判定为中断")
	}
	if out.Failure.Code != "upstream_interrupted" {
		t.Errorf("code = %q", out.Failure.Code)
	}
}

// TestStreamEmptyOutputIsError 覆盖 F8:
// 此前把中文运维文案当 assistant 正文发出去,HTTP 200 + 照常计费。
func TestStreamEmptyOutputIsError(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).streamOpenAI(c, baseRC(), sseFrom(frameNoText, frameFinish))

	body := w.Body.String()
	if strings.Contains(body, "请联系管理员") || strings.Contains(body, "请稍后重试") {
		if !strings.Contains(body, `"error"`) {
			t.Error("运维文案只能出现在 error 事件里,不能当 assistant 正文")
		}
	}
	if !strings.Contains(body, `"error"`) {
		t.Fatalf("零输出必须以 error 事件收尾:\n%s", body)
	}
	if out.Failure == nil || out.Failure.Code != "upstream_empty_output" {
		t.Fatalf("outcome = %#v", out)
	}
}

// TestStreamTruncatesAtMaxTokens 覆盖 U21:max_tokens 此前只影响计费不截断输出。
func TestStreamTruncatesAtMaxTokens(t *testing.T) {
	rc := baseRC()
	rc.MaxTokens = 1 // 约 4 字符
	c, w := newStreamCtx()
	out := (&Handler{}).streamOpenAI(c, rc, sseFrom(frameHello, frameWorld, frameFinish))

	if out.FinishReason != "length" {
		t.Errorf("finish_reason = %q, want length", out.FinishReason)
	}
	if !strings.Contains(w.Body.String(), `"finish_reason":"length"`) {
		t.Errorf("流里应发 length:\n%s", w.Body.String())
	}
	if out.Failure != nil {
		t.Errorf("截断是正常结束,不是失败: %#v", out.Failure)
	}
}

// ---------------------------------------------------------------- 非流式

func TestCollectHappyPath(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).collectOpenAI(c, baseRC(), sseFrom(frameHello, frameWorld, frameFinish))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v (%s)", err, w.Body.String())
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %q", resp.Object)
	}
	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Errorf("id 前缀不对: %q", resp.ID)
	}
	if resp.Choices[0].Message.Content != "hello world" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %q", resp.Choices[0].FinishReason)
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 11 {
		t.Fatalf("usage = %#v", resp.Usage)
	}
	if resp.Usage.TotalTokens != resp.Usage.PromptTokens+resp.Usage.CompletionTokens {
		t.Errorf("total 不等于两者之和: %#v", resp.Usage)
	}
	if out.Failure != nil {
		t.Errorf("不该失败: %#v", out.Failure)
	}
}

// TestCollectEmptyOutputReturns502 覆盖 F8 的非流式分支。
func TestCollectEmptyOutputReturns502(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).collectOpenAI(c, baseRC(), sseFrom(frameNoText, frameFinish))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	e := decodeErr(t, w.Body.Bytes())
	if e["code"] != "upstream_empty_output" {
		t.Errorf("code = %v", e["code"])
	}
	if e["type"] != "server_error" {
		t.Errorf("type = %v", e["type"])
	}
	if out.Failure == nil {
		t.Error("outcome 应标记失败")
	}
}

func TestCollectInterruptedReturns502(t *testing.T) {
	c, w := newStreamCtx()
	out := (&Handler{}).collectOpenAI(c, baseRC(), sseThenErr(errUpstreamTest, frameHello))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	if out.Failure == nil || out.Failure.Code != "upstream_interrupted" {
		t.Fatalf("outcome = %#v", out)
	}
}

func TestCollectTruncatesAtMaxTokens(t *testing.T) {
	rc := baseRC()
	rc.MaxTokens = 1
	c, w := newStreamCtx()
	out := (&Handler{}).collectOpenAI(c, rc, sseFrom(frameHello, frameWorld, frameFinish))

	var resp ChatCompletionResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Choices[0].FinishReason != "length" {
		t.Errorf("finish_reason = %q, want length", resp.Choices[0].FinishReason)
	}
	if out.Failure != nil {
		t.Errorf("截断不是失败")
	}
}

// TestResponseMessageHasProtocolKeys 覆盖 U35:
// refusal / annotations 是 OpenAI 协议规定必须存在的键(值可为 null)。
// schema 校验型的中间层按缺键报错,不是按 null 报错。
func TestResponseMessageHasProtocolKeys(t *testing.T) {
	c, w := newStreamCtx()
	(&Handler{}).collectOpenAI(c, baseRC(), sseFrom(frameHello, frameFinish))

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	choice := raw["choices"].([]interface{})[0].(map[string]interface{})
	msg := choice["message"].(map[string]interface{})

	for _, k := range []string{"role", "content", "refusal", "annotations"} {
		if _, ok := msg[k]; !ok {
			t.Errorf("message 缺键 %q: %s", k, w.Body.String())
		}
	}
	if msg["refusal"] != nil {
		t.Errorf("refusal 应为 null,实际 %v", msg["refusal"])
	}
	if _, ok := choice["logprobs"]; !ok {
		t.Errorf("choice 缺 logprobs 键: %s", w.Body.String())
	}
}
