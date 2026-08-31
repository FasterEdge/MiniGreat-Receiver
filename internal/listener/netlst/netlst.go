// Package netlst 提供 TCP / UDP / HTTP / WebSocket 服务端监听器。
package netlst

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"minigreat-receiver/internal/core"
)

// TCPListener 监听 TCP 端口, 接收所有连接数据。
type TCPListener struct{}

// Name 返回监听器名。
func (TCPListener) Name() string { return "tcp" }

// Description 返回描述。
func (TCPListener) Description() string { return "TCP 服务端: 监听端口, 接收连接数据(可回显)" }

// Validate 校验参数。
func (TCPListener) Validate(cfg *core.Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("tcp: listenAddr 不能为空 (如 :9000)")
	}
	return nil
}

// Run 启动 TCP 监听。
func (TCPListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("tcp: 监听失败: %w", err)
	}
	defer ln.Close()
	sink(core.Event{Protocol: "tcp", Time: now(), Source: cfg.ListenAddr, DataTxt: "TCP 监听已启动: " + cfg.ListenAddr})

	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("tcp: accept 失败: %w", err)
			}
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer c.Close()
			remote := c.RemoteAddr().String()
			sink(core.Event{Protocol: "tcp", Time: now(), Source: remote, DataTxt: "新连接: " + remote})
			r := bufio.NewReader(c)
			buf := make([]byte, 8192)
			for {
				n, rerr := r.Read(buf)
				if n > 0 {
					data := append([]byte(nil), buf[:n]...)
					sink(core.Event{Protocol: "tcp", Time: now(), Source: remote,
						Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data)})
					if cfg.Echo {
						_, _ = c.Write(data)
					}
				}
				if rerr != nil {
					if rerr != io.EOF {
						sink(core.Event{Protocol: "tcp", Time: now(), Source: remote, DataTxt: "连接关闭: " + rerr.Error()})
					}
					return
				}
			}
		}(conn)
	}
}

// UDPListener 监听 UDP 端口, 接收数据报。
type UDPListener struct{}

// Name 返回监听器名。
func (UDPListener) Name() string { return "udp" }

// Description 返回描述。
func (UDPListener) Description() string { return "UDP 服务端: 监听端口, 接收数据报(可回显)" }

// Validate 校验参数。
func (UDPListener) Validate(cfg *core.Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("udp: listenAddr 不能为空 (如 :9001)")
	}
	return nil
}

// Run 启动 UDP 监听。
func (UDPListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	addr, err := net.ResolveUDPAddr("udp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("udp: 地址解析失败: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("udp: 监听失败: %w", err)
	}
	defer conn.Close()
	sink(core.Event{Protocol: "udp", Time: now(), Source: cfg.ListenAddr, DataTxt: "UDP 监听已启动: " + cfg.ListenAddr})

	buf := make([]byte, 65536)
	for {
		n, raddr, rerr := conn.ReadFromUDP(buf)
		if rerr != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("udp: 读取失败: %w", rerr)
			}
		}
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			sink(core.Event{Protocol: "udp", Time: now(), Source: raddr.String(),
				Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data)})
			if cfg.Echo {
				_, _ = conn.WriteToUDP(data, raddr)
			}
		}
	}
}

// HTTPListener 监听 HTTP 端口, 记录所有请求。
type HTTPListener struct{}

// Name 返回监听器名。
func (HTTPListener) Name() string { return "http" }

// Description 返回描述。
func (HTTPListener) Description() string { return "HTTP 服务端: 记录所有请求方法与参数, 可自定义响应" }

// Validate 校验参数。
func (HTTPListener) Validate(cfg *core.Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("http: listenAddr 不能为空 (如 :8080)")
	}
	return nil
}

// Run 启动 HTTP 监听。
func (HTTPListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		data := append([]byte(nil), body...)
		ev := core.Event{
			Protocol: "http", Time: now(), Source: r.RemoteAddr,
			Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data),
			Meta: map[string]any{
				"method":  r.Method,
				"path":    r.URL.Path,
				"query":   r.URL.RawQuery,
				"proto":   r.Proto,
				"headers": flattenH(r.Header),
			},
		}
		sink(ev)
		// 响应
		status := cfg.HTTPStatusCode
		if status == 0 {
			status = 200
		}
		for k, v := range cfg.HTTPHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		if cfg.HTTPBody != "" {
			_, _ = io.WriteString(w, cfg.HTTPBody)
		}
	})
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	sink(core.Event{Protocol: "http", Time: now(), Source: cfg.ListenAddr, DataTxt: "HTTP 监听已启动: " + cfg.ListenAddr})
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return fmt.Errorf("http: 监听失败: %w", err)
	}
}

func flattenH(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = strings.Join(v, ", ")
	}
	return out
}

// WSListener 监听 WebSocket 端口, 接收消息。
type WSListener struct{}

// Name 返回监听器名。
func (WSListener) Name() string { return "ws" }

// Description 返回描述。
func (WSListener) Description() string { return "WebSocket 服务端: 接收文本/二进制消息" }

// Validate 校验参数。
func (WSListener) Validate(cfg *core.Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("ws: listenAddr 不能为空 (如 :8081)")
	}
	return nil
}

// Run 启动 WebSocket 监听。
func (WSListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		remote := r.RemoteAddr
		sink(core.Event{Protocol: "ws", Time: now(), Source: remote, DataTxt: "WS 连接: " + remote})
		for {
			mt, data, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			ev := core.Event{Protocol: "ws", Time: now(), Source: remote,
				Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data)}
			if mt == websocket.TextMessage {
				ev.DataTxt = string(data)
				ev.Meta = map[string]any{"type": "text"}
			} else {
				ev.Meta = map[string]any{"type": "binary"}
			}
			sink(ev)
			if cfg.Echo {
				_ = conn.WriteMessage(mt, data) // #nosec G104
			}
		}
	})
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	sink(core.Event{Protocol: "ws", Time: now(), Source: cfg.ListenAddr, DataTxt: "WS 监听已启动: " + cfg.ListenAddr})
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return fmt.Errorf("ws: 监听失败: %w", err)
	}
}

// JSONEvent 用于 web 面板展示的事件 JSON 序列化。
func JSONEvent(e core.Event) string {
	b, _ := json.Marshal(e)
	return string(b)
}

func now() string {
	return time.Now().Format("15:04:05.000")
}