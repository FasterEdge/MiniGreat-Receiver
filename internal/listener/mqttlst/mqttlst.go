// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package mqttlst 提供 MQTT 订阅监听器。
package mqttlst

import (
	"context"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"minigreat-receiver/internal/core"
)

// MQTTListener 订阅一个或多个主题并把消息转为事件。
type MQTTListener struct{}

// Name 返回监听器名。
func (MQTTListener) Name() string { return "mqtt" }

// Description 返回描述。
func (MQTTListener) Description() string { return "MQTT 订阅端: 订阅主题, 实时接收消息" }

// Validate 校验参数。
func (MQTTListener) Validate(cfg *core.Config) error {
	if cfg.Broker == "" {
		return fmt.Errorf("mqtt: broker 不能为空 (如 tcp://127.0.0.1:1883)")
	}
	if len(cfg.Topics) == 0 {
		return fmt.Errorf("mqtt: topics 不能为空")
	}
	return nil
}

// Run 订阅并阻塞转发消息。
func (MQTTListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	clientID := cfg.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("minigreat-receiver-%d", time.Now().UnixNano()%100000)
	}
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(clientID).
		SetConnectTimeout(5 * time.Second).
		SetAutoReconnect(true)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	client := mqtt.NewClient(opts)
	tok := client.Connect()
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		return fmt.Errorf("mqtt: 连接失败: %v", tok.Error())
	}
	defer client.Disconnect(100)

	handler := mqtt.MessageHandler(func(c mqtt.Client, msg mqtt.Message) {
		data := append([]byte(nil), msg.Payload()...)
		sink(core.Event{
			Protocol: "mqtt", Time: time.Now().Format("15:04:05.000"), Source: msg.Topic(),
			Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data),
			Meta: map[string]any{
				"topic":  msg.Topic(),
				"qos":    msg.Qos(),
				"retained": msg.Retained(),
				"clientId": clientID,
			},
		})
	})
	for _, t := range cfg.Topics {
		ptok := client.Subscribe(t, 0, handler)
		if !ptok.WaitTimeout(5*time.Second) || ptok.Error() != nil {
			return fmt.Errorf("mqtt: 订阅 %s 失败: %v", t, ptok.Error())
		}
	}
	sink(core.Event{Protocol: "mqtt", Time: time.Now().Format("15:04:05.000"),
		Source: cfg.Broker, DataTxt: "MQTT 已连接并订阅: " + join(cfg.Topics)})

	<-ctx.Done()
	return nil
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}