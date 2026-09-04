// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/web"
)

// cmdWeb 启动本地 Web 调试面板。
func cmdWeb(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址")
	openBrowser := fs.Bool("open", false, "启动后尝试打开浏览器")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(stderr, "监听失败:", err)
		return 1
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv := web.New(logger)

	port := ln.Addr().(*net.TCPAddr).Port
	host := "127.0.0.1"
	if h, _, e := net.SplitHostPort(*addr); e == nil && h != "" && h != "0.0.0.0" && h != "::" {
		host = h
	}
	panelURL := fmt.Sprintf("http://%s:%d", host, port)
	fmt.Fprintf(stdout, "MiniGreat-Receiver Web 调试面板: %s\n", panelURL)
	if *openBrowser {
		openBrowserCmd(panelURL)
	}
	// ReadHeaderTimeout 防慢速客户端无限占用 handler goroutine
	// (慢连接 DoS: 连上不发完整请求头即可挂住一个 goroutine)。
	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := httpSrv.Serve(ln); err != nil {
		fmt.Fprintln(stderr, "Web 服务异常:", err)
		return 1
	}
	return 0
}

// openBrowserCmd 尝试用系统默认浏览器打开 URL。
func openBrowserCmd(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start() // #nosec G204
}
