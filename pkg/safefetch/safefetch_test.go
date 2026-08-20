package safefetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestIsBlockedCoversSSRFTargets 是本包存在的理由:
// 用户给的 URL 不能把服务器变成内网探测器。169.254.169.254 是云元数据端点,
// 拿到它基本等于拿到实例凭据。
func TestIsBlockedCoversSSRFTargets(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.1.2.3", // 环回
		"10.0.0.1", "172.16.0.1", "172.31.255.255", "192.168.1.1", // 私有
		"169.254.169.254", // 云元数据
		"100.64.0.1",      // CGNAT
		"0.0.0.0",
		"224.0.0.1",       // 组播
		"255.255.255.255", // 广播
		"::1", "fe80::1", "fc00::1", "::ffff:127.0.0.1",
	}
	for _, s := range blocked {
		if ip := net.ParseIP(s); !IsBlocked(ip) {
			t.Errorf("%s 必须被拦截", s)
		}
	}

	allowed := []string{
		"1.1.1.1", "8.8.8.8", "104.18.0.1", "172.32.0.1", "9.255.255.255",
		"2606:4700:4700::1111",
	}
	for _, s := range allowed {
		if ip := net.ParseIP(s); IsBlocked(ip) {
			t.Errorf("%s 是公网地址,不该被拦截", s)
		}
	}
}

func TestIsBlockedRejectsNil(t *testing.T) {
	if !IsBlocked(nil) {
		t.Error("解析不出 IP 时必须按拦截处理")
	}
}

// TestGetBlocksLoopback 用 httptest(监听 127.0.0.1)当靶子:
// 它被拦下就说明 DialContext 层的判定生效。
func TestGetBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("请求不该真的打到内网服务上")
	}))
	defer srv.Close()

	_, _, err := Get(context.Background(), srv.URL, 1024, 5*time.Second)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
}

// TestGetBlocksRedirectToInternal 覆盖"前置校验被 302 绕过"的经典绕法:
// 校验 hostname 是挡不住的,必须在 dial 层判真实 IP。
func TestGetBlocksRedirectToInternal(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("重定向后的内网请求不该发出去")
	}))
	defer internal.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer redirector.Close()

	_, _, err := Get(context.Background(), redirector.URL, 1024, 5*time.Second)
	if err == nil {
		t.Fatal("必须失败")
	}
}

func TestGetRejectsNonHTTPScheme(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "gopher://a/b", "ftp://x/y", "//x/y"} {
		if _, _, err := Get(context.Background(), u, 1024, time.Second); !errors.Is(err, ErrBlocked) {
			t.Errorf("%s → %v, want ErrBlocked", u, err)
		}
	}
}

// TestErrorsAreSanitized 保证错误文案不泄漏内网拓扑。
func TestErrorsAreSanitized(t *testing.T) {
	// 一个不可能解析成功的域名:错误必须是笼统的,不能带 DNS 细节。
	_, _, err := Get(context.Background(), "https://this-host-does-not-exist.invalid/x", 1024, 3*time.Second)
	if err == nil {
		t.Fatal("应报错")
	}
	for _, leak := range []string{"no such host", "lookup", "dial tcp", "i/o timeout"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("错误文案泄漏了 %q: %q", leak, err.Error())
		}
	}
}

func TestClientHasTimeoutAndRedirectCap(t *testing.T) {
	c := Client(7 * time.Second)
	if c.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v", c.Timeout)
	}
	if c.CheckRedirect == nil {
		t.Fatal("必须限制重定向")
	}
	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	via := make([]*http.Request, 3)
	if err := c.CheckRedirect(req, via); err == nil {
		t.Error("超过跳数上限应报错")
	}
	if err := c.CheckRedirect(req, nil); err != nil {
		t.Errorf("首跳不该报错: %v", err)
	}
}
