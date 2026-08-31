// Package registry 汇总注册全部接收监听器。
package registry

import (
	"minigreat-receiver/internal/core"
	"minigreat-receiver/internal/listener/blelst"
	"minigreat-receiver/internal/listener/canlst"
	"minigreat-receiver/internal/listener/i2clst"
	"minigreat-receiver/internal/listener/modbuslst"
	"minigreat-receiver/internal/listener/mqttlst"
	"minigreat-receiver/internal/listener/netlst"
	"minigreat-receiver/internal/listener/serlst"
	"minigreat-receiver/internal/listener/spilst"
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