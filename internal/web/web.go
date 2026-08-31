// Package web 提供 MiniGreat-Receiver 本地 Web 调试面板:
//  - 静态页面 (内嵌)
//  - POST /api/start   启动监听 (JSON body = core.Config)
//  - POST /api/stop    停止当前监听
//  - GET  /api/status  当前状态
//  - GET  /api/protocols 列出监听器
//  - GET  /api/history 历史事件
//  - WS   /api/ws      实时事件推送
package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"minigreat-receiver/internal/core"
	"minigreat-receiver/internal/registry"
)

//go:embed static/*
var staticFS embed.FS

// Server Web 面板服务器。
type Server struct {
	reg      *core.Registry
	logger   *slog.Logger
	upgrader websocket.Upgrader

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	runID   uint64
	curCfg  *core.Config
	events  []core.Event
	clients map[*websocket.Conn]bool
}

// New 创建 Web 服务器。
func New(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		reg:     registry.New(),
		logger:  logger,
		events:  make([]core.Event, 0, 500),
		clients: make(map[*websocket.Conn]bool),
	}
}

// Handler 返回 HTTP 路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/protocols", s.handleProtocols)
	mux.HandleFunc("/api/start", s.handleStart)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/ws", s.handleWS)
	return mux
}

func (s *Server) handleProtocols(w http.ResponseWriter, r *http.Request) {
	type info struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := []info{}
	for _, n := range s.reg.Names() {
		l, _ := s.reg.Get(n)
		out = append(out, info{n, l.Description()})
	}
	writeJSON(w, out)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var cfg core.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "JSON 解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	l, ok := s.reg.Get(cfg.Protocol)
	if !ok {
		http.Error(w, "未知协议: "+cfg.Protocol, http.StatusBadRequest)
		return
	}
	if err := l.Validate(&cfg); err != nil {
		http.Error(w, "参数校验失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	// 停止旧的
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = nil
	s.running = false
	myRun := s.runID + 1
	s.runID = myRun
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.running = true
	s.curCfg = &cfg
	s.mu.Unlock()

	go func() {
		err := l.Run(ctx, &cfg, s.onEvent)
		s.mu.Lock()
		if s.runID == myRun {
			s.running = false
			s.cancel = nil
		}
		s.mu.Unlock()
		if err != nil {
			s.push(core.Event{Protocol: cfg.Protocol, Time: time.Now().Format("15:04:05.000"),
				DataTxt: "监听出错: " + err.Error()})
			s.logger.Error("listener error", "err", err)
		}
	}()
	writeJSON(w, map[string]any{"ok": true, "protocol": cfg.Protocol, "listenAddr": cfg.ListenAddr})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, map[string]any{"running": s.running, "config": s.curCfg})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, s.events)
}

func (s *Server) onEvent(ev core.Event) {
	s.push(ev)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// push 记录事件并广播到所有 WS 客户端。
func (s *Server) push(ev core.Event) {
	s.mu.Lock()
	s.events = append(s.events, ev)
	if len(s.events) > 1000 {
		s.events = s.events[len(s.events)-1000:]
	}
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	for _, c := range clients {
		_ = c.WriteMessage(websocket.TextMessage, b) // #nosec G104
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v) // #nosec G104
}