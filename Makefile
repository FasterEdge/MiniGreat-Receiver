# MiniGreat-Receiver Makefile
VERSION ?= 1.0.20260902
BIN     := minigreat-receiver
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build darwin linux linux-arm64 linux-amd64 test vet clean docker docker-arm64 run-web

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

darwin:
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-darwin-arm64 .

linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-arm64 .

linux-amd64:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-amd64 .

linux-arm64:
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)-linux-arm64 .

test:
	go vet ./...
	go test ./...

docker:
	@test -n "$(TARGETARCH)" || TARGETARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
	echo "build docker for $$TARGETARCH"; \
	docker build --build-arg TARGETARCH=$$TARGETARCH --build-arg VERSION=$(VERSION) -t $(BIN):latest .

docker-arm64:
	docker build --build-arg TARGETARCH=arm64 --build-arg VERSION=$(VERSION) -t $(BIN):arm64 .

run-web:
	docker run --rm -it --privileged --network host $(BIN):latest web --addr 0.0.0.0:8080

clean:
	rm -f $(BIN) $(BIN)-*