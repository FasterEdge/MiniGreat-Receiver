// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
//go:build !linux

// Package canlst 在非 Linux 平台提供占位实现。
package canlst

import (
	"context"
	"fmt"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
)

// CANListener 非 Linux 平台占位。
type CANListener struct{}

// Name 返回监听器名。
func (CANListener) Name() string { return "can" }

// Description 返回描述。
func (CANListener) Description() string { return "CAN 总线监听 (仅 Linux SocketCAN 支持)" }

// Validate 校验参数。
func (CANListener) Validate(cfg *core.Config) error {
	return fmt.Errorf("can: 仅 Linux 平台支持 SocketCAN")
}

// Run 返回不支持错误。
func (CANListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	return fmt.Errorf("can: 当前平台不支持 SocketCAN, 请在 Linux 下运行")
}
