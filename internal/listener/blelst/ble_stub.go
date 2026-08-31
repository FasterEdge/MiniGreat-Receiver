//go:build !linux

// Package blelst 在非 Linux 平台提供占位实现。
package blelst

import (
	"context"
	"fmt"

	"minigreat-receiver/internal/core"
)

// BLEListener 非 Linux 平台占位。
type BLEListener struct{}

// Name 返回监听器名。
func (BLEListener) Name() string { return "ble" }

// Description 返回描述。
func (BLEListener) Description() string { return "BLE 扫描 (仅 Linux BlueZ 支持)" }

// Validate 校验参数。
func (BLEListener) Validate(cfg *core.Config) error {
	return fmt.Errorf("ble: 仅 Linux 平台支持 BlueZ 扫描")
}

// Run 返回不支持错误。
func (BLEListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	return fmt.Errorf("ble: 当前平台不支持, 请在 Linux 下运行")
}