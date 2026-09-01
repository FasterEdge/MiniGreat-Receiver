// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
//go:build linux

// Package canlst 实现 SocketCAN 监听器 (Linux only)。
package canlst

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"minigreat-receiver/internal/core"
)

// CANListener 监听 CAN 总线报文。
type CANListener struct{}

// Name 返回监听器名。
func (CANListener) Name() string { return "can" }

// Description 返回描述。
func (CANListener) Description() string { return "CAN 总线监听 (SocketCAN): 实时读取 can0/vcan0 报文" }

// Validate 校验参数。
func (CANListener) Validate(cfg *core.Config) error {
	if cfg.CANInterface == "" {
		return fmt.Errorf("can: canInterface 不能为空 (如 can0/vcan0)")
	}
	return nil
}

// Run 监听 CAN 帧。
func (CANListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW, 1 /*CAN_RAW*/)
	if err != nil {
		return fmt.Errorf("can: 打开 SocketCAN 失败: %w", err)
	}
	defer unix.Close(fd)

	iface, err := net.InterfaceByName(cfg.CANInterface)
	if err != nil {
		return fmt.Errorf("can: 找不到接口 %s: %w", cfg.CANInterface, err)
	}
	addr := &unix.SockaddrCAN{Ifindex: iface.Index}
	if err := unix.Bind(fd, addr); err != nil {
		return fmt.Errorf("can: 绑定 %s 失败: %w", cfg.CANInterface, err)
	}
	_ = unix.SetNonblock(fd, false)
	_ = unix.SetsockoptTimeval(fd, 0, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1})

	sink(core.Event{Protocol: "can", Time: time.Now().Format("15:04:05.000"), Source: cfg.CANInterface,
		DataTxt: "CAN 监听已启动: " + cfg.CANInterface})

	buf := make([]byte, 32) // struct can_frame = 16 字节
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, err := unix.Read(fd, buf)
		if err != nil {
			// 超时重试
			continue
		}
		if n < 8 {
			continue
		}
		// 解析 can_frame
		id := binary.LittleEndian.Uint32(buf[0:4])
		dlc := buf[4]
		var data []byte
		if int(dlc) > 0 && int(dlc) <= 8 && n >= 8+int(dlc) {
			data = append([]byte(nil), buf[8:8+dlc]...)
		}
		ext := id&0x80000000 != 0
		rtr := id&0x40000000 != 0
		rawID := id & 0x1FFFFFFF
		if cfg.CANFilterID != 0 && rawID != cfg.CANFilterID {
			continue
		}
		meta := map[string]any{
			"interface": cfg.CANInterface,
			"id":        fmt.Sprintf("0x%X", rawID),
			"extended":  ext,
			"rtr":       rtr,
			"dlc":       dlc,
		}
		evt := core.Event{Protocol: "can", Time: time.Now().Format("15:04:05.000"),
			Source: fmt.Sprintf("%s 0x%X", cfg.CANInterface, rawID),
			Data:   data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data),
			Meta: meta}
		if len(data) == 0 {
			evt.DataTxt = "(空帧/RTR)"
		}
		sink(evt)
		_ = unsafe.Sizeof(canFrame{})
	}
}

type canFrame struct {
	ID   uint32
	DLC  uint8
	Pad  [3]uint8
	Data [8]uint8
}