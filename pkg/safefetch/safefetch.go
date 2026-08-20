// Package safefetch 提供"按调用方给的 URL 取内容"时的出站防护。
//
// 网关有两处会拉取用户提供的 URL:chat 的 vision 图片输入、
// images 的 reference_images。裸用 http.DefaultClient 的话:
//
//  1. 用户可以让服务器去请求内网地址(127.0.0.1 / 10.x / 169.254.169.254 云元数据)
//  2. 跟随重定向意味着一个正常的 https 外链可以 302 到内网,前置校验白做
//  3. 错误文案把上游状态码和 dial 错误原样回传,就是一个内网端口探测 oracle
//
// 本包在 DialContext 层按解析出的**真实 IP** 拦截,重定向的每一跳都会重新经过
// 它,因此第 2 点自然被覆盖。错误信息统一脱敏。
package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ErrBlocked 表示目标解析到了不允许的地址。
// 调用方应原样回传这个错误,不要附带底层 dial 细节。
var ErrBlocked = errors.New("目标地址不被允许(仅支持公网地址)")

// v4Blocked / v6Blocked 是禁止出站的网段,按地址族分开。
//
// 必须分开:net.ParseIP("1.1.1.1") 返回的是 16 字节的 IPv4-mapped 形式,
// 如果把 ::ffff:0:0/96 放进统一列表,它会匹配**所有** IPv4 地址,
// 把整个公网一起拦掉。
var (
	// 环回、私有、链路本地(含 169.254.169.254 云元数据)、CGNAT、
	// 文档/基准测试保留段、组播、保留段、广播。
	v4Blocked = mustCIDRs(
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
		"224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
	)
	// 未指定、环回、ULA、链路本地、组播。
	v6Blocked = mustCIDRs("::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8")
)

func mustCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// IsBlocked 判断一个 IP 是否落在禁止网段内。
//
// IPv4-mapped 的 IPv6 地址(::ffff:127.0.0.1)会被规范化成 IPv4 后再判,
// 否则它是一条绕过 IPv4 黑名单的现成通道。
func IsBlocked(ip net.IP) bool {
	if ip == nil {
		return true
	}
	nets := v6Blocked
	if v4 := ip.To4(); v4 != nil {
		ip, nets = v4, v4Blocked
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Client 返回一个带出站防护的 http.Client。
//
// 关键点是在 DialContext 里判 IP 而不是在请求前判 hostname:
// DNS 可以在校验之后、连接之前变更(DNS rebinding),而 dial 拿到的是真实 IP。
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, ErrBlocked
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil || len(ips) == 0 {
					return nil, ErrBlocked
				}
				for _, ip := range ips {
					if IsBlocked(ip.IP) {
						return nil, ErrBlocked
					}
				}
				// 用已解析且通过校验的第一个 IP 直接拨号,避免 Dial 内部再查一次 DNS
				// 拿到不同的结果(rebinding)。
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   2,
		},
		// 重定向每一跳都会重新走 DialContext,这里只限制跳数与协议。
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("重定向次数过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrBlocked
			}
			return nil
		},
	}
}

// Get 拉取一个用户提供的 URL,最多读 maxBytes 字节。
//
// 错误信息刻意不含目标状态码之外的细节,也不含 dial 错误原文 ——
// 那会把网关变成内网探测 oracle。
func Get(ctx context.Context, rawURL string, maxBytes int64, timeout time.Duration) ([]byte, string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, "", ErrBlocked
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("URL 不合法")
	}
	res, err := Client(timeout).Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return nil, "", ErrBlocked
		}
		// 统一脱敏:不回传 dial / TLS 的具体失败原因。
		return nil, "", errors.New("下载失败")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("下载失败(HTTP %d)", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, "", errors.New("读取失败")
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("内容超过 %dMB 上限", maxBytes/1024/1024)
	}
	if len(data) == 0 {
		return nil, "", errors.New("内容为空")
	}
	return data, res.Header.Get("Content-Type"), nil
}
