package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------- content 解析

func TestMessageContentAcceptsStringAndArray(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantText  string
		wantParts []string
	}{
		{"纯字符串", `"hello"`, "hello", nil},
		{"null", `null`, "", nil},
		{"空字符串", `""`, "", nil},
		{"单个 text part", `[{"type":"text","text":"hello"}]`, "hello", nil},
		{"多个 text part 用换行拼接", `[{"type":"text","text":"a"},{"type":"text","text":"b"}]`, "a\nb", nil},
		{"含 image_url", `[{"type":"text","text":"看图"},{"type":"image_url","image_url":{"url":"data:..."}}]`, "看图", []string{"image_url"}},
		{"纯 image_url", `[{"type":"image_url","image_url":{"url":"x"}}]`, "", []string{"image_url"}},
		{"含 input_audio", `[{"type":"input_audio","input_audio":{}}]`, "", []string{"input_audio"}},
		{"含 file", `[{"type":"file","file":{}}]`, "", []string{"file"}},
		{"空数组", `[]`, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mc MessageContent
			if err := json.Unmarshal([]byte(tc.raw), &mc); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.raw, err)
			}
			if mc.Text != tc.wantText {
				t.Errorf("Text = %q, want %q", mc.Text, tc.wantText)
			}
			if strings.Join(mc.NonTextParts, ",") != strings.Join(tc.wantParts, ",") {
				t.Errorf("NonTextParts = %v, want %v", mc.NonTextParts, tc.wantParts)
			}
		})
	}
}

func TestMessageContentRejectsGarbage(t *testing.T) {
	var mc MessageContent
	if err := json.Unmarshal([]byte(`123`), &mc); err == nil {
		t.Error("数字应当解析失败")
	}
}

// TestMultimodalRequestBindsInsteadOfCrashing 是 F4 的核心:
// 修复前 content 数组会让整个请求体反序列化失败,并把 Go 内部类型名吐给客户端。
func TestMultimodalRequestBinds(t *testing.T) {
	body := `{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	var req ChatCompletionsRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("多模态请求体应能解析: %v", err)
	}
	if len(req.Messages) != 1 || req.Messages[0].Content.Text != "hi" {
		t.Fatalf("messages = %#v", req.Messages)
	}
}

// ---------------------------------------------------------------- 参数策略

// bind 是测试用的最小绑定:走真实的 json.Unmarshal 路径,不经过 gin。
func bind(t *testing.T, body string) *ChatCompletionsRequest {
	t.Helper()
	var req ChatCompletionsRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("bind %s: %v", body, err)
	}
	return &req
}

const baseReq = `"model":"gpt-5","messages":[{"role":"user","content":"hi"}]`

func TestHardRejectedParams(t *testing.T) {
	cases := []struct {
		name      string
		extra     string
		wantParam string
	}{
		{"tools 非空", `"tools":[{"type":"function","function":{"name":"f"}}]`, "tools"},
		{"tool_choice required", `"tool_choice":"required"`, "tool_choice"},
		{"tool_choice 指定函数", `"tool_choice":{"type":"function","function":{"name":"f"}}`, "tool_choice"},
		{"functions 旧式", `"functions":[{"name":"f"}]`, "functions"},
		{"response_format json_object", `"response_format":{"type":"json_object"}`, "response_format"},
		{"response_format json_schema", `"response_format":{"type":"json_schema","json_schema":{"name":"s"}}`, "response_format"},
		{"n 大于 1", `"n":3`, "n"},
		{"stop 非空", `"stop":["\n\n"]`, "stop"},
		{"stop 字符串", `"stop":"END"`, "stop"},
		{"logprobs true", `"logprobs":true`, "logprobs"},
		{"top_logprobs", `"top_logprobs":5`, "top_logprobs"},
		{"logit_bias 非空", `"logit_bias":{"123":-100}`, "logit_bias"},
		{"presence_penalty 非零", `"presence_penalty":1.5`, "presence_penalty"},
		{"frequency_penalty 非零", `"frequency_penalty":-0.5`, "frequency_penalty"},
		{"seed", `"seed":42`, "seed"},
		{"audio", `"audio":{"voice":"alloy","format":"mp3"}`, "audio"},
		{"prediction", `"prediction":{"type":"content","content":"x"}`, "prediction"},
		{"web_search_options", `"web_search_options":{}`, "web_search_options"},
		{"modalities 含 audio", `"modalities":["text","audio"]`, "modalities"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := bind(t, "{"+baseReq+","+tc.extra+"}")
			v := validateChatRequest(req)
			if v.RejectParam == "" {
				t.Fatalf("期望硬拒 %q,实际放行", tc.wantParam)
			}
			if v.RejectParam != tc.wantParam {
				t.Errorf("RejectParam = %q, want %q", v.RejectParam, tc.wantParam)
			}
			if v.RejectMessage == "" {
				t.Error("拒绝必须带可读原因")
			}
		})
	}
}

// TestDefaultValuesAreNotRejected 是本层最重要的约束:
// 真实客户端会无条件带上默认值(openai-node 总发 "tools":[]、
// LangChain 总发 "stop":null),按 presence 判断会把正常请求全部拒掉。
func TestDefaultValuesAreNotRejected(t *testing.T) {
	benign := []string{
		`"tools":[]`,
		`"tools":null`,
		`"tool_choice":null`,
		`"tool_choice":"none"`,
		`"tool_choice":"auto"`,
		`"functions":[]`,
		`"response_format":{"type":"text"}`,
		`"response_format":null`,
		`"n":1`,
		`"n":null`,
		`"stop":[]`,
		`"stop":null`,
		`"stop":""`,
		`"logprobs":false`,
		`"logprobs":null`,
		`"top_logprobs":null`,
		`"logit_bias":{}`,
		`"logit_bias":null`,
		`"presence_penalty":0`,
		`"frequency_penalty":0`,
		`"seed":null`,
		`"audio":null`,
		`"prediction":null`,
		`"modalities":["text"]`,
		`"modalities":null`,
		`"parallel_tool_calls":false`,
		`"parallel_tool_calls":true`,
		`"store":false`,
		`"metadata":{}`,
		`"temperature":0.7`,
		`"top_p":1`,
		`"user":"u-1"`,
		`"service_tier":"auto"`,
		`"stream_options":{"include_usage":true}`,
	}
	for _, extra := range benign {
		t.Run(extra, func(t *testing.T) {
			req := bind(t, "{"+baseReq+","+extra+"}")
			if v := validateChatRequest(req); v.RejectParam != "" {
				t.Errorf("%s 不应被拒,实际拒了 %q: %s", extra, v.RejectParam, v.RejectMessage)
			}
		})
	}
}

func TestSoftIgnoredParamsAreReported(t *testing.T) {
	req := bind(t, "{"+baseReq+`,"temperature":0.3,"top_p":0.9,"store":true,"service_tier":"flex"}`)
	v := validateChatRequest(req)
	if v.RejectParam != "" {
		t.Fatalf("软忽略参数不应被拒: %q", v.RejectParam)
	}
	got := strings.Join(v.Ignored, ",")
	for _, want := range []string{"temperature", "top_p", "store", "service_tier"} {
		if !strings.Contains(got, want) {
			t.Errorf("Ignored 缺 %q,实际 %v", want, v.Ignored)
		}
	}
}

func TestSoftIgnoredOnlyWhenMeaningful(t *testing.T) {
	// temperature=1 是官方默认值,带不带都一样,不该噪声化地报忽略。
	req := bind(t, "{"+baseReq+`,"temperature":1,"top_p":1,"store":false}`)
	v := validateChatRequest(req)
	if len(v.Ignored) != 0 {
		t.Errorf("默认值不该进 Ignored: %v", v.Ignored)
	}
}

func TestMaxCompletionTokensAlias(t *testing.T) {
	req := bind(t, "{"+baseReq+`,"max_completion_tokens":256}`)
	v := validateChatRequest(req)
	if v.RejectParam != "" {
		t.Fatalf("不该拒: %q", v.RejectParam)
	}
	if req.MaxTokens != 256 {
		t.Errorf("max_completion_tokens 应别名到 MaxTokens,实际 %d", req.MaxTokens)
	}
}

func TestMaxTokensWinsOverAliasWhenBothPresent(t *testing.T) {
	req := bind(t, "{"+baseReq+`,"max_tokens":100,"max_completion_tokens":256}`)
	validateChatRequest(req)
	if req.MaxTokens != 100 {
		t.Errorf("两者都传时以 max_tokens 为准,实际 %d", req.MaxTokens)
	}
}

// TestMultimodalPartsRejectedWithParam 覆盖 F4 的止血分支:
// 上游拿不到图片输入,必须明确报错,不能悄悄只发文字。
func TestMultimodalPartsRejected(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"这张图"},{"type":"image_url","image_url":{"url":"data:image/png;base64,xx"}}]}]}`)
	v := validateChatRequest(req)
	if v.RejectParam != "messages[0].content" {
		t.Fatalf("RejectParam = %q, want messages[0].content", v.RejectParam)
	}
	if !strings.Contains(v.RejectMessage, "image_url") {
		t.Errorf("消息应指明是哪种 part: %q", v.RejectMessage)
	}
}

func TestPureTextPartsPass(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	if v := validateChatRequest(req); v.RejectParam != "" {
		t.Fatalf("纯文本 part 不该被拒: %q %s", v.RejectParam, v.RejectMessage)
	}
}

func TestUnsupportedRoleRejected(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"tool","content":"result","tool_call_id":"c1"}]}`)
	v := validateChatRequest(req)
	if v.RejectParam == "" {
		t.Fatal("tool role 依赖 function calling,上游做不到,应明确拒绝")
	}
}

func TestDeveloperRoleMapsToSystem(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"developer","content":"be brief"}]}`)
	if v := validateChatRequest(req); v.RejectParam != "" {
		t.Fatalf("developer role 是 system 的新名字,应放行: %q", v.RejectParam)
	}
	ups := req.upstreamMessages()
	if ups[0].Role != "system" {
		t.Errorf("developer 应映射为 system,实际 %q", ups[0].Role)
	}
}

func TestUpstreamMessagesFlattens(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}`)
	ups := req.upstreamMessages()
	if len(ups) != 1 || ups[0].Content != "a\nb" {
		t.Fatalf("ups = %#v", ups)
	}
}

// ---------------------------------------------------------------- HTTP 层

func TestWriteUnsupportedParamShape(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	writeUnsupportedParam(c, "tools", "上游没有工具通道")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	e := decodeErr(t, w.Body.Bytes())
	if e["param"] != "tools" {
		t.Errorf("param = %v", e["param"])
	}
	if e["code"] != "unsupported_parameter" {
		t.Errorf("code = %v", e["code"])
	}
	if !strings.Contains(e["message"].(string), "chatgpt.com") {
		t.Errorf("消息应说明为什么做不到: %q", e["message"])
	}
}

func TestIgnoredParamsHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	setIgnoredParamsHeader(c, []string{"temperature", "top_p"})

	if got := w.Header().Get("X-Gateway-Ignored-Params"); got != "temperature,top_p" {
		t.Errorf("header = %q", got)
	}
}

func TestIgnoredParamsHeaderSkippedWhenEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	setIgnoredParamsHeader(c, nil)

	if got := w.Header().Get("X-Gateway-Ignored-Params"); got != "" {
		t.Errorf("空列表不该写头,实际 %q", got)
	}
}

// ---------------------------------------------------------------- 上游 slug 解析

// TestResolveUpstreamSlugIsSharedByBothPaths 覆盖 U32:
// chat 会过灰度 slug 映射,image 此前直接用 m.UpstreamModelSlug ——
// 同一个后台配置项在两条路径上行为不同,排查时会完全找错方向。
func TestResolveUpstreamSlug(t *testing.T) {
	cases := map[string]string{
		"gpt-5":   "gpt-5-3", // 抓包实证的兜底映射
		"gpt-5-3": "gpt-5-3", // 已经是灰度 slug,no-op
		"auto":    "auto",
		"gpt-4o":  "gpt-4o", // 未收录的原样透传
		"":        "",
	}
	for in, want := range cases {
		if got := resolveUpstreamSlug(in); got != want {
			t.Errorf("resolveUpstreamSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUpstreamSlugFallbacksAreEvidenceBased(t *testing.T) {
	// 兜底表刻意保持极小:每一条都要有 HAR 抓包实证。
	// 它变大就意味着有人在凭猜测加映射,那会制造难以排查的静默拒绝。
	if len(upstreamSlugFallbacks) > 3 {
		t.Errorf("兜底映射表膨胀到 %d 条,请确认每条都有抓包证据", len(upstreamSlugFallbacks))
	}
}

// ---------------------------------------------------------------- vision 开关

// TestVisionOffRejectsImageParts 确认默认行为不变:
// 关闭 vision 时 image_url 仍然明确 400,不会静默只发文字。
func TestVisionOffRejectsImageParts(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[
		{"type":"text","text":"看图"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aa"}}]}]}`)
	if v := validateChatRequestWithVision(req, false); v.RejectParam != "messages[0].content" {
		t.Fatalf("RejectParam = %q", v.RejectParam)
	}
}

func TestVisionOnAllowsImageParts(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[
		{"type":"text","text":"看图"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aa"}}]}]}`)
	if v := validateChatRequestWithVision(req, true); v.RejectParam != "" {
		t.Fatalf("开启 vision 后不该拒: %q %s", v.RejectParam, v.RejectMessage)
	}
}

// TestVisionOnStillRejectsAudioAndFile 确认开关只放行图片:
// input_audio / file 无论如何都没有上游通道。
func TestVisionOnStillRejectsAudioAndFile(t *testing.T) {
	for _, part := range []string{
		`{"type":"input_audio","input_audio":{}}`,
		`{"type":"file","file":{}}`,
	} {
		req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[`+part+`]}]}`)
		if v := validateChatRequestWithVision(req, true); v.RejectParam == "" {
			t.Errorf("%s 应始终被拒", part)
		}
	}
}

func TestImageSourcesCaptured(t *testing.T) {
	req := bind(t, `{"model":"gpt-5","messages":[{"role":"user","content":[
		{"type":"image_url","image_url":{"url":"https://a/1.png"}},
		{"type":"image_url","image_url":{"url":"data:image/png;base64,bb"}}]}]}`)
	got := req.Messages[0].Content.ImageSources
	if len(got) != 2 || got[0] != "https://a/1.png" || got[1] != "data:image/png;base64,bb" {
		t.Fatalf("ImageSources = %v", got)
	}
}

func TestBlockedPartsFiltersOnlyImageURL(t *testing.T) {
	in := []string{"image_url", "input_audio", "file"}
	if got := blockedParts(in, true); strings.Join(got, ",") != "input_audio,file" {
		t.Errorf("vision 开启时 = %v", got)
	}
	if got := blockedParts(in, false); len(got) != 3 {
		t.Errorf("vision 关闭时 = %v", got)
	}
}
