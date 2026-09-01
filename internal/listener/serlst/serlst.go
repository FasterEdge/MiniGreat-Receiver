// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package serlst 提供串口(UART/RS232/RS485)与射频模块监听器。
package serlst

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"

	"minigreat-receiver/internal/core"
)

// SerialListener 监听串口数据。
type SerialListener struct{}

// Name 返回监听器名。
func (SerialListener) Name() string { return "serial" }

// Description 返回描述。
func (SerialListener) Description() string { return "串口监听(UART/RS232/RS485): 实时读取串口数据" }

// Validate 校验参数。
func (SerialListener) Validate(cfg *core.Config) error {
	if cfg.SerialDevice == "" {
		return fmt.Errorf("serial: serialDevice 不能为空 (如 /dev/ttyUSB0)")
	}
	return nil
}

// Run 打开串口并持续读取。
func (SerialListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	return runSerial(ctx, cfg, "serial", sink)
}

// RFListener 监听射频模块(经串口透传的 LoRa/433MHz/Zigbee 等)上报数据。
type RFListener struct{}

// Name 返回监听器名。
func (RFListener) Name() string { return "rf" }

// Description 返回描述。
func (RFListener) Description() string {
	return "射频模块监听(LoRa/433MHz/Zigbee/BLE-SPP): 读取模块经串口上报的数据"
}

// Validate 校验参数。
func (RFListener) Validate(cfg *core.Config) error {
	if cfg.SerialDevice == "" {
		return fmt.Errorf("rf: serialDevice 不能为空 (如 /dev/ttyUSB0 接射频模块)")
	}
	return nil
}

// Run 打开串口并持续读取。
func (RFListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	return runSerial(ctx, cfg, "rf", sink)
}

func runSerial(ctx context.Context, cfg *core.Config, proto string, sink core.Sink) error {
	baud := cfg.SerialBaud
	if baud == 0 {
		baud = 115200
	}
	databits := cfg.SerialDataBits
	if databits == 0 {
		databits = 8
	}
	stopbits := cfg.SerialStopBits
	if stopbits == 0 {
		stopbits = 1
	}
	parity := serial.NoParity
	switch cfg.SerialParity {
	case "E":
		parity = serial.EvenParity
	case "O":
		parity = serial.OddParity
	}
	port, err := serial.Open(cfg.SerialDevice, &serial.Mode{
		BaudRate: baud, DataBits: databits, Parity: parity, StopBits: serial.StopBits(stopbits),
	})
	if err != nil {
		return fmt.Errorf("%s: 打开串口失败: %w", proto, err)
	}
	defer port.Close()
	_ = port.SetReadTimeout(200 * time.Millisecond)
	sink(core.Event{Protocol: proto, Time: time.Now().Format("15:04:05.000"), Source: cfg.SerialDevice,
		DataTxt: fmt.Sprintf("%s 监听已启动: %s @%d", strings.ToUpper(proto), cfg.SerialDevice, baud)})

	buf := make([]byte, 8192)
	var acc []byte
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, rerr := port.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			if len(acc) > 4096 {
				flush(proto, &acc, sink)
			}
			// 200ms 静默即视为一条完整消息
			_ = port.SetReadTimeout(200 * time.Millisecond)
			more, _ := port.Read(buf)
			if more > 0 {
				acc = append(acc, buf[:more]...)
			}
			flush(proto, &acc, sink)
		}
		if rerr != nil && !strings.Contains(rerr.Error(), "timeout") {
			return fmt.Errorf("%s: 读取失败: %w", proto, rerr)
		}
	}
}

func flush(proto string, acc *[]byte, sink core.Sink) {
	if len(*acc) == 0 {
		return
	}
	data := append([]byte(nil), (*acc)...)
	*acc = nil
	sink(core.Event{Protocol: proto, Time: time.Now().Format("15:04:05.000"),
		Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data)})
}