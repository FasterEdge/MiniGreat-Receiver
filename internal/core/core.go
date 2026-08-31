// Package core 定义 MiniGreat-Receiver 的核心抽象:
// 监听器接口(Listener)、配置(Config)与事件(Event)。
package core

import (
	"context"
	"time"
)

// Config 是一次监听会话的配置, 不同监听器只取用自己关心的字段。
type Config struct {
	// 监听器名, 例如 tcp/udp/http/ws/mqtt/modbus/serial/ble/can/spi/i2c/rf
	Protocol string `json:"protocol" yaml:"protocol"`

	// ---- 通用 ----
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// ---- 网络监听: tcp/udp/http/ws ----
	ListenAddr string `json:"listenAddr,omitempty" yaml:"listenAddr"` // :9000
	// HTTP 服务可选的响应码/响应体(回显用)
	HTTPStatusCode int    `json:"httpStatusCode,omitempty" yaml:"httpStatusCode"`
	HTTPBody       string `json:"httpBody,omitempty" yaml:"httpBody"`
	HTTPHeaders    map[string]string `json:"httpHeaders,omitempty" yaml:"httpHeaders"`
	Echo           bool   `json:"echo,omitempty" yaml:"echo"` // 收到数据后原样回显

	// ---- mqtt ----
	Broker   string `json:"broker,omitempty" yaml:"broker"`
	ClientID string `json:"clientId,omitempty" yaml:"clientId"`
	Username string `json:"username,omitempty" yaml:"username"`
	Password string `json:"password,omitempty" yaml:"password"`
	Topics   []string `json:"topics,omitempty" yaml:"topics"`

	// ---- modbus ----
	ModbusUnitID   byte   `json:"modbusUnitId,omitempty" yaml:"modbusUnitId"`
	ModbusBaud     int    `json:"modbusBaud,omitempty" yaml:"modbusBaud"`
	ModbusParity   string `json:"modbusParity,omitempty" yaml:"modbusParity"`
	ModbusDataBits int    `json:"modbusDataBits,omitempty" yaml:"modbusDataBits"`
	ModbusStopBits int    `json:"modbusStopBits,omitempty" yaml:"modbusStopBits"`

	// ---- serial ----
	SerialDevice   string `json:"serialDevice,omitempty" yaml:"serialDevice"`
	SerialBaud     int    `json:"serialBaud,omitempty" yaml:"serialBaud"`
	SerialDataBits int    `json:"serialDataBits,omitempty" yaml:"serialDataBits"`
	SerialParity   string `json:"serialParity,omitempty" yaml:"serialParity"`
	SerialStopBits int    `json:"serialStopBits,omitempty" yaml:"serialStopBits"`

	// ---- ble ----
	BLEScanSeconds int `json:"bleScanSeconds,omitempty" yaml:"bleScanSeconds"`

	// ---- can ----
	CANInterface string `json:"canInterface,omitempty" yaml:"canInterface"`
	CANFilterID  uint32 `json:"canFilterId,omitempty" yaml:"canFilterId"` // 0 = 不过滤

	// ---- spi ----
	SPIDevice string `json:"spiDevice,omitempty" yaml:"spiDevice"`
	SPIMode   uint8  `json:"spiMode,omitempty" yaml:"spiMode"`
	SPIBits   uint8  `json:"spiBits,omitempty" yaml:"spiBits"`
	SPISpeed  int64  `json:"spiSpeed,omitempty" yaml:"spiSpeed"`
	SPIPollMS int    `json:"spiPollMs,omitempty" yaml:"spiPollMs"`

	// ---- i2c ----
	I2CBus      int    `json:"i2cBus,omitempty" yaml:"i2cBus"`
	I2CAddr     int    `json:"i2cAddr,omitempty" yaml:"i2cAddr"`
	I2CRegister int    `json:"i2cRegister,omitempty" yaml:"i2cRegister"`
	I2CLen      int    `json:"i2cLen,omitempty" yaml:"i2cLen"`
	I2CPollMS   int    `json:"i2cPollMs,omitempty" yaml:"i2cPollMs"`
}

// Event 是监听器收到的一条消息/事件。
type Event struct {
	Protocol string         `json:"protocol"`
	Time     string         `json:"time"`
	Source   string         `json:"source,omitempty"` // 来源(如远端地址/设备名)
	Data     []byte         `json:"data,omitempty"`
	DataHex  string         `json:"dataHex,omitempty"`
	DataTxt  string         `json:"dataTxt,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

// Sink 接收事件的回调。
type Sink func(Event)

// Listener 是所有协议监听器的统一接口。
type Listener interface {
	// Name 返回监听器名。
	Name() string
	// Description 返回一句话描述。
	Description() string
	// Validate 校验配置。
	Validate(cfg *Config) error
	// Run 启动监听, 阻塞直到 ctx 取消或出错; 每收到数据调用 sink。
	// 返回 nil 表示正常结束(如 BLE 扫描完成), 返回 error 表示出错。
	Run(ctx context.Context, cfg *Config, sink Sink) error
}

// Registry 监听器注册表。
type Registry struct {
	listeners map[string]Listener
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{listeners: make(map[string]Listener)}
}

// Register 注册监听器。
func (r *Registry) Register(l Listener) {
	r.listeners[l.Name()] = l
}

// Get 按名获取。
func (r *Registry) Get(name string) (Listener, bool) {
	l, ok := r.listeners[name]
	return l, ok
}

// Names 返回全部监听器名。
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.listeners))
	for n := range r.listeners {
		out = append(out, n)
	}
	return out
}

// FormatDataHex 十六进制文本。
func FormatDataHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*3)
	for i, c := range b {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, hexdigits[c>>4], hexdigits[c&0x0f])
	}
	return string(out)
}

// FormatDataTxt 可打印文本, 不可打印转义。
func FormatDataTxt(b []byte) string {
	out := make([]rune, 0, len(b))
	for _, c := range b {
		if c >= 0x20 && c < 0x7f {
			out = append(out, rune(c))
		} else if c == '\n' {
			out = append(out, '\\', 'n')
		} else if c == '\r' {
			out = append(out, '\\', 'r')
		} else {
			out = append(out, '\\', 'x')
			const hexdigits = "0123456789abcdef"
			out = append(out, rune(hexdigits[c>>4]), rune(hexdigits[c&0x0f]))
		}
	}
	return string(out)
}