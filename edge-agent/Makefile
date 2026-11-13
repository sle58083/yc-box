# edge-agent 构建脚本（本地/CI 调试用，GoReleaser 见 .goreleaser.yaml）
GOOS    ?= linux
GOARCH  ?= amd64
GOARM   ?= 7
VERSION ?= dev
BIN     := singbox-agent
DIST    := dist

.PHONY: build cross clean

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) \
		go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" \
		-o $(DIST)/$(BIN)-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,) ./cmd/agent

# 五类目标矩阵（T03 验收硬性要求）
cross:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/$(BIN)-linux-amd64 ./cmd/agent
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/$(BIN)-linux-arm64 ./cmd/agent
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm   GOARM=7 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/$(BIN)-linux-armv7 ./cmd/agent
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/$(BIN)-windows-amd64.exe ./cmd/agent
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/$(BIN)-windows-arm64.exe ./cmd/agent

clean:
	rm -rf $(DIST)
