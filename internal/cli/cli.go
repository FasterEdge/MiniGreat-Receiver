// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
// Package cli 提供命令行监听工具。
// 用法示例:
//
//	minigreat-receiver listen --proto tcp --listen :9000 --echo
//	minigreat-receiver listen --proto http --listen :8080 --body '{"ok":1}'
//	minigreat-receiver listen --proto mqtt --broker tcp://127.0.0.1:1883 --topic a --topic b
//	minigreat-receiver listen --proto modbus --listen :502 --unit 1
//	minigreat-receiver listen --proto serial --device /dev/ttyUSB0 --baud 115200
//	minigreat-receiver listen --proto can --iface vcan0
//	minigreat-receiver listen --proto ble --scan 10
//	minigreat-receiver web --addr :8080
//	minigreat-receiver list
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/registry"
)

// Run 解析 os.Args 并执行。返回进程退出码。
func Run(args []string, stdout, stderr *os.File) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	cmd := args[1]
	switch cmd {
	case "list":
		return cmdList(stdout)
	case "listen", "l":
		return cmdListen(args[2:], stdout, stderr)
	case "web", "w":
		return cmdWeb(args[2:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "未知子命令: %s\n\n", cmd)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `MiniGreat-Receiver - 全方面多协议调试接收工具

用法:
  minigreat-receiver list                    列出全部支持的监听器
  minigreat-receiver listen <选项>           启动一个监听器
  minigreat-receiver web   <选项>            启动本地 Web 调试面板
  minigreat-receiver help                    显示本帮助

listen 子命令选项:
  --proto <名>      协议名: tcp|udp|http|ws|mqtt|modbus|serial|rf|can|spi|i2c|ble
  --listen host:port 监听地址 (网络类)
  --echo            收到数据后原样回显 (tcp/udp/ws)
  --body <str>      HTTP 响应体
  --status <code>   HTTP 响应码 (默认 200)

MQTT: --broker tcp://host:1883 --topic t(可多次) --user --pass --client
Modbus: --listen host:502 或 --device /dev/ttyUSB0 --unit 1 --baud 9600
串口 (serial/rf): --device /dev/ttyUSB0 --baud 115200 --databits 8 --parity N --stopbits 1
CAN (can): --iface can0 --filter 0x123
SPI (spi): --spi /dev/spidev0.0 --mode 0 --bits 8 --speed 1000000 --poll 500
I2C (i2c): --bus 1 --addr 0x48 --reg 0x00 --len 16 --poll 500
BLE (ble): --scan 10 (扫描秒数)

web 子命令选项:
  --addr host:port    监听地址 (默认 127.0.0.1:8080)
`)
}

func cmdList(w *os.File) int {
	reg := registry.New()
	fmt.Fprintln(w, "MiniGreat-Receiver 支持的监听器:")
	for _, n := range reg.Names() {
		l, _ := reg.Get(n)
		fmt.Fprintf(w, "  %-8s %s\n", n, l.Description())
	}
	return 0
}

type listenFlags struct {
	proto  string
	listen string
	echo   bool
	body   string
	status int

	broker string
	topics multiFlag
	user   string
	pass   string
	client string

	device   string
	baud     int
	databits int
	parity   string
	stopbits int
	unit     int

	iface  string
	filter string

	spiDev string
	mode   int
	bits   int
	speed  int64
	poll   int

	bus    int
	addr   int
	reg    int
	length int

	scan int
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func cmdListen(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f listenFlags
	fs.StringVar(&f.proto, "proto", "", "协议名")
	fs.StringVar(&f.listen, "listen", "", "监听地址")
	fs.BoolVar(&f.echo, "echo", false, "回显")
	fs.StringVar(&f.body, "body", "", "HTTP 响应体")
	fs.IntVar(&f.status, "status", 0, "HTTP 响应码")

	fs.StringVar(&f.broker, "broker", "", "MQTT broker")
	fs.Var(&f.topics, "topic", "MQTT 主题 (可多次)")
	fs.StringVar(&f.user, "user", "", "用户名")
	fs.StringVar(&f.pass, "pass", "", "密码")
	fs.StringVar(&f.client, "client", "", "client id")

	fs.StringVar(&f.device, "device", "", "串口设备")
	fs.IntVar(&f.baud, "baud", 115200, "波特率")
	fs.IntVar(&f.databits, "databits", 8, "数据位")
	fs.StringVar(&f.parity, "parity", "N", "校验")
	fs.IntVar(&f.stopbits, "stopbits", 1, "停止位")
	fs.IntVar(&f.unit, "unit", 1, "Modbus 从机ID")

	fs.StringVar(&f.iface, "iface", "", "CAN 接口")
	fs.StringVar(&f.filter, "filter", "", "CAN 过滤ID")

	fs.StringVar(&f.spiDev, "spi", "", "SPI 设备")
	fs.IntVar(&f.mode, "mode", 0, "SPI 模式")
	fs.IntVar(&f.bits, "bits", 8, "SPI 位宽")
	fs.Int64Var(&f.speed, "speed", 1000000, "SPI 频率")
	fs.IntVar(&f.poll, "poll", 500, "轮询间隔ms")

	fs.IntVar(&f.bus, "bus", -1, "I2C 总线")
	fs.IntVar(&f.addr, "addr", 0, "I2C 从机地址")
	fs.IntVar(&f.reg, "reg", -1, "I2C 寄存器")
	fs.IntVar(&f.length, "len", 16, "I2C 读取长度")

	fs.IntVar(&f.scan, "scan", 0, "BLE 扫描秒数")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := buildConfig(&f)
	if err != nil {
		fmt.Fprintln(stderr, "参数错误:", err)
		return 2
	}
	reg := registry.New()
	l, ok := reg.Get(cfg.Protocol)
	if !ok {
		fmt.Fprintf(stderr, "未知协议: %s (可用: %s)\n", cfg.Protocol, strings.Join(reg.Names(), ", "))
		return 2
	}
	if err := l.Validate(cfg); err != nil {
		fmt.Fprintln(stderr, "参数校验失败:", err)
		return 2
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sink := func(ev core.Event) {
		printEvent(stdout, ev)
	}
	if err := l.Run(ctx, cfg, sink); err != nil {
		fmt.Fprintln(stderr, "监听出错:", err)
		return 1
	}
	return 0
}

func buildConfig(f *listenFlags) (*core.Config, error) {
	cfg := &core.Config{
		Protocol:       f.proto,
		ListenAddr:     f.listen,
		Echo:           f.echo,
		HTTPBody:       f.body,
		HTTPStatusCode: f.status,
		Broker:         f.broker,
		Topics:         f.topics,
		Username:       f.user,
		Password:       f.pass,
		ClientID:       f.client,
		SerialDevice:   f.device,
		SerialBaud:     f.baud,
		SerialDataBits: f.databits,
		SerialParity:   f.parity,
		SerialStopBits: f.stopbits,
		ModbusUnitID:   byte(f.unit),
		ModbusBaud:     f.baud,
		ModbusParity:   f.parity,
		ModbusDataBits: f.databits,
		ModbusStopBits: f.stopbits,
		CANInterface:   f.iface,
		SPIDevice:      f.spiDev,
		SPIMode:        uint8(f.mode),
		SPIBits:        uint8(f.bits),
		SPISpeed:       f.speed,
		SPIPollMS:      f.poll,
		I2CBus:         f.bus,
		I2CAddr:        f.addr,
		I2CRegister:    f.reg,
		I2CLen:         f.length,
		I2CPollMS:      f.poll,
		BLEScanSeconds: f.scan,
	}
	if f.filter != "" {
		var id uint64
		if _, err := fmt.Sscanf(f.filter, "0x%X", &id); err != nil {
			if _, err2 := fmt.Sscanf(f.filter, "%d", &id); err2 != nil {
				return nil, fmt.Errorf("filter 解析失败")
			}
		}
		cfg.CANFilterID = uint32(id)
	}
	return cfg, nil
}

func printEvent(w *os.File, ev core.Event) {
	line := fmt.Sprintf("[%s] %s %s", ev.Time, ev.Protocol, ev.Source)
	if ev.DataTxt != "" {
		line += " | " + ev.DataTxt
	}
	if ev.DataHex != "" {
		line += " | HEX: " + ev.DataHex
	}
	if len(ev.Meta) > 0 {
		b, _ := json.Marshal(ev.Meta)
		line += " | " + string(b)
	}
	fmt.Fprintln(w, line)
	_ = time.Now
}
