package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jiji262/gpt2api/pkg/safefetch"
)

func TestDecodeDataURL(t *testing.T) {
	raw := []byte("\x89PNG\r\n\x1a\nfake")
	src := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)

	data, name, err := decodeImageSource(context.Background(), src)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(data) != string(raw) {
		t.Errorf("字节不一致")
	}
	if !strings.HasSuffix(name, ".png") {
		t.Errorf("文件名 = %q", name)
	}
}

func TestDecodeDataURLExtensionFromMime(t *testing.T) {
	cases := map[string]string{
		"image/jpeg": ".jpg",
		"image/webp": ".webp",
		"image/gif":  ".gif",
		"image/png":  ".png",
	}
	for mime, ext := range cases {
		src := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString([]byte("x"))
		_, name, err := decodeImageSource(context.Background(), src)
		if err != nil {
			t.Fatalf("%s: %v", mime, err)
		}
		if !strings.HasSuffix(name, ext) {
			t.Errorf("%s → %q, want 后缀 %q", mime, name, ext)
		}
	}
}

func TestDecodeImageSourceErrors(t *testing.T) {
	cases := map[string]string{
		"":                          "url 为空",
		"iVBORw0KGgo=":              "必须是",    // 裸 base64 是常见手写错误
		"data:image/png,notbase64":  "base64", // 缺 ;base64
		"data:image/png;base64":     "逗号",     // 缺逗号
		"data:image/png;base64,!!!": "解码",     // 非法 base64
		"ftp://x/y.png":             "必须是",
	}
	for src, wantSubstr := range cases {
		_, _, err := decodeImageSource(context.Background(), src)
		if err == nil {
			t.Errorf("%q 应报错", src)
			continue
		}
		if !strings.Contains(err.Error(), wantSubstr) {
			t.Errorf("%q 的错误 %q 不含 %q", src, err.Error(), wantSubstr)
		}
	}
}

// TestFetchRemoteImageBlocksLoopback 是 SSRF 防护的正向断言:
// 用户给的 image_url 指向本机/内网时必须被挡住,而且错误文案不能泄漏
// 连接细节 —— 否则网关就是一个内网端口探测 oracle。
//
// 用 httptest(监听 127.0.0.1)当靶子最直接:它被拦下就说明防护生效。
func TestFetchRemoteImageBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("请求不该真的打到内网服务上")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, _, err := decodeImageSource(context.Background(), srv.URL+"/a.jpg")
	if err == nil {
		t.Fatal("指向 127.0.0.1 的 URL 必须被拒绝")
	}
	if !errors.Is(err, safefetch.ErrBlocked) {
		t.Errorf("应是 ErrBlocked,实际 %v", err)
	}
	// 错误文案不得含状态码 / dial 细节。
	for _, leak := range []string{"connection refused", "dial", "127.0.0.1", "HTTP "} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("错误文案泄漏了 %q: %q", leak, err.Error())
		}
	}
}

func TestFetchRemoteImageRejectsNonHTTPScheme(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "gopher://a/b", "ftp://x/y.png"} {
		if _, _, err := decodeImageSource(context.Background(), u); err == nil {
			t.Errorf("%s 应被拒绝", u)
		}
	}
}

// TestVisionDisabledByDefault 是一条刻意的约束:
// text 通路 + attachments 的组合没有 HAR 抓包实证,不能默认开。
func TestVisionDisabledByDefault(t *testing.T) {
	if (&Handler{}).visionEnabled() {
		t.Error("未注入 Settings 时必须视为关闭")
	}
}

type visionSettings struct{ on bool }

func (v visionSettings) GatewayUpstreamTimeoutSec() int { return 60 }
func (v visionSettings) GatewaySSEReadTimeoutSec() int  { return 120 }
func (v visionSettings) GatewayVisionEnabled() bool     { return v.on }

func TestVisionEnabledReadsSettings(t *testing.T) {
	h := &Handler{Settings: visionSettings{on: true}}
	if !h.visionEnabled() {
		t.Error("设置为 true 时应开启")
	}
	h = &Handler{Settings: visionSettings{on: false}}
	if h.visionEnabled() {
		t.Error("设置为 false 时应关闭")
	}
}

// TestAttachImagesNoopWithoutImages 保证没有图片时不碰上游,
// 也就不会因为 cli 为 nil 而 panic。
func TestAttachImagesNoopWithoutImages(t *testing.T) {
	msgs := []RequestMessage{{Role: "user", Content: MessageContent{Text: "hi"}}}
	if err := (&Handler{}).attachImages(context.Background(), nil, msgs, nil); err != nil {
		t.Fatalf("无图片时应直接返回: %v", err)
	}
}

func TestAttachImagesRejectsTooMany(t *testing.T) {
	msgs := []RequestMessage{{Content: MessageContent{
		ImageSources: []string{"a", "b", "c", "d", "e"},
	}}}
	err := (&Handler{}).attachImages(context.Background(), nil, msgs, nil)
	if err == nil || !strings.Contains(err.Error(), "最多") {
		t.Fatalf("超过张数上限应报错,实际 %v", err)
	}
}
