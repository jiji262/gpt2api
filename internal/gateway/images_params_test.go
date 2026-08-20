package gateway

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// tlsStub 只用来让 c.Request.TLS 非 nil,内容无所谓。
var tlsStub = tls.ConnectionState{}

// ---------------------------------------------------------------- 绝对 URL

// TestPublicBaseURLFromForwardedHeaders 覆盖 F2:
// 图片 URL 此前是相对路径 /p/img/...,任何非同 origin 的客户端
// (openai-python、Cherry Studio、curl)拿到就是废的。
func TestPublicBaseURLFromForwardedHeaders(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		headers map[string]string
		tls     bool
		want    string
	}{
		{"裸 host", "api.example.com", nil, false, "http://api.example.com"},
		{"TLS", "api.example.com", nil, true, "https://api.example.com"},
		{"X-Forwarded-Proto", "api.example.com",
			map[string]string{"X-Forwarded-Proto": "https"}, false, "https://api.example.com"},
		{"X-Forwarded-Host 优先", "internal:8080",
			map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "gpt.example.com"},
			false, "https://gpt.example.com"},
		{"多级代理取第一个", "internal:8080",
			map[string]string{"X-Forwarded-Proto": "https, http", "X-Forwarded-Host": "a.example.com, b.internal"},
			false, "https://a.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			c.Request.Host = tc.host
			if tc.tls {
				c.Request.TLS = &tlsStub
			}
			for k, v := range tc.headers {
				c.Request.Header.Set(k, v)
			}
			if got := publicBaseURL(c, ""); got != tc.want {
				t.Errorf("publicBaseURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPublicBaseURLPrefersConfigured(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Host = "internal:8080"

	if got := publicBaseURL(c, "https://gpt.example.com/"); got != "https://gpt.example.com" {
		t.Errorf("配置的 base url 应优先且去掉尾斜杠,实际 %q", got)
	}
}

func TestAbsoluteURLJoinsPath(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Host = "api.example.com"

	got := absoluteURL(c, "", "/p/img/abc/0?exp=1&sig=x")
	if got != "http://api.example.com/p/img/abc/0?exp=1&sig=x" {
		t.Errorf("absoluteURL = %q", got)
	}
}

func TestAbsoluteURLLeavesAbsoluteAlone(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Host = "api.example.com"

	in := "https://files.oaiusercontent.com/x.png"
	if got := absoluteURL(c, "", in); got != in {
		t.Errorf("已是绝对 URL 不该再拼: %q", got)
	}
}

// ---------------------------------------------------------------- 图像参数

func bindImg(t *testing.T, body string) *ImageGenRequest {
	t.Helper()
	var req ImageGenRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("bind: %v", err)
	}
	return &req
}

func TestImageHardRejectedParams(t *testing.T) {
	cases := []struct {
		name      string
		extra     string
		wantParam string
	}{
		{"background 透明", `"background":"transparent"`, "background"},
		{"output_format webp", `"output_format":"webp"`, "output_format"},
		{"output_compression", `"output_compression":80`, "output_compression"},
		{"moderation low", `"moderation":"low"`, "moderation"},
		{"partial_images", `"partial_images":2`, "partial_images"},
		{"stream", `"stream":true`, "stream"},
		{"input_fidelity high", `"input_fidelity":"high"`, "input_fidelity"},
		{"style vivid", `"style":"vivid"`, "style"},
		{"quality high", `"quality":"high"`, "quality"},
		{"size 非方形", `"size":"1536x1024"`, "size"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := bindImg(t, `{"prompt":"cat",`+tc.extra+`}`)
			p, why := validateImageRequest(req)
			if p != tc.wantParam {
				t.Fatalf("param = %q, want %q (why=%s)", p, tc.wantParam, why)
			}
			if why == "" {
				t.Error("必须说明原因")
			}
		})
	}
}

func TestImageDefaultValuesPass(t *testing.T) {
	benign := []string{
		`"background":"auto"`,
		`"background":""`,
		`"output_format":"png"`,
		`"moderation":"auto"`,
		`"partial_images":0`,
		`"stream":false`,
		`"quality":"auto"`,
		`"style":"natural"`,
		`"size":"auto"`,
		`"size":"1024x1024"`,
		`"n":1`,
		`"n":4`,
		`"response_format":"url"`,
		`"response_format":"b64_json"`,
		`"user":"u-1"`,
	}
	for _, extra := range benign {
		t.Run(extra, func(t *testing.T) {
			req := bindImg(t, `{"prompt":"cat",`+extra+`}`)
			if p, why := validateImageRequest(req); p != "" {
				t.Errorf("%s 不该被拒: %s %s", extra, p, why)
			}
		})
	}
}

// TestImageNOutOfRangeRejected 覆盖"静默钳位"问题:
// n=10 此前被悄悄改成 4,用户以为要到 10 张,只拿到 4 张还按 4 张计费。
func TestImageNOutOfRangeRejected(t *testing.T) {
	req := bindImg(t, `{"prompt":"cat","n":10}`)
	p, why := validateImageRequest(req)
	if p != "n" {
		t.Fatalf("param = %q, want n", p)
	}
	if !strings.Contains(why, "4") {
		t.Errorf("应告知上限: %q", why)
	}
}

func TestImageResponseFormatValidated(t *testing.T) {
	req := bindImg(t, `{"prompt":"cat","response_format":"weird"}`)
	if p, _ := validateImageRequest(req); p != "response_format" {
		t.Fatalf("param = %q", p)
	}
}

// ---------------------------------------------------------------- edits: mask

// TestMaskRejected 覆盖 U3:mask 此前被当成普通参考图塞进同一数组,
// inpainting 语义完全丢失,还白占一个参考图配额。
func TestMaskRejected(t *testing.T) {
	if p, why := validateEditExtras(true, ""); p != "mask" {
		t.Fatalf("param = %q, want mask (why=%s)", p, why)
	}
}

func TestEditsInputFidelityRejected(t *testing.T) {
	if p, _ := validateEditExtras(false, "high"); p != "input_fidelity" {
		t.Fatalf("param = %q, want input_fidelity", p)
	}
}

func TestEditsWithoutExtrasPasses(t *testing.T) {
	if p, why := validateEditExtras(false, ""); p != "" {
		t.Fatalf("不该拒: %s %s", p, why)
	}
}

// ---------------------------------------------------------------- 响应形状

func TestImageResponseEchoesRequestFields(t *testing.T) {
	req := bindImg(t, `{"prompt":"cat","size":"1024x1024","quality":"auto","output_format":"png"}`)
	out := newImageGenResponse(req, 1700000000, "task-1")

	if out.Created != 1700000000 {
		t.Errorf("created = %d", out.Created)
	}
	if out.Size != "1024x1024" {
		t.Errorf("size 未回显: %q", out.Size)
	}
	if out.Quality != "auto" {
		t.Errorf("quality 未回显: %q", out.Quality)
	}
	if out.OutputFormat != "png" {
		t.Errorf("output_format 未回显: %q", out.OutputFormat)
	}
	if out.Background != "auto" {
		t.Errorf("background 应回显默认值 auto,实际 %q", out.Background)
	}
}

// TestImageResponseHasNoFabricatedUsage 是一条刻意的约束:
// 官方 gpt-image-1 响应带 usage,但本网关的上游不返回任何 token 计数。
// 编一个数字比缺字段更糟——成本核算侧车会拿它当真。
func TestImageResponseHasNoFabricatedUsage(t *testing.T) {
	req := bindImg(t, `{"prompt":"cat"}`)
	b, err := json.Marshal(newImageGenResponse(req, 1, "t"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"usage"`) {
		t.Errorf("不得输出编造的 usage: %s", b)
	}
}

// ---------------------------------------------------------------- 图像走 chat 的流式分叉

// TestImageAsChatStreamEmitsSSE 覆盖 F3:
// 此前无视 req.Stream 一律返回非流式 JSON,客户端按 SSE 解析直接失败 ——
// 钱扣了、图生成了、用户什么都没拿到。
func TestImageAsChatStreamEmitsSSE(t *testing.T) {
	c, w := newStreamCtx()
	rc := respCtx{ID: "chatcmpl-x", Model: "gpt-image-2", Created: 1700000000, PromptTokens: 5}

	writeImageAsChatStream(c, rc, "![generated](https://api.example.com/p/img/t/0)")

	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	lines := sseLines(t, w.Body.String())
	if lines[len(lines)-1] != "[DONE]" {
		t.Fatalf("缺 [DONE]: %v", lines)
	}
	var content, finish string
	for _, l := range lines[:len(lines)-1] {
		ck := decodeChunk(t, l)
		if ck.Created != 1700000000 {
			t.Errorf("created 漂移: %d", ck.Created)
		}
		if len(ck.Choices) == 0 {
			continue
		}
		content += ck.Choices[0].Delta.Content
		if ck.Choices[0].FinishReason != nil {
			finish = *ck.Choices[0].FinishReason
		}
	}
	if !strings.Contains(content, "![generated]") {
		t.Errorf("图片 markdown 没进流: %q", content)
	}
	if finish != "stop" {
		t.Errorf("finish_reason = %q", finish)
	}
}

func TestImageAsChatStreamIncludeUsage(t *testing.T) {
	c, w := newStreamCtx()
	rc := respCtx{ID: "chatcmpl-x", Model: "gpt-image-2", Created: 1, PromptTokens: 5, IncludeUsage: true}

	writeImageAsChatStream(c, rc, "abcd")

	lines := sseLines(t, w.Body.String())
	last := decodeChunk(t, lines[len(lines)-2])
	if last.Usage == nil || len(last.Choices) != 0 {
		t.Fatalf("末尾应是 usage chunk: %s", lines[len(lines)-2])
	}
	if last.Usage.PromptTokens != 5 {
		t.Errorf("prompt_tokens = %d", last.Usage.PromptTokens)
	}
}

// TestResponseFormatNormalizedToLower 覆盖审查发现的口径不一致:
// 校验用 lower() 放行,下游是 == "b64_json" 精确比较。
// 不归一的话 "B64_JSON" 通过校验后静默返回 url —— 正是三档策略要禁掉的行为。
func TestResponseFormatNormalizedToLower(t *testing.T) {
	for _, in := range []string{"B64_JSON", "b64_JSON", " b64_json "} {
		req := bindImg(t, `{"prompt":"cat","response_format":"`+strings.TrimSpace(in)+`"}`)
		req.ResponseFormat = in
		if p, why := validateImageRequest(req); p != "" {
			t.Fatalf("%q 不该被拒: %s %s", in, p, why)
		}
		applyImageDefaults(req)
		if req.ResponseFormat != "b64_json" {
			t.Errorf("%q 归一后 = %q, want b64_json", in, req.ResponseFormat)
		}
	}
}
