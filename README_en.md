<div align="center">
  <img src="Logo.png" alt="MiniGreat-Receiver" width="120"/>
  <h2>MiniGreat-Receiver</h2>
  <h3>All-in-One Multi-Protocol Debugging & Receiving Tool</h3>
</div>

### 1. Introduction

- An **all-in-one multi-protocol debugging & receiving tool**, the companion of **[MiniGreat-Sender](../MiniGreat-Sender/)**: it "receives" while Sender "sends", together covering full end-to-end link debugging. It can also run standalone as a protocol server / sniffer.
- Pure Go implementation, covering both wired and wireless links.
- Two usage modes: a **CLI** for scripting and embedded environments, and a **local Web debug panel** for visual monitoring of incoming data.
- All listeners are registered through a unified abstraction (`Config/Event/Listener`), making protocols pluggable and easy to extend.
- **Self-built Modbus slave** (TCP + RTU): no third-party library needed to answer master read/write requests, with read/write events reported in real time.

### 2. Key Features

| Category | Protocol | Description |
|----------|----------|-------------|
| Wired network | TCP | Server-side listener, multiple connections, optional echo |
| Wired network | UDP | Server-side listener, receive datagrams, optional echo |
| Wired network | HTTP | Receive any method; customizable status / body / response headers |
| Wired network | WebSocket | Receive text / binary messages, optional echo |
| Industrial | MQTT | Subscribe to one or more topics (wildcards supported) |
| Industrial | Modbus | Self-built slave (TCP+RTU) answering 01/02/03/04/05/06/0F/10, events live |
| Serial | UART/RS232/RS485 | Watch serial data (split into frames on silence) |
| Wireless RF | RF-AT passthrough | Read LoRa / 433MHz / Zigbee / BLE-SPP module output |
| Wireless BLE | Bluetooth | BlueZ scan, report address / name / RSSI |
| Bus | CAN | SocketCAN listener, optional ID filter |
| Bus | SPI | Periodically read slave MISO data |
| Bus | I2C | Periodically poll slave registers |

- Each received message is printed live: `[time] protocol source | text | HEX | metadata`.
- Cross-platform: network / industrial / serial / RF protocols run on Linux, macOS and Windows; CAN / SPI / I2C / BLE require Linux (Raspberry Pi / Jetson etc.) and give a clear notice on other platforms.

### 3. Quick Start

> **Requirements**: Go 1.21+.

```bash
make build                          # or: go build -o minigreat-receiver .
./minigreat-receiver list           # list all supported listeners
./minigreat-receiver help           # show all options
```

**Network:**

```bash
./minigreat-receiver listen --proto tcp --listen :9000 --echo
./minigreat-receiver listen --proto udp --listen :9001
./minigreat-receiver listen --proto http --listen :8080 --body '{"ok":1}' --status 200
./minigreat-receiver listen --proto ws --listen :9002
```

**Industrial:**

```bash
./minigreat-receiver listen --proto mqtt --broker tcp://127.0.0.1:1883 --topic test/# --topic data/+
./minigreat-receiver listen --proto modbus --listen :502 --unit 1
./minigreat-receiver listen --proto modbus --device /dev/ttyUSB0 --baud 9600 --unit 1
```

**Serial / RF:**

```bash
./minigreat-receiver listen --proto serial --device /dev/ttyUSB0 --baud 115200
./minigreat-receiver listen --proto rf --device /dev/ttyUSB0 --baud 9600
```

**Bus (Linux):**

```bash
./minigreat-receiver listen --proto can --iface vcan0 --filter 0x123
./minigreat-receiver listen --proto spi --spi /dev/spidev0.0 --poll 500
./minigreat-receiver listen --proto i2c --bus 1 --addr 0x48 --reg 0x00 --len 16 --poll 500
```

**Bluetooth (Linux, BlueZ):**

```bash
./minigreat-receiver listen --proto ble --scan 10
```

Press Ctrl+C to stop listening.

### 4. Web Debug Panel

```bash
./minigreat-receiver web --addr 127.0.0.1:8080 --open
```

Open the browser for a visual panel:

- Pick a protocol on the left → fill in listen parameters → click "▶ Start" and events stream live via WebSocket.
- The same panel can "⏹ Stop" and switch listeners at any time without restart.
- "Clear" resets the event-stream view.

### 5. Docker Deployment

```bash
# Build (amd64 by default; use arm64 for Raspberry Pi / Jetson)
make docker                        # or: make docker TARGETARCH=arm64

# Run CLI (hardware protocols need --privileged --network host)
docker run --rm -it --privileged --network host minigreat-receiver:latest \
    listen --proto tcp --listen :9000

# Run the Web panel (hardware passthrough already configured)
docker compose up -d receiver-web
```

> Serial / SPI / I2C / CAN / BLE require the container to run with `--privileged --network host` and to pass through host devices and D-Bus via `devices` / `volumes` (see `docker-compose.yml`). The image is an Alpine static binary, deployable on any Linux host with one command (including cross-arch).

### 6. Directory Structure

```
MiniGreat-Receiver/
├─ main.go                     # Entry point
├─ internal/
│  ├─ cli/                     # CLI subcommands (listen / web / list)
│  ├─ core/                    # Core abstractions: Config / Event / Listener
│  ├─ listener/                # Protocol listeners
│  │  ├─ netlst/               # TCP / UDP / HTTP / WebSocket
│  │  ├─ mqttlst/              # MQTT subscribe
│  │  ├─ modbuslst/            # Modbus slave (self-built TCP / RTU)
│  │  ├─ serlst/               # Serial / RF
│  │  ├─ canlst/  spilst/  i2clst/  blelst/   # Linux hardware listeners (+ non-Linux stubs)
│  └─ web/                     # Web debug panel (embedded via go:embed)
├─ Dockerfile
├─ docker-compose.yml
├─ Makefile
├─ go.mod / go.sum
├─ Logo.png
└─ README.md / README_en.md
```

### 7. Platform Support

| Category | Linux | macOS / Windows |
|----------|:-----:|:---------------:|
| TCP / UDP / HTTP / WS / MQTT / Modbus | ✅ | ✅ |
| Serial / RF-AT | ✅ | ✅ |
| CAN / SPI / I2C / BLE | ✅ | ❌ (use Linux) |
| Docker | ✅ (amd64 / arm64) | — |

### 8. License

Apache License 2.0
