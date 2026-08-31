# MiniGreat-Receiver 多阶段构建
# 用法:
#   docker build -t minigreat-receiver .
#   docker build --build-arg TARGETARCH=arm64 -t minigreat-receiver:arm64 .
# 运行(需要访问串口/SPI/I2C/CAN/BLE 时建议加 --privileged --network host):
#   docker run --rm -it --privileged --network host minigreat-receiver listen --proto tcp --listen :9000
#   docker run --rm -it --privileged --network host -p 8080:8080 minigreat-receiver web --addr 0.0.0.0:8080

# ---------- 构建阶段 ----------
FROM golang:1.24-alpine AS builder

ARG TARGETARCH=amd64
ARG VERSION=1.0.20260901

WORKDIR /src

# 国内网络加速
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/minigreat-receiver .

# ---------- 运行阶段 ----------
FROM alpine:3.21

RUN apk add --no-cache \
    bluez \
    coreutils \
    tzdata \
    ca-certificates \
    && mkdir -p /etc/bluetooth

WORKDIR /app
COPY --from=builder /out/minigreat-receiver /usr/local/bin/minigreat-receiver

ENV TZ=Asia/Shanghai

EXPOSE 9000 9001 8080 502

ENTRYPOINT ["minigreat-receiver"]
CMD ["help"]