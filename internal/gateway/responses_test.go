package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustTranslate(t *testing.T, body string) *ChatCompletionsRequest {
	t.Helper()
	var r ResponsesRequest
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("bind: %v", err)
	}
	out, param, why := translateResponsesRequest(&r)
	if param != "" {
		t.Fatalf("不该被拒: %s %s", param, why)
	}
	return out
}

func translateErr(t *testing.T, body string) (string, string) {
	t.Helper()
	var r ResponsesRequest
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("bind: %v", err)
	}
	_, param, why := translateResponsesRequest(&r)
	return param, why
}

// ---------------------------------------------------------------- input 归一

func TestResponsesInputAsString(t *testing.T) {
	req := mustTranslate(t, `{"model":"gpt-5","input":"你好"}`)
	if len(req.Messages) != 1 {
		t.Fatalf("messages = %#v", req.Messages)
	}
	if req.Messages[0].Role != "user" || req.Messages[0].Content.Text != "你好" {
		t.Errorf("msg = %#v", req.Messages[0])
	}
}

// TestResponsesInputTextParts 是最容易写错的一处:
// Responses 的 content part 类型是 input_text / output_text,不是 chat 的 text。
// 直接复用 chat 的解析会把纯文本请求当成多模态而误拒。
func TestResponsesInputTextParts(t *testing.T) {
	req := mustTranslate(t, `{"model":"gpt-5","input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"a"},{"type":"input_text","text":"b"}]}
	]}`)
	if len(req.Messages) != 1 || req.Messages[0].Content.Text != "a\nb" {
		t.Fatalf("msg = %#v", req.Messages)
	}
	if len(req.Messages[0].Content.NonTextParts) != 0 {
		t.Errorf("input_text 不该被当成非文本 part: %v", req.Messages[0].Content.NonTextParts)
	}
}

func TestResponsesInputImageRejectedDownstream(t *testing.T) {
	req := mustTranslate(t, `{"model":"gpt-5","input":[
		{"type":"message","role":"user","content":[{"type":"input_image","image_url":"x"}]}
	]}`)
	// 转译层放行,由统一的 chat 校验层拒绝,保证两个端点口径一致。
	if v := validateChatRequest(req); v.RejectParam == "" {
		t.Fatal("图片 part 应被拒绝")
	}
}

func TestResponsesInstructionsBecomesSystem(t *testing.T) {
	req := mustTranslate(t, `{"model":"gpt-5","instructions":"简洁","input":"hi"}`)
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %#v", req.Messages)
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content.Text != "简洁" {
		t.Errorf("首条应是 system instructions: %#v", req.Messages[0])
	}
}

func TestResponsesMaxOutputTokensMapsToMaxTokens(t *testing.T) {
	req := mustTranslate(t, `{"model":"gpt-5","input":"hi","max_output_tokens":128}`)
	if req.MaxTokens != 128 {
		t.Errorf("MaxTokens = %d", req.MaxTokens)
	}
}

func TestResponsesNonMessageItemRejected(t *testing.T) {
	param, why := translateErr(t, `{"model":"gpt-5","input":[{"type":"function_call","call_id":"c1"}]}`)
	if param != "input" {
		t.Fatalf("param = %q", param)
	}
	if !strings.Contains(why, "function_call") {
		t.Errorf("应指明是哪种 item: %q", why)
	}
}

// ---------------------------------------------------------------- 有状态字段

func TestResponsesStatefulFieldsRejected(t *testing.T) {
	cases := map[string]string{
		`{"model":"gpt-5","input":"hi","previous_response_id":"resp_1"}`: "previous_response_id",
		`{"model":"gpt-5","input":"hi","conversation":"conv_1"}`:         "conversation",
		`{"model":"gpt-5","input":"hi","background":true}`:               "background",
		`{"model":"gpt-5","input":"hi","store":true}`:                    "store",
	}
	for body, want := range cases {
		t.Run(want, func(t *testing.T) {
			param, why := translateErr(t, body)
			if param != want {
				t.Fatalf("param = %q, want %q", param, want)
			}
			if why == "" {
				t.Error("必须说明原因")
			}
		})
	}
}

func TestResponsesStatelessDefaultsPass(t *testing.T) {
	for _, body := range []string{
		`{"model":"gpt-5","input":"hi","store":false}`,
		`{"model":"gpt-5","input":"hi","background":false}`,
		`{"model":"gpt-5","input":"hi","truncation":"disabled"}`,
		`{"model":"gpt-5","input":"hi","text":{"format":{"type":"text"}}}`,
	} {
		if param, why := translateErr(t, body); param != "" {
			t.Errorf("%s 不该被拒: %s %s", body, param, why)
		}
	}
}

func TestResponsesJSONSchemaRejected(t *testing.T) {
	param, _ := translateErr(t, `{"model":"gpt-5","input":"hi","text":{"format":{"type":"json_schema"}}}`)
	if param != "text.format" {
		t.Fatalf("param = %q", param)
	}
}

func TestResponsesToolsRejectedByChatValidator(t *testing.T) {
	req := mustTranslate(t, `{"model":"gpt-5","input":"hi","tools":[{"type":"function","name":"f"}]}`)
	if v := validateChatRequest(req); v.RejectParam != "tools" {
		t.Fatalf("RejectParam = %q", v.RejectParam)
	}
}

// ---------------------------------------------------------------- 响应形状

func TestResponsesUsageFieldNames(t *testing.T) {
	// 字段名与 chat 不同:input_tokens / output_tokens,不是 prompt_/completion_。
	b, err := json.Marshal(responsesUsageOf(respCtx{PromptTokens: 7}, 3))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"input_tokens":7`, `"output_tokens":3`, `"total_tokens":10`,
		`"input_tokens_details"`, `"output_tokens_details"`} {
		if !strings.Contains(s, want) {
			t.Errorf("usage 缺 %s: %s", want, s)
		}
	}
	if strings.Contains(s, "prompt_tokens") {
		t.Errorf("不该出现 chat 的字段名: %s", s)
	}
}

func TestResponsesObjectShape(t *testing.T) {
	req := &ResponsesRequest{Model: "gpt-5", Truncation: ""}
	obj := newResponsesObject(req, respCtx{ID: "chatcmpl-abc", Model: "gpt-5", Created: 1700000000}, responseCompleted)

	if obj.Object != "response" {
		t.Errorf("object = %q", obj.Object)
	}
	if !strings.HasPrefix(obj.ID, "resp_") {
		t.Errorf("id 前缀应是 resp_: %q", obj.ID)
	}
	if obj.CreatedAt != 1700000000 {
		t.Errorf("created_at = %d", obj.CreatedAt)
	}
	if obj.Truncation != "disabled" {
		t.Errorf("truncation 默认应是 disabled: %q", obj.Truncation)
	}

	b, _ := json.Marshal(obj)
	// 这些键即使为空也必须存在:SDK 按 null 判断,不是按缺键。
	for _, want := range []string{`"error":null`, `"usage":null`, `"previous_response_id":null`, `"output":[]`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("缺 %s: %s", want, b)
		}
	}
	if strings.Contains(string(b), `"created":`) {
		t.Errorf("Responses 用 created_at,不是 chat 的 created: %s", b)
	}
}

func TestMessageItemShape(t *testing.T) {
	b, _ := json.Marshal(messageItem("msg_1", "hi", responseCompleted))
	s := string(b)
	for _, want := range []string{`"type":"message"`, `"role":"assistant"`,
		`"type":"output_text"`, `"text":"hi"`, `"annotations":[]`} {
		if !strings.Contains(s, want) {
			t.Errorf("缺 %s: %s", want, s)
		}
	}
}

// ---------------------------------------------------------------- 流式具名事件

// parseNamedSSE 把 "event: X\ndata: {...}" 块拆成 (事件名, 载荷) 序列。
func parseNamedSSE(t *testing.T, body string) ([]string, []map[string]interface{}) {
	t.Helper()
	var names []string
	var payloads []map[string]interface{}
	for _, blk := range strings.Split(body, "\n\n") {
		blk = strings.TrimSpace(blk)
		if blk == "" {
			continue
		}
		lines := strings.SplitN(blk, "\n", 2)
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "event: ") {
			t.Fatalf("Responses 流必须用具名事件,拿到: %q", blk)
		}
		names = append(names, strings.TrimPrefix(lines[0], "event: "))
		var p map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &p); err != nil {
			t.Fatalf("载荷解析失败: %v (%s)", err, lines[1])
		}
		payloads = append(payloads, p)
	}
	return names, payloads
}

func renderResponsesStream(t *testing.T, rc respCtx, payloads ...string) (string, streamOutcome) {
	t.Helper()
	c, w := newStreamCtx()
	rc.Stream = true
	r := responsesRenderer{h: &Handler{}, req: &ResponsesRequest{Model: rc.Model}}
	out := r.Render(c, rc, sseFrom(payloads...))
	return w.Body.String(), out
}

func TestResponsesStreamEventSequence(t *testing.T) {
	body, out := renderResponsesStream(t, baseRC(), frameHello, frameWorld, frameFinish)

	names, payloads := parseNamedSSE(t, body)
	want := []string{
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("事件序列不符:\n got %v\nwant %v", names, want)
	}
	// sequence_number 必须从 0 起单调递增。
	for i, p := range payloads {
		if int(p["sequence_number"].(float64)) != i {
			t.Errorf("第 %d 个事件 sequence_number = %v", i, p["sequence_number"])
		}
		if p["type"] != names[i] {
			t.Errorf("载荷 type=%v 与事件名 %s 不一致", p["type"], names[i])
		}
	}
	if out.FinishReason != "stop" {
		t.Errorf("outcome = %#v", out)
	}
}

// TestResponsesStreamHasNoDoneSentinel 是刻意固化的一条差异:
// Responses 流没有 [DONE],终止靠 response.completed / failed 事件。
// 照抄 chat 的 [DONE] 会让严格实现的客户端多解析出一个非法事件。
func TestResponsesStreamHasNoDoneSentinel(t *testing.T) {
	body, _ := renderResponsesStream(t, baseRC(), frameHello, frameFinish)
	if strings.Contains(body, "[DONE]") {
		t.Fatalf("Responses 流不得有 [DONE]:\n%s", body)
	}
}

func TestResponsesStreamDeltaCarriesText(t *testing.T) {
	body, _ := renderResponsesStream(t, baseRC(), frameHello, frameWorld, frameFinish)
	names, payloads := parseNamedSSE(t, body)

	var text strings.Builder
	for i, n := range names {
		if n == "response.output_text.delta" {
			text.WriteString(payloads[i]["delta"].(string))
			if payloads[i]["item_id"] == nil {
				t.Error("delta 事件必须带 item_id")
			}
		}
	}
	if text.String() != "hello world" {
		t.Errorf("拼接文本 = %q", text.String())
	}
}

func TestResponsesStreamFailureEmitsResponseFailed(t *testing.T) {
	c, w := newStreamCtx()
	rc := baseRC()
	rc.Stream = true
	r := responsesRenderer{h: &Handler{}, req: &ResponsesRequest{Model: rc.Model}}
	out := r.Render(c, rc, sseThenErr(errUpstreamTest, frameHello))

	names, payloads := parseNamedSSE(t, w.Body.String())
	last := names[len(names)-1]
	if last != "response.failed" {
		t.Fatalf("最后一个事件 = %q, want response.failed", last)
	}
	resp := payloads[len(payloads)-1]["response"].(map[string]interface{})
	if resp["status"] != "failed" {
		t.Errorf("status = %v", resp["status"])
	}
	if resp["error"] == nil {
		t.Error("失败响应必须带 error 对象")
	}
	if out.Failure == nil {
		t.Error("outcome 应标记失败,供上层退款")
	}
}

func TestResponsesStreamTruncationIsIncomplete(t *testing.T) {
	rc := baseRC()
	rc.MaxTokens = 1
	body, _ := renderResponsesStream(t, rc, frameHello, frameWorld, frameFinish)

	names, payloads := parseNamedSSE(t, body)
	if names[len(names)-1] != "response.incomplete" {
		t.Fatalf("截断应以 response.incomplete 收尾,实际 %q", names[len(names)-1])
	}
	resp := payloads[len(payloads)-1]["response"].(map[string]interface{})
	det, _ := resp["incomplete_details"].(map[string]interface{})
	if det["reason"] != "max_output_tokens" {
		t.Errorf("incomplete_details = %v", resp["incomplete_details"])
	}
}

func TestResponsesCollectShape(t *testing.T) {
	c, w := newStreamCtx()
	r := responsesRenderer{h: &Handler{}, req: &ResponsesRequest{Model: "gpt-5"}}
	r.Render(c, baseRC(), sseFrom(frameHello, frameWorld, frameFinish))

	var obj ResponsesObject
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatalf("解析失败: %v (%s)", err, w.Body.String())
	}
	if obj.Status != "completed" {
		t.Errorf("status = %q", obj.Status)
	}
	if len(obj.Output) != 1 || obj.Output[0].Content[0].Text != "hello world" {
		t.Fatalf("output = %#v", obj.Output)
	}
	if obj.Usage == nil || obj.Usage.InputTokens != 11 {
		t.Fatalf("usage = %#v", obj.Usage)
	}
}

func TestResponsesCollectFailureUsesErrorEnvelope(t *testing.T) {
	c, w := newStreamCtx()
	r := responsesRenderer{h: &Handler{}, req: &ResponsesRequest{Model: "gpt-5"}}
	r.Render(c, baseRC(), sseFrom(frameNoText, frameFinish))

	if w.Code != 502 {
		t.Fatalf("status = %d", w.Code)
	}
	e := decodeErr(t, w.Body.Bytes())
	if e["code"] != "upstream_empty_output" {
		t.Errorf("code = %v", e["code"])
	}
}

// TestResponsesRejectsImageModel 覆盖审查发现的 P1:
// runChat 对图像模型无条件转派到只会输出 Chat Completions 报文的分支,
// Responses 客户端会收到 object=chat.completion,解析器直接报错 ——
// 而钱已经扣了、图已经生成了。必须在扣费前拦住。
func TestResponsesRejectsImageModelBeforeBilling(t *testing.T) {
	// 这条断言在 chat.go 的图像转派分支上:非 chatRenderer 时提前 400。
	// 这里直接验证类型判定成立,防止有人把 renderer 改成值/指针混用后判定失效。
	var rd renderer = responsesRenderer{}
	if _, isChat := rd.(chatRenderer); isChat {
		t.Fatal("responsesRenderer 不该被判成 chatRenderer")
	}
	rd = chatRenderer{}
	if _, isChat := rd.(chatRenderer); !isChat {
		t.Fatal("chatRenderer 必须被判成 chatRenderer")
	}
}

func TestResponsesTruncationAlwaysDisabled(t *testing.T) {
	// 上游没有截断策略通道。原样回显用户传的 "auto" 会让人以为它生效了。
	req := &ResponsesRequest{Model: "gpt-5", Truncation: "auto"}
	obj := newResponsesObject(req, respCtx{ID: "chatcmpl-x", Model: "gpt-5"}, responseCompleted)
	if obj.Truncation != "disabled" {
		t.Errorf("truncation = %q, want disabled", obj.Truncation)
	}
}

func TestResponsesIgnoredParams(t *testing.T) {
	got := responsesIgnored(&ResponsesRequest{Truncation: "auto", Include: []string{"reasoning.encrypted_content"}})
	if strings.Join(got, ",") != "truncation,include" {
		t.Errorf("Ignored = %v", got)
	}
	if len(responsesIgnored(&ResponsesRequest{Truncation: "disabled"})) != 0 {
		t.Error("默认值不该进忽略清单")
	}
}
