// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
//go:build linux

// Package i2clst 实现 I2C 监听器: 周期轮询从机寄存器读取数据。
package i2clst

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"minigreat-receiver/internal/core"
)

// I2CListener I2C 轮询监听器。
type I2CListener struct{}

// Name 返回监听器名。
func (I2CListener) Name() string { return "i2c" }

// Description 返回描述。
func (I2CListener) Description() string {
	return "I2C 监听: 周期性读取从机寄存器数据 (主设备模式轮询)"
}

// Validate 校验参数。
func (I2CListener) Validate(cfg *core.Config) error {
	if cfg.I2CBus < 0 {
		return fmt.Errorf("i2c: i2cBus 必须 >= 0 (如 1 对应 /dev/i2c-1)")
	}
	if cfg.I2CAddr <= 0 || cfg.I2CAddr > 0x7F {
		return fmt.Errorf("i2c: i2cAddr 必须在 1~127 之间(7位地址)")
	}
	return nil
}

// Run 轮询读取。
func (I2CListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	dev := fmt.Sprintf("/dev/i2c-%d", cfg.I2CBus)
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("i2c: 打开 %s 失败: %w", dev, err)
	}
	defer f.Close()
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(0x0703 /*I2C_SLAVE*/), uintptr(cfg.I2CAddr)); errno != 0 {
		return fmt.Errorf("i2c: 设置从机地址 0x%02X 失败: %v", cfg.I2CAddr, errno)
	}
	readLen := cfg.I2CLen
	if readLen <= 0 {
		readLen = 16
	}
	pollMS := cfg.I2CPollMS
	if pollMS == 0 {
		pollMS = 500
	}
	sink(core.Event{Protocol: "i2c", Time: time.Now().Format("15:04:05.000"), Source: dev,
		DataTxt: fmt.Sprintf("I2C 监听已启动: %s addr=0x%02X reg=%d len=%d poll=%dms",
			dev, cfg.I2CAddr, cfg.I2CRegister, readLen, pollMS)})

	ticker := time.NewTicker(time.Duration(pollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		buf := make([]byte, readLen)
		if cfg.I2CRegister >= 0 {
			// 写寄存器地址, 再读
			if _, werr := f.Write([]byte{byte(cfg.I2CRegister)}); werr != nil {
				return fmt.Errorf("i2c: 写寄存器失败: %w", werr)
			}
		}
		n, rerr := f.Read(buf)
		if rerr != nil && n == 0 {
			sink(core.Event{Protocol: "i2c", Time: time.Now().Format("15:04:05.000"), Source: dev,
				DataTxt: "读取失败: " + rerr.Error()})
			continue
		}
		data := append([]byte(nil), buf[:n]...)
		sink(core.Event{Protocol: "i2c", Time: time.Now().Format("15:04:05.000"), Source: dev,
			Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data),
			Meta: map[string]any{"bus": cfg.I2CBus, "addr": fmt.Sprintf("0x%02X", cfg.I2CAddr), "len": n}})
	}
}