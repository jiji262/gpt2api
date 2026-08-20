package chatgpt

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// trackedReader 记录 Close 是否被调用,用来验证连接没有泄漏。
type trackedReader struct {
	r      io.Reader
	closed atomic.Bool
}

func (t *trackedReader) Read(p []byte) (int, error) {
	if t.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	return t.r.Read(p)
}

func (t *trackedReader) Close() error {
	t.closed.Store(true)
	// 真实的 http.Response.Body.Close 会解阻塞正在进行的 Read,
	// 这里把语义补上,否则看门狗关不掉卡住的读。
	if c, ok := t.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func newTracked(s string) *trackedReader { return &trackedReader{r: strings.NewReader(s)} }

const twoEvents = "event: delta\ndata: {\"v\":\"a\"}\n\ndata: {\"v\":\"b\"}\n\n"

func TestParseSSEEmitsEvents(t *testing.T) {
	tr := newTracked(twoEvents)
	out := make(chan SSEEvent, 8)
	go parseSSE(context.Background(), tr, out, time.Second)

	var got []string
	for ev := range out {
		if ev.Err != nil {
			t.Fatalf("unexpected err: %v", ev.Err)
		}
		got = append(got, string(ev.Data))
	}
	if len(got) != 2 || got[0] != `{"v":"a"}` || got[1] != `{"v":"b"}` {
		t.Fatalf("events = %#v", got)
	}
	if !tr.closed.Load() {
		t.Error("流读完后必须 Close")
	}
}

// TestParseSSEUnblocksOnContextCancel 覆盖 U23:
// 消费方拿到结束标记就 break,不会把 channel 读干。此前 out<- 是无条件阻塞发送,
// 生产者永久卡住,defer r.Close() 永不执行,goroutine 与上游连接一起泄漏。
func TestParseSSEUnblocksOnContextCancel(t *testing.T) {
	// 无缓冲 channel:发出第一个事件后生产者就会阻塞在第二个上。
	body := strings.Repeat("data: {\"v\":\"x\"}\n\n", 50)
	tr := newTracked(body)
	out := make(chan SSEEvent)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { parseSSE(ctx, tr, out, time.Minute); close(done) }()

	<-out // 只读一个就走人,模拟消费方 break
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ctx 取消后生产者仍未退出:goroutine 泄漏")
	}
	if !tr.closed.Load() {
		t.Error("生产者退出时必须关闭底层连接")
	}
}

// TestParseSSEReadTimeoutClosesBody 覆盖 gateway.sse_read_timeout_sec 此前是死配置:
// readTimeout 参数被直接丢弃,两次事件之间卡住多久都不会超时。
func TestParseSSEReadTimeoutClosesBody(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	tr := &trackedReader{r: pr}
	out := make(chan SSEEvent, 4)

	go parseSSE(context.Background(), tr, out, 80*time.Millisecond)

	// 只写一个事件,然后什么都不做,让看门狗超时。
	go func() { _, _ = pw.Write([]byte("data: {\"v\":\"a\"}\n\n")) }()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				if !tr.closed.Load() {
					t.Error("读超时应关闭底层连接")
				}
				return
			}
		case <-deadline:
			t.Fatal("读超时未生效,parseSSE 一直挂着")
		}
	}
}

func TestParseSSEReportsReadError(t *testing.T) {
	pr, pw := io.Pipe()
	tr := &trackedReader{r: pr}
	out := make(chan SSEEvent, 4)
	go parseSSE(context.Background(), tr, out, time.Minute)

	go func() {
		_, _ = pw.Write([]byte("data: {\"v\":\"a\"}\n\n"))
		_ = pw.CloseWithError(io.ErrUnexpectedEOF)
	}()

	var sawErr bool
	for ev := range out {
		if ev.Err != nil {
			sawErr = true
		}
	}
	if !sawErr {
		t.Error("非 EOF 的读错误必须作为 SSEEvent.Err 上报,否则上层会把中断当正常结束")
	}
}
