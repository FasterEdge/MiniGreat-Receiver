// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package registry 汇总注册全部接收监听器。
package registry

import (
	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/blelst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/canlst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/i2clst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/modbuslst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/mqttlst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/netlst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/serlst"
	"github.com/FasterEdge/MiniGreat-Receiver/internal/listener/spilst"
)

// New 创建并注册全部监听器。
func New() *core.Registry {
	r := core.NewRegistry()
	listeners := []core.Listener{
		netlst.TCPListener{},
		netlst.UDPListener{},
		netlst.HTTPListener{},
		netlst.WSListener{},
		mqttlst.MQTTListener{},
		modbuslst.ModbusListener{},
		serlst.SerialListener{},
		serlst.RFListener{},
		canlst.CANListener{},
		spilst.SPIListener{},
		i2clst.I2CListener{},
		blelst.BLEListener{},
	}
	for _, l := range listeners {
		r.Register(l)
	}
	return r
}
