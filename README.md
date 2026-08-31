<div align="center">
  <img src="Logo.png" alt="MiniGreat-Receiver" width="120"/>
  <h2>MiniGreat-Receiver</h2>
  <h3>全方面多协议调试接收工具</h3>
</div>

### 一、项目简介

- 一个**全方面多协议调试接收工具**，与 **[MiniGreat-Sender](../MiniGreat-Sender/)** 配套，支持各种协议的服务端监听与数据采集，有线无线全覆盖，纯 Go 实现。
- 本工具负责「收」，Sender 负责「发」，成对使用可完成完整的链路调试；也可以独立运行，作为协议服务端 / 抓包工具使用。
- 提供 **CLI 命令行** 与 **本地 Web 调试面板** 两种使用形态：CLI 适合脚本化与嵌入式环境；Web 面板适合可视化监控接收数据流。
- 全部监听器通过统一抽象（`Config/Event/Listener`）注册，协议可插拔、易扩展。
- **自研 Modbus 从站**（TCP + RTU 双模式），无需第三方库即可应答主站读写请求，读写事件实时上报。

### 二、主要特性

| 类别 | 协议 | 说明 |
|------|------|------|
| 有线网络 | TCP | 服务端监听，多连接接收，可原样回显 |
| 有线网络 | UDP | 服务端监听，接收数据报，可回显 |
| 有线网络 | HTTP | 服务端接收任意方法请求，自定义响应码 / 响应体 / 响应头 |
| 有线网络 | WebSocket | 服务端接收文本 / 二进制消息，可回显 |
| 工控 | MQTT | 订阅一个或多个主题（支持通配符），实时接收 |
| 工控 | Modbus | 自研从站（TCP+RTU），应答 01/02/03/04/05/06/0F/10，读写事件实时上报 |
| 串口 | UART/RS232/RS485 | 监听串口数据（静默分隔成条） |
| 无线射频 | RF-AT 透传 | 读取 LoRa / 433MHz / Zigbee / BLE-SPP 模块上报 |
| 无线 BLE | 蓝牙 | BlueZ 扫描，上报设备地址 / 名称 / RSSI |
| 总线 | CAN | SocketCAN 监听，ID 过滤可选 |
| 总线 | SPI | 周期读取从设备 MISO 数据 |
| 总线 | I2C | 周期轮询从机寄存器 |

- 每收到一条数据实时打印：`[时间] 协议 来源 | 文本 | HEX | 元信息`。
- 跨平台：网络 / 工控 / 串口 / 射频类在 Linux/macOS/Windows 均可运行；CAN / SPI / I2C / BLE 需 Linux（树莓派 / Jetson 等），非 Linux 平台给出明确提示。

### 三、快速开始

> **环境要求**：Go 1.21+。

```bash
make build                          # 或 go build -o minigreat-receiver .
./minigreat-receiver list           # 查看全部支持的监听器
./minigreat-receiver help           # 查看全部选项
```

**网络类：**

```bash
./minigreat-receiver listen --proto tcp --listen :9000 --echo
./minigreat-receiver listen --proto udp --listen :9001
./minigreat-receiver listen --proto http --listen :8080 --body '{"ok":1}' --status 200
./minigreat-receiver listen --proto ws --listen :9002
```

**工控类：**

```bash
./minigreat-receiver listen --proto mqtt --broker tcp://127.0.0.1:1883 --topic test/# --topic data/+
./minigreat-receiver listen --proto modbus --listen :502 --unit 1
./minigreat-receiver listen --proto modbus --device /dev/ttyUSB0 --baud 9600 --unit 1
```

**串口 / 射频：**

```bash
./minigreat-receiver listen --proto serial --device /dev/ttyUSB0 --baud 115200
./minigreat-receiver listen --proto rf --device /dev/ttyUSB0 --baud 9600
```

**总线类（Linux）：**

```bash
./minigreat-receiver listen --proto can --iface vcan0 --filter 0x123
./minigreat-receiver listen --proto spi --spi /dev/spidev0.0 --poll 500
./minigreat-receiver listen --proto i2c --bus 1 --addr 0x48 --reg 0x00 --len 16 --poll 500
```

**蓝牙（Linux, BlueZ）：**

```bash
./minigreat-receiver listen --proto ble --scan 10
```

Ctrl+C 结束监听。

### 四、Web 调试面板

```bash
./minigreat-receiver web --addr 127.0.0.1:8080 --open
```

打开浏览器即得可视化面板：

- 左侧选择协议 → 填写监听参数 → 点「▶ 开始监听」，实时事件流经 WebSocket 推送。
- 同一面板可随时「⏹ 停止」并切换监听器，无需重启。
- 「清空」可重置事件流视图。

### 五、Docker 部署

```bash
# 构建（amd64 默认；树莓派 / Jetson 用 arm64）
make docker                          # 或 make docker TARGETARCH=arm64

# 运行 CLI（硬件协议需 --privileged --network host）
docker run --rm -it --privileged --network host minigreat-receiver:latest \
    listen --proto tcp --listen :9000

# 运行 Web 面板（已配置硬件透传示例）
docker compose up -d receiver-web
```

> 串口 / SPI / I2C / CAN / BLE 需要容器以 `--privileged --network host` 运行，并通过 `devices` / `volumes` 透传宿主设备与 D-Bus（见 `docker-compose.yml`）。镜像为 Alpine 静态二进制，任意 Linux 主机一键部署（含跨架构）。

### 六、目录结构

```
MiniGreat-Receiver/
├─ main.go                     # 入口
├─ internal/
│  ├─ cli/                     # 命令行子命令（listen / web / list）
│  ├─ core/                    # 核心抽象：Config / Event / Listener
│  ├─ listener/                # 各协议监听器
│  │  ├─ netlst/               # TCP / UDP / HTTP / WebSocket
│  │  ├─ mqttlst/              # MQTT 订阅
│  │  ├─ modbuslst/            # Modbus 从站（自研 TCP / RTU）
│  │  ├─ serlst/               # 串口 / 射频
│  │  ├─ canlst/  spilst/  i2clst/  blelst/   # Linux 硬件监听（+非 Linux 桩）
│  └─ web/                     # Web 调试面板（go:embed 内嵌静态资源）
├─ Dockerfile
├─ docker-compose.yml
├─ Makefile
├─ go.mod / go.sum
├─ Logo.png
└─ README.md / README_en.md
```

### 七、平台支持

| 类别 | Linux | macOS / Windows |
|------|:-----:|:---------------:|
| TCP / UDP / HTTP / WS / MQTT / Modbus | ✅ | ✅ |
| 串口 / RF-AT | ✅ | ✅ |
| CAN / SPI / I2C / BLE | ✅ | ❌（提示改用 Linux） |
| Docker | ✅（amd64 / arm64） | — |

### 八、License

Apache License 2.0
