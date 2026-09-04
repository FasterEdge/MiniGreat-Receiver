// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
// Package mqttlst 提供 MQTT 订阅监听器。
package mqttlst

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/FasterEdge/MiniGreat-Receiver/internal/core"
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
	if cfg.QoS > 2 {
		return fmt.Errorf("mqtt: qos 必须在 0~2 之间")
	}
	return nil
}

// randHex 返回 n 字节随机数的十六进制 (用于生成低碰撞概率的默认 ClientID)。
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Run 订阅并阻塞转发消息。
func (MQTTListener) Run(ctx context.Context, cfg *core.Config, sink core.Sink) error {
	clientID := cfg.ClientID
	if clientID == "" {
		// 时间戳+随机数: 仅时间戳取模碰撞空间小, 多实例并发订阅会被 broker 踢旧连接
		clientID = fmt.Sprintf("minigreat-receiver-%d-%s", time.Now().UnixNano(), randHex(4))
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
	if err := waitToken(ctx, tok, 5*time.Second); err != nil {
		return fmt.Errorf("mqtt: 连接失败: %w", err)
	}
	defer client.Disconnect(100)

	handler := mqtt.MessageHandler(func(c mqtt.Client, msg mqtt.Message) {
		data := append([]byte(nil), msg.Payload()...)
		sink(core.Event{
			Protocol: "mqtt", Time: time.Now().Format("15:04:05.000"), Source: msg.Topic(),
			Data: data, DataHex: core.FormatDataHex(data), DataTxt: core.FormatDataTxt(data),
			Meta: map[string]any{
				"topic":    msg.Topic(),
				"qos":      msg.Qos(),
				"retained": msg.Retained(),
				"clientId": clientID,
			},
		})
	})
	for _, t := range cfg.Topics {
		ptok := client.Subscribe(t, cfg.QoS, handler)
		if err := waitToken(ctx, ptok, 5*time.Second); err != nil {
			return fmt.Errorf("mqtt: 订阅 %s 失败: %w", t, err)
		}
	}
	sink(core.Event{Protocol: "mqtt", Time: time.Now().Format("15:04:05.000"),
		Source: cfg.Broker, DataTxt: "MQTT 已连接并订阅: " + join(cfg.Topics)})

	<-ctx.Done()
	return nil
}

// waitToken 等待 MQTT token 完成; ctx 取消/超时优先于 token, 保证调用方中止能立即返回。
func waitToken(ctx context.Context, tok mqtt.Token, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-tok.Done():
		return tok.Error()
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timeout after %v", timeout)
	}
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
