// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
//go:build !linux

// Package spilst 在非 Linux 平台提供占位实现。
package spilst

import (
	"context"
	"fmt"

	"minigreat-receiver/internal/core"
)

// SPIListener 非 Linux 平台占位。
type SPIListener struct{}

// Name 返回监听器名。
func (SPIListener) Name() string { return "spi" }

// Description 返回描述。
func (SPIListener) Description() string { return "SPI 监听 (仅 Linux /dev/spidev 支持)" }

// Validate 校验参数。
func (SPIListener) Validate(cfg *core.Config) error {
	return fmt.Errorf("spi: 仅 Linux 平台支持 /dev/spidev")
}

// Run 返回不支持错误。
func (SPIListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	return fmt.Errorf("spi: 当前平台不支持, 请在 Linux 下运行")
}