// FasterEdge 开源项目 · https://github.com/FasterEdge · https://gitee.com/FasterEdge
// MiniGreat-Receiver: 全方面多协议调试接收工具。
// 与 MiniGreat-Sender 配套, 提供 TCP/UDP/HTTP/WebSocket/MQTT/Modbus/串口/
// RF-AT/CAN/SPI/I2C/BLE 的接收/监听能力, 有线无线全覆盖,
// 提供 CLI 与本地 Web 调试面板两种使用方式。
package main

import (
	"os"

	"minigreat-receiver/internal/cli"
)

var version = "1.0.20260901" // 可通过 -ldflags "-X main.version=..." 覆盖

func main() {
	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr))
}