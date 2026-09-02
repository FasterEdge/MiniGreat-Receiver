// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
//go:build linux

// Package blelst 实现 BLE 扫描监听器 (经 BlueZ D-Bus)。
package blelst

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
)

const (
	bluezName       = "org.bluez"
	adapterIf       = "org.bluez.Adapter1"
	deviceIf        = "org.bluez.Device1"
	objectManagerIf = "org.freedesktop.DBus.ObjectManager"
)

// BLEListener 扫描蓝牙设备并上报发现事件。
type BLEListener struct{}

// Name 返回监听器名。
func (BLEListener) Name() string { return "ble" }

// Description 返回描述。
func (BLEListener) Description() string {
	return "BLE 扫描 (BlueZ): 发现周围蓝牙设备(地址/名称/RSSI)"
}

// Validate 校验参数。
func (BLEListener) Validate(cfg *core.Config) error {
	if cfg.BLEScanSeconds <= 0 {
		cfg.BLEScanSeconds = 10
	}
	return nil
}

// Run 扫描并对每个首次发现的设备上报事件。
func (BLEListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	if cfg.BLEScanSeconds <= 0 {
		cfg.BLEScanSeconds = 10
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("ble: 无法连接系统总线: %w", err)
	}
	defer conn.Close()

	adapterPath := ""
	obj := conn.Object(bluezName, "/")
	var managed map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&managed); err != nil {
		return fmt.Errorf("ble: 读取 BlueZ 对象失败: %w", err)
	}
	for path, ifaces := range managed {
		if _, ok := ifaces[adapterIf]; ok {
			adapterPath = string(path)
			break
		}
	}
	if adapterPath == "" {
		return fmt.Errorf("ble: 未找到蓝牙适配器 (请检查 bluetoothctl/bluetoothd)")
	}

	adapter := conn.Object(bluezName, dbus.ObjectPath(adapterPath))
	_ = adapter.Call(adapterIf+".StartDiscovery", 0).Store()
	defer adapter.Call(adapterIf+".StopDiscovery", 0) // #nosec G104

	sink(core.Event{Protocol: "ble", Time: time.Now().Format("15:04:05.000"), Source: adapterPath,
		DataTxt: fmt.Sprintf("BLE 扫描已启动 (%ds), 适配器: %s", cfg.BLEScanSeconds, adapterPath)})

	deadline := time.Now().Add(time.Duration(cfg.BLEScanSeconds) * time.Second)
	seen := map[string]bool{}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		obj := conn.Object(bluezName, "/")
		var m map[dbus.ObjectPath]map[string]map[string]dbus.Variant
		if err := obj.Call(objectManagerIf+".GetManagedObjects", 0).Store(&m); err == nil {
			for path, ifaces := range m {
				d, ok := ifaces[deviceIf]
				if !ok {
					continue
				}
				addr := getStr(d, "Address")
				if addr == "" || seen[addr] {
					continue
				}
				name := getStr(d, "Name")
				if name == "" {
					name = "(无名称)"
				}
				seen[addr] = true
				meta := map[string]any{
					"address":   addr,
					"name":      name,
					"rssi":      getI16(d, "RSSI"),
					"connected": getBool(d, "Connected"),
					"path":      string(path),
				}
				sink(core.Event{Protocol: "ble", Time: time.Now().Format("15:04:05.000"), Source: addr,
					DataTxt: fmt.Sprintf("%s (%s) RSSI=%d", name, addr, getI16(d, "RSSI")),
					Meta:    meta})
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	sink(core.Event{Protocol: "ble", Time: time.Now().Format("15:04:05.000"),
		DataTxt: fmt.Sprintf("BLE 扫描结束, 共发现 %d 台设备", len(seen))})
	return nil
}

func getStr(m map[string]dbus.Variant, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

func getI16(m map[string]dbus.Variant, key string) int16 {
	if v, ok := m[key]; ok {
		switch t := v.Value().(type) {
		case int16:
			return t
		case int32:
			return int16(t)
		}
	}
	return 0
}

func getBool(m map[string]dbus.Variant, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}
