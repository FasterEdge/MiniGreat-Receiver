// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
// Package modbuslst 提供自实现的 Modbus 从站(Server), 支持 TCP 与 RTU。
// 内存中维护线圈区与寄存器区, 响应常见功能码:
//
//	01 读线圈 / 02 读离散输入 / 03 读保持寄存器 / 04 读输入寄存器
//	05 写单线圈 / 06 写单寄存器 / 0F 写多线圈 / 10 写多寄存器
package modbuslst

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
)

// ModbusListener Modbus 从站监听器。
type ModbusListener struct{}

// Name 返回监听器名。
func (ModbusListener) Name() string { return "modbus" }

// Description 返回描述。
func (ModbusListener) Description() string {
	return "Modbus 从站(TCP/RTU): 应答主站读写线圈/寄存器请求, 结果实时上报"
}

// Validate 校验参数。
func (ModbusListener) Validate(cfg *core.Config) error {
	if cfg.ListenAddr == "" && cfg.SerialDevice == "" {
		return fmt.Errorf("modbus: 需要 listenAddr(TCP) 或 serialDevice(RTU)")
	}
	return nil
}

// Run 启动 Modbus 从站, 解析并响应请求, 把读写事件推给 sink。
func (ModbusListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	unitID := cfg.ModbusUnitID
	if unitID == 0 {
		unitID = 1
	}
	dev := &modbusDevice{coils: make([]byte, 65536), dis: make([]byte, 65536), hr: make([]uint16, 65536), ir: make([]uint16, 65536)}

	if cfg.SerialDevice != "" {
		return runRTU(ctx, cfg, unitID, dev, sink)
	}
	return runTCP(ctx, cfg, unitID, dev, sink)
}

func runTCP(ctx context.Context, cfg *core.Config, unitID byte, dev *modbusDevice, sink core.Sink) error {
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("modbus(tcp): 监听失败: %w", err)
	}
	defer ln.Close()
	sink(core.Event{Protocol: "modbus", Time: now(), Source: cfg.ListenAddr,
		DataTxt: fmt.Sprintf("Modbus TCP 从站已启动: %s (unit=%d)", cfg.ListenAddr, unitID)})
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("modbus(tcp): accept 失败: %w", err)
			}
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer c.Close()
			remote := c.RemoteAddr().String()
			sink(core.Event{Protocol: "modbus", Time: now(), Source: remote, DataTxt: "主站连接: " + remote})
			// 处理直到连接关闭
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err := handleTCPFrame(c, unitID, dev, remote, sink); err != nil {
					return
				}
			}
		}(conn)
	}
}

// handleTCPFrame 读取并处理一个 MBAP 帧。
func handleTCPFrame(c net.Conn, unitID byte, dev *modbusDevice, remote string, sink core.Sink) error {
	_ = c.SetReadDeadline(time.Now().Add(30 * time.Second))
	hdr := make([]byte, 7)
	if _, err := io.ReadFull(c, hdr); err != nil {
		return err
	}
	transaction := binary.BigEndian.Uint16(hdr[0:2])
	length := int(binary.BigEndian.Uint16(hdr[4:6]))
	if length < 2 || length > 254 {
		return fmt.Errorf("modbus: 非法 MBAP 长度 %d", length)
	}
	body := make([]byte, length-1) // 除 unit 外的 PDU
	if _, err := io.ReadFull(c, body); err != nil {
		return err
	}
	unit := hdr[6]
	pdu := body
	respPDU, evt := dev.process(unitID, unit, pdu, remote)
	if respPDU != nil {
		resp := make([]byte, 0, 9+len(respPDU))
		resp = binary.BigEndian.AppendUint16(resp, transaction)
		resp = binary.BigEndian.AppendUint16(resp, 0) // protocol
		resp = binary.BigEndian.AppendUint16(resp, uint16(1+len(respPDU)))
		resp = append(resp, unit)
		resp = append(resp, respPDU...)
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, _ = c.Write(resp)
	}
	if evt != nil {
		sink(*evt)
	}
	return nil
}

func runRTU(ctx context.Context, cfg *core.Config, unitID byte, dev *modbusDevice, sink core.Sink) error {
	baud := cfg.SerialBaud
	if baud == 0 {
		baud = 9600
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
	port, err := serial.Open(cfg.SerialDevice, &serial.Mode{BaudRate: baud, DataBits: databits, Parity: parity, StopBits: serial.StopBits(stopbits)})
	if err != nil {
		return fmt.Errorf("modbus(rtu): 打开串口失败: %w", err)
	}
	defer port.Close()
	_ = port.SetReadTimeout(1 * time.Second)
	sink(core.Event{Protocol: "modbus", Time: now(), Source: cfg.SerialDevice,
		DataTxt: fmt.Sprintf("Modbus RTU 从站已启动: %s @%d (unit=%d)", cfg.SerialDevice, baud, unitID)})

	// RTU 帧解析状态机: 3.5字符静默作为帧间隔, 简化用 4ms/字节
	buf := make([]byte, 0, 256)
	readBuf := make([]byte, 256)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, rerr := port.Read(readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
			// 尝试解析完整 RTU 帧: addr(1)+func(1)+data+CRC(2), 至少 4 字节
			if len(buf) >= 4 {
				frameLen, valid := detectRTUFrame(buf, unitID)
				if valid && frameLen <= len(buf) {
					frame := buf[:frameLen]
					buf = buf[frameLen:]
					// 校验帧 CRC: 损坏的请求帧必须静默丢弃
					// (detectRTUFrame 无法从请求头推断精确长度时按保守帧长截断,
					// 这里以 CRC 为准做最终判定)。
					if len(frame) < 4 || rtuCRC(frame[:len(frame)-2]) != uint16(frame[len(frame)-2])|uint16(frame[len(frame)-1])<<8 {
						continue
					}
					resp, evt := dev.processRTU(unitID, frame)
					if resp != nil {
						_, _ = port.Write(resp)
					}
					if evt != nil {
						sink(*evt)
					}
				} else if len(buf) > 256 {
					buf = buf[1:] // 丢弃无法对齐的首字节
				}
			}
		}
		if rerr != nil {
			if !strings.Contains(rerr.Error(), "timeout") {
				return fmt.Errorf("modbus(rtu): 读取失败: %w", rerr)
			}
		}
	}
}

// detectRTUFrame 尝试按 RTU 从 buf 头部提取一帧并校验 CRC。
// 返回 (帧长, 是否有效)。
func detectRTUFrame(buf []byte, unitID byte) (int, bool) {
	if len(buf) < 4 {
		return 0, false
	}
	addr := buf[0]
	if addr != unitID && addr != 0 { // 0=广播
		return 1, false
	}
	fc := buf[1]
	switch {
	case fc >= 1 && fc <= 4:
		// req: addr+func+2(start)+2(qty) = 8; resp: addr+func+1(cnt)+N
		if len(buf) < 4 {
			return 0, false
		}
		// 无法从请求头判断长度, 保守等待到 8 字节以上
		if len(buf) >= 8 {
			return 8, true
		}
		return 0, false
	case fc == 5 || fc == 6:
		if len(buf) >= 8 {
			return 8, true
		}
		return 0, false
	case fc == 15 || fc == 16:
		if len(buf) < 7 {
			return 0, false
		}
		bc := int(buf[6])
		total := 7 + bc + 2
		if len(buf) >= total {
			return total, true
		}
		return 0, false
	default:
		return 1, false
	}
}

// processRTU 处理一个 RTU 帧并返回响应帧。
func (d *modbusDevice) processRTU(unitID byte, frame []byte) ([]byte, *core.Event) {
	if len(frame) < 4 {
		return nil, nil
	}
	unit := frame[0]
	pdu := frame[1 : len(frame)-2] // 去掉 CRC
	respPDU, evt := d.process(unitID, unit, pdu, "RTU")
	if respPDU == nil {
		return nil, nil
	}
	resp := make([]byte, 0, len(pdu)+4)
	resp = append(resp, unit)
	resp = append(resp, respPDU...)
	crc := rtuCRC(resp)
	resp = append(resp, byte(crc), byte(crc>>8))
	return resp, evt
}

// modbusDevice 内存数据模型与协议处理。
type modbusDevice struct {
	mu    sync.Mutex
	coils []byte   // 线圈
	dis   []byte   // 离散输入(只读)
	hr    []uint16 // 保持寄存器
	ir    []uint16 // 输入寄存器(只读)
}

// process 处理 PDU, 返回响应 PDU 与可选事件。
func (d *modbusDevice) process(unitID, unit byte, pdu []byte, remote string) ([]byte, *core.Event) {
	if unit != unitID && unit != 0 {
		return nil, nil
	}
	if len(pdu) < 1 {
		return exceptionResp(1, 1), nil
	}
	fc := pdu[0]
	// 广播 (unit==0) 仅允许写功能码且从不响应 (Modbus 规范:
	// 广播帧不产生响应, 否则同总线上多从站同时应答会冲突)。
	broadcast := unit == 0
	mkEvt := func(desc string, data []byte, meta map[string]any) *core.Event {
		evt := &core.Event{Protocol: "modbus", Time: now(), Source: remote,
			DataTxt: desc, DataHex: core.FormatDataHex(data), Data: data, Meta: meta}
		return evt
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	// 广播读请求: 规范不允许, 静默丢弃 (不执行也不响应)。
	if broadcast && (fc == 0x01 || fc == 0x02 || fc == 0x03 || fc == 0x04) {
		return nil, nil
	}

	switch fc {
	case 0x01, 0x02:
		if len(pdu) < 5 {
			return exceptionResp(fc, 3), nil
		}
		addr := binary.BigEndian.Uint16(pdu[1:3])
		qty := binary.BigEndian.Uint16(pdu[3:5])
		var region []byte
		name := "线圈"
		if fc == 0x02 {
			region = d.dis
			name = "离散输入"
		} else {
			region = d.coils
		}
		data, err := readBits(region, addr, qty)
		if err != nil {
			return exceptionResp(fc, 2), nil
		}
		resp := append([]byte{fc, byte(len(data))}, data...)
		return resp, mkEvt(fmt.Sprintf("读%s addr=%d qty=%d", name, addr, qty), data,
			map[string]any{"func": fc, "addr": addr, "qty": qty, "unit": unit})
	case 0x03, 0x04:
		if len(pdu) < 5 {
			return exceptionResp(fc, 3), nil
		}
		addr := binary.BigEndian.Uint16(pdu[1:3])
		qty := binary.BigEndian.Uint16(pdu[3:5])
		var region []uint16
		name := "保持寄存器"
		if fc == 0x04 {
			region = d.ir
			name = "输入寄存器"
		} else {
			region = d.hr
		}
		if int(addr)+int(qty) > len(region) {
			return exceptionResp(fc, 2), nil
		}
		data := make([]byte, qty*2)
		for i := 0; i < int(qty); i++ {
			binary.BigEndian.PutUint16(data[i*2:], region[int(addr)+i])
		}
		resp := append([]byte{fc, byte(len(data))}, data...)
		return resp, mkEvt(fmt.Sprintf("读%s addr=%d qty=%d", name, addr, qty), data,
			map[string]any{"func": fc, "addr": addr, "qty": qty, "unit": unit})
	case 0x05:
		if len(pdu) < 5 {
			return exceptionResp(fc, 3), nil
		}
		addr := binary.BigEndian.Uint16(pdu[1:3])
		val := binary.BigEndian.Uint16(pdu[3:5])
		if val == 0xFF00 {
			d.coils[addr] = 1
		} else if val == 0x0000 {
			d.coils[addr] = 0
		} else {
			return exceptionResp(fc, 3), nil
		}
		resp := pdu
		if broadcast {
			return nil, mkEvt(fmt.Sprintf("广播写单线圈 addr=%d val=%d", addr, val), []byte{byte(val)},
				map[string]any{"func": fc, "addr": addr, "unit": unit})
		}
		return resp, mkEvt(fmt.Sprintf("写单线圈 addr=%d val=%d", addr, val), []byte{byte(val)},
			map[string]any{"func": fc, "addr": addr, "unit": unit})
	case 0x06:
		if len(pdu) < 5 {
			return exceptionResp(fc, 3), nil
		}
		addr := binary.BigEndian.Uint16(pdu[1:3])
		val := binary.BigEndian.Uint16(pdu[3:5])
		d.hr[addr] = val
		resp := pdu
		if broadcast {
			return nil, mkEvt(fmt.Sprintf("广播写单寄存器 addr=%d val=%d", addr, val), pdu[3:5],
				map[string]any{"func": fc, "addr": addr, "val": val, "unit": unit})
		}
		return resp, mkEvt(fmt.Sprintf("写单寄存器 addr=%d val=%d", addr, val), pdu[3:5],
			map[string]any{"func": fc, "addr": addr, "val": val, "unit": unit})
	case 0x0F:
		if len(pdu) < 6 { // fc+start2+qty2+bc1; 数据长度由 6+bc 检查兜底
			return exceptionResp(fc, 3), nil
		}
		addr := binary.BigEndian.Uint16(pdu[1:3])
		qty := binary.BigEndian.Uint16(pdu[3:5])
		bc := int(pdu[5])
		if len(pdu) < 6+bc {
			return exceptionResp(fc, 3), nil
		}
		if int(addr)+int(qty) > len(d.coils) {
			return exceptionResp(fc, 2), nil
		}
		for i := 0; i < int(qty); i++ {
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			if byteIdx >= bc {
				break
			}
			if pdu[6+byteIdx]&(1<<bitIdx) != 0 {
				d.coils[int(addr)+i] = 1
			} else {
				d.coils[int(addr)+i] = 0
			}
		}
		resp := []byte{fc, byte(addr >> 8), byte(addr), byte(qty >> 8), byte(qty)}
		if broadcast {
			return nil, mkEvt(fmt.Sprintf("广播写多线圈 addr=%d qty=%d", addr, qty), pdu[6:6+bc],
				map[string]any{"func": fc, "addr": addr, "qty": qty, "unit": unit})
		}
		return resp, mkEvt(fmt.Sprintf("写多线圈 addr=%d qty=%d", addr, qty), pdu[6:6+bc],
			map[string]any{"func": fc, "addr": addr, "qty": qty, "unit": unit})
	case 0x10:
		if len(pdu) < 8 {
			return exceptionResp(fc, 3), nil
		}
		addr := binary.BigEndian.Uint16(pdu[1:3])
		qty := binary.BigEndian.Uint16(pdu[3:5])
		bc := int(pdu[5])
		if len(pdu) < 6+bc || bc != int(qty)*2 {
			return exceptionResp(fc, 3), nil
		}
		if int(addr)+int(qty) > len(d.hr) {
			return exceptionResp(fc, 2), nil
		}
		for i := 0; i < int(qty); i++ {
			d.hr[int(addr)+i] = binary.BigEndian.Uint16(pdu[6+i*2:])
		}
		resp := []byte{fc, byte(addr >> 8), byte(addr), byte(qty >> 8), byte(qty)}
		if broadcast {
			return nil, mkEvt(fmt.Sprintf("广播写多寄存器 addr=%d qty=%d", addr, qty), pdu[6:6+bc],
				map[string]any{"func": fc, "addr": addr, "qty": qty, "unit": unit})
		}
		return resp, mkEvt(fmt.Sprintf("写多寄存器 addr=%d qty=%d", addr, qty), pdu[6:6+bc],
			map[string]any{"func": fc, "addr": addr, "qty": qty, "unit": unit})
	default:
		return exceptionResp(fc, 1), nil
	}
}

func readBits(region []byte, addr, qty uint16) ([]byte, error) {
	if int(addr)+int(qty) > len(region) {
		return nil, fmt.Errorf("out of range")
	}
	nbytes := (int(qty) + 7) / 8
	out := make([]byte, nbytes)
	for i := 0; i < int(qty); i++ {
		bit := region[int(addr)+i]
		if bit != 0 {
			out[i/8] |= 1 << uint(i%8)
		}
	}
	return out, nil
}

func exceptionResp(fc, code byte) []byte {
	return []byte{fc | 0x80, code}
}

// rtuCRC 计算 Modbus RTU CRC16。
func rtuCRC(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func now() string {
	return time.Now().Format("15:04:05.000")
}
