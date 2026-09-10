// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
// Package netlst 单元测试: ctx 取消时监听器必须及时退出。
// 回归场景: TCP 空闲连接(客户端连上不发数据)曾永久占住 goroutine,
// ctx 取消后 Run 的 wg.Wait() 挂死 → 服务无法优雅停止; UDP 曾因
// ReadFromUDP 阻塞导致 ctx 取消后 Run 不返回。
package netlst

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
)

func nopSink(core.Event) {}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("预留端口失败: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// TestTCPListenerCtxCancelWithIdleConn 回归: 空闲连接 + ctx 取消时,
// Run 必须在超时内返回(修复前 wg.Wait 永久挂起)。
func TestTCPListenerCtxCancelWithIdleConn(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- (TCPListener{}).Run(ctx, &core.Config{ListenAddr: net.JoinHostPort("127.0.0.1", itoa(port))}, nopSink)
	}()

	// 等监听就绪
	var conn net.Conn
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(port)), 300*time.Millisecond)
		if err == nil {
			conn = c
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if conn == nil {
		t.Fatal("未能连上 TCP 监听")
	}
	defer conn.Close()
	// 客户端连上但不发数据, 让服务端进入阻塞 Read
	time.Sleep(150 * time.Millisecond)

	cancel()
	select {
	case <-done:
		// 通过: Run 已返回
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后 TCP Run 未在超时内返回(goroutine 挂死)")
	}
}

// TestUDPListenerCtxCancel 回归: ctx 取消时 UDP Run 必须返回
// (修复前 ReadFromUDP 阻塞, 无数据包到达时永不返回)。
func TestUDPListenerCtxCancel(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- (UDPListener{}).Run(ctx, &core.Config{ListenAddr: net.JoinHostPort("127.0.0.1", itoa(port))}, nopSink)
	}()
	// 等 UDP socket 就绪
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort("127.0.0.1", itoa(port)))
		if err != nil {
			t.Fatalf("udp 解析失败: %v", err)
		}
		c, err := net.DialUDP("udp", nil, addr)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
		// 通过
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后 UDP Run 未在超时内返回")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
