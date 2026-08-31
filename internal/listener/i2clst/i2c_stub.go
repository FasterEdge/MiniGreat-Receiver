//go:build !linux

// Package i2clst 在非 Linux 平台提供占位实现。
package i2clst

import (
	"context"
	"fmt"

	"minigreat-receiver/internal/core"
)

// I2CListener 非 Linux 平台占位。
type I2CListener struct{}

// Name 返回监听器名。
func (I2CListener) Name() string { return "i2c" }

// Description 返回描述。
func (I2CListener) Description() string { return "I2C 监听 (仅 Linux /dev/i2c-N 支持)" }

// Validate 校验参数。
func (I2CListener) Validate(cfg *core.Config) error {
	return fmt.Errorf("i2c: 仅 Linux 平台支持 /dev/i2c-N")
}

// Run 返回不支持错误。
func (I2CListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	return fmt.Errorf("i2c: 当前平台不支持, 请在 Linux 下运行")
}