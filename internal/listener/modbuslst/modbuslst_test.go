// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
// Package modbuslst 单元测试: 协议帧处理、越界防护与 CRC 校验。
package modbuslst

import (
	"testing"
)

func newTestDevice() *modbusDevice {
	return &modbusDevice{
		coils: make([]byte, 65536),
		dis:   make([]byte, 65536),
		hr:    make([]uint16, 65536),
		ir:    make([]uint16, 65536),
	}
}

// TestModbusWriteSingleAndRead 验证 06 写单寄存器 / 03 读保持寄存器 往返一致。
func TestModbusWriteSingleAndRead(t *testing.T) {
	dev := newTestDevice()
	// 06 写单寄存器 addr=10 val=0x1234
	resp, evt := dev.process(1, 1, []byte{0x06, 0x00, 0x0A, 0x12, 0x34}, "test")
	if len(resp) != 5 || resp[0] != 0x06 {
		t.Fatalf("06 write resp = %x", resp)
	}
	if evt == nil {
		t.Fatal("06 write should emit event")
	}
	if dev.hr[10] != 0x1234 {
		t.Fatalf("hr[10] = %#x", dev.hr[10])
	}
	// 03 读保持寄存器 addr=10 qty=1
	resp2, evt2 := dev.process(1, 1, []byte{0x03, 0x00, 0x0A, 0x00, 0x01}, "test")
	if len(resp2) != 4 || resp2[0] != 0x03 || resp2[2] != 0x12 || resp2[3] != 0x34 {
		t.Fatalf("03 read resp = %x", resp2)
	}
	if evt2 == nil {
		t.Fatal("03 read should emit event")
	}
}

// TestModbusWriteMultipleBoundary 回归: 0x0F/0x10 多写越过内存区边界时
// 必须返回异常码 2 而不是 panic (旧实现数组越界崩溃 → 远程 DoS)。
func TestModbusWriteMultipleBoundary(t *testing.T) {
	dev := newTestDevice()
	// 0x10 写多寄存器 addr=65535 qty=2 (越界)
	pdu := []byte{0x10, 0xFF, 0xFF, 0x00, 0x02, 0x04, 0x00, 0x01, 0x00, 0x02}
	resp, _ := dev.process(1, 1, pdu, "test")
	if len(resp) != 2 || resp[0] != 0x90 || resp[1] != 0x02 {
		t.Fatalf("0x10 overflow should return exception 2, got %x", resp)
	}
	// 0x0F 写多线圈 addr=65535 qty=16 (越界)
	pdu = []byte{0x0F, 0xFF, 0xFF, 0x00, 0x10, 0x02, 0xFF, 0xFF}
	resp, _ = dev.process(1, 1, pdu, "test")
	if len(resp) != 2 || resp[0] != 0x8F || resp[1] != 0x02 {
		t.Fatalf("0x0F overflow should return exception 2, got %x", resp)
	}
	// 边界内最大合法: addr=65535 qty=1 应成功
	resp, _ = dev.process(1, 1, []byte{0x10, 0xFF, 0xFF, 0x00, 0x01, 0x02, 0x00, 0xAB}, "test")
	if len(resp) != 5 || resp[0] != 0x10 {
		t.Fatalf("0x10 boundary write should succeed, got %x", resp)
	}
	// 0x0F qty=1 (bc=1) 最小合法帧应成功, 且边界内
	resp, _ = dev.process(1, 1, []byte{0x0F, 0xFF, 0xFF, 0x00, 0x01, 0x01, 0x01}, "test")
	if len(resp) != 5 || resp[0] != 0x0F {
		t.Fatalf("0x0F qty=1 write should succeed, got %x", resp)
	}
}

// TestRTUCRCKnownVector 验证 rtuCRC 实现与标准向量一致:
// 经典示例帧 01 03 00 00 00 0A 的 CRC16 = 0xCDC5
// (uint16 低字节 0xC5 在线上先行, 标准帧为 ... 0A C5 CD)。
func TestRTUCRCKnownVector(t *testing.T) {
	frame := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	crc := rtuCRC(frame)
	want := uint16(0xCDC5)
	if crc != want {
		t.Fatalf("rtuCRC(%x) = %#04x, want %#04x", frame, crc, want)
	}
	// 追加 CRC(低字节先行)后整帧再算应得到 0: CRC(数据+CRC) == 0
	full := append(append([]byte(nil), frame...), byte(crc), byte(crc>>8))
	if rtuCRC(full) != 0 {
		t.Fatalf("rtuCRC(frame+crc) = %#04x, want 0", rtuCRC(full))
	}
}

// TestProcessRTUFrameStructure 验证 processRTU 解析帧结构并响应
// (CRC 校验在 runRTU 调用方完成, processRTU 只负责 PDU 处理)。
func TestProcessRTUFrameStructure(t *testing.T) {
	dev := newTestDevice()
	// 用错误 CRC 构造完整 RTU 帧
	req := []byte{0x01, 0x06, 0x00, 0x01, 0x12, 0x34}
	crc := rtuCRC(req)
	frame := append(append([]byte(nil), req...), byte(crc^0xFF), byte(crc>>8^0xFF)) // 故意损坏 CRC
	resp, evt := dev.processRTU(1, frame)
	if resp == nil || evt == nil {
		t.Fatalf("processRTU should still process (CRC checked by caller), resp=%x evt=%v", resp, evt)
	}
}

// TestDetectRTUFrame 验证帧边界检测: 广播地址(0)与错误地址均能定位。
func TestDetectRTUFrame(t *testing.T) {
	// 广播写单寄存器 8 字节
	if n, ok := detectRTUFrame([]byte{0x00, 0x06, 0x00, 0x01, 0x00, 0x02, 0x00, 0x00}, 1); !ok || n != 8 {
		t.Fatalf("broadcast frame: n=%d ok=%v", n, ok)
	}
	// 错误从机地址: 应返回 1 表示丢弃该字节
	if n, ok := detectRTUFrame([]byte{0x02, 0x06, 0x00, 0x01, 0x00, 0x02, 0x00, 0x00}, 1); ok || n != 1 {
		t.Fatalf("wrong addr frame: n=%d ok=%v", n, ok)
	}
	// 0x10 写多寄存器: bc=4 → 总长 7+4+2=13
	mreq := make([]byte, 13)
	mreq[0], mreq[1], mreq[6] = 0x01, 0x10, 0x04
	if n, ok := detectRTUFrame(mreq, 1); !ok || n != 13 {
		t.Fatalf("multi-write frame: n=%d ok=%v", n, ok)
	}
}

// TestBroadcastNoResponse 回归: 广播帧 (unit=0) 必须执行写但不产生响应
// (Modbus 规范: 广播不响应, 否则 RTU 总线上多从站同时应答会冲突)。
func TestBroadcastNoResponse(t *testing.T) {
	dev := newTestDevice()
	// 广播写单寄存器 addr=10 val=0x1234: 执行写, 但 resp 必须为 nil
	resp, evt := dev.process(1, 0, []byte{0x06, 0x00, 0x0A, 0x12, 0x34}, "test")
	if resp != nil {
		t.Fatalf("broadcast write must not respond, got resp %x", resp)
	}
	if evt == nil {
		t.Fatal("broadcast write should still emit event")
	}
	if dev.hr[10] != 0x1234 {
		t.Fatalf("broadcast write did not apply: hr[10] = %#x", dev.hr[10])
	}
	// 广播写多寄存器 addr=20 qty=2
	resp, _ = dev.process(1, 0, []byte{0x10, 0x00, 0x14, 0x00, 0x02, 0x04, 0x00, 0x01, 0x00, 0x02}, "test")
	if resp != nil {
		t.Fatalf("broadcast multi-write must not respond, got resp %x", resp)
	}
	if dev.hr[20] != 0x0001 || dev.hr[21] != 0x0002 {
		t.Fatalf("broadcast multi-write not applied: hr[20:22] = %#x", dev.hr[20:22])
	}
	// 广播读: 规范不允许, 静默丢弃 (不执行也不响应, 不产生事件)
	resp, evt = dev.process(1, 0, []byte{0x03, 0x00, 0x0A, 0x00, 0x01}, "test")
	if resp != nil || evt != nil {
		t.Fatalf("broadcast read must be silently dropped, got resp=%x evt=%v", resp, evt)
	}
	// 对照: 单播写必须正常响应 (回归防误伤)
	resp, _ = dev.process(1, 1, []byte{0x06, 0x00, 0x0A, 0xAB, 0xCD}, "test")
	if len(resp) != 5 || resp[0] != 0x06 {
		t.Fatalf("unicast write must respond, got %x", resp)
	}
}