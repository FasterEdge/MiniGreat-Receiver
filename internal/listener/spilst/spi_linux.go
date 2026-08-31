//go:build linux

// Package spilst 实现 SPI 监听器: 作为主设备周期性读取从设备 MISO 数据。
package spilst

import (
	"context"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"minigreat-receiver/internal/core"
)

// SPIListener SPI 轮询监听器。
type SPIListener struct{}

// Name 返回监听器名。
func (SPIListener) Name() string { return "spi" }

// Description 返回描述。
func (SPIListener) Description() string {
	return "SPI 监听: 周期发送 0x00 读取从设备 MISO 数据 (主设备模式)"
}

// Validate 校验参数。
func (SPIListener) Validate(cfg *core.Config) error {
	if cfg.SPIDevice == "" {
		return fmt.Errorf("spi: spiDevice 不能为空 (如 /dev/spidev0.0)")
	}
	return nil
}

// Run 轮询读取。
func (SPIListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	mode := cfg.SPIMode
	bits := cfg.SPIBits
	if bits == 0 {
		bits = 8
	}
	speed := cfg.SPISpeed
	if speed == 0 {
		speed = 1000000
	}
	pollMS := cfg.SPIPollMS
	if pollMS == 0 {
		pollMS = 500
	}

	f, err := os.OpenFile(cfg.SPIDevice, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("spi: 打开设备失败: %w", err)
	}
	defer f.Close()

	setIO := func(req, ptr uintptr) error {
		if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), req, ptr); errno != 0 {
			return fmt.Errorf("spi ioctl 失败: %v", errno)
		}
		return nil
	}
	if err := setIO(0x40016B01, uintptr(unsafe.Pointer(&mode))); err != nil { // SPI_IOC_WR_MODE
		return err
	}
	if err := setIO(0x40016B03, uintptr(unsafe.Pointer(&bits))); err != nil { // SPI_IOC_WR_BITS
		return err
	}
	if err := setIO(0x40046B04, uintptr(unsafe.Pointer(&speed))); err != nil { // SPI_IOC_WR_MAX_SPEED
		return err
	}

	sink(core.Event{Protocol: "spi", Time: time.Now().Format("15:04:05.000"), Source: cfg.SPIDevice,
		DataTxt: fmt.Sprintf("SPI 监听已启动: %s (mode=%d bits=%d speed=%d poll=%dms)",
			cfg.SPIDevice, mode, bits, speed, pollMS)})

	tx := make([]byte, 16)
	rx := make([]byte, 16)
	tr := struct {
		TxBuf       uint64
		RxBuf       uint64
		Len         uint32
		SpeedHz     uint32
		DelayUsecs  uint16
		BitsPerWord uint8
		CsChange    uint8
		TxNBits     uint8
		RxNBits     uint8
		Pad         uint16
	}{}
	tr.TxBuf = uint64(uintptr(unsafe.Pointer(&tx[0])))
	tr.RxBuf = uint64(uintptr(unsafe.Pointer(&rx[0])))
	tr.Len = 16
	tr.SpeedHz = uint32(speed)
	tr.BitsPerWord = bits

	ticker := time.NewTicker(time.Duration(pollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), uintptr(0x40006B00 /*SPI_IOC_MESSAGE(1)*/), uintptr(unsafe.Pointer(&tr))); errno != 0 {
			return fmt.Errorf("spi: 传输失败: %v", errno)
		}
		// 检查是否收到非零/非空数据
		if hasData(rx) {
			data := append([]byte(nil), rx...)
			sink(core.Event{Protocol: "spi", Time: time.Now().Format("15:04:05.000"), Source: cfg.SPIDevice,
				Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data),
				Meta: map[string]any{"device": cfg.SPIDevice, "len": len(data)}})
		}
	}
}

func hasData(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return true
		}
	}
	return false
}