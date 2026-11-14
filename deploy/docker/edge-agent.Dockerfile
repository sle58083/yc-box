# syntax=docker/dockerfile:1
# 边缘 Agent（Edge Agent）镜像：多阶段构建，静态编译 + 精简运行镜像。
# 构建上下文为仓库根目录（见 deploy/docker-compose.yml 的 build.context）。

# ---------- 构建阶段 1：编译 edge-agent ----------
FROM golang:1.22-alpine AS build-agent

WORKDIR /src

COPY edge-agent/go.mod edge-agent/go.sum ./
RUN go mod download

COPY edge-agent/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/edge-agent \
      ./cmd/agent

# ---------- 构建阶段 2：编译 sing-box（带 with_v2ray_api） ----------
# 官方预编译二进制默认不含 v2ray_api（流量统计 gRPC 接口），故此处自编译。
# sing-box v1.14.4 需 Go 1.25.5+，故用独立高版本 Go 阶段（与 edge-agent 的 Go 1.22 解耦）。
FROM golang:1.26-alpine AS build-singbox

ARG TARGETARCH
ARG SINGBOX_VERSION=1.14.4
# 官方默认 tags（release/DEFAULT_BUILD_TAGS）去掉 with_naive_outbound（需 cronet/Chromium CGO）
# 和 with_tailscale（需 derp 特殊依赖），再加上 with_v2ray_api（流量统计 gRPC 接口）。
# 这些 tag 在 CGO_ENABLED=0 纯静态交叉编译下均可构建。
ARG SINGBOX_TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_v2ray_api,with_ccm,with_ocm,with_cloudflared,with_usbip,with_openvpn,with_openconnect,badlinkname,tfogo_checklinkname0"
ARG SINGBOX_LDFLAGS="-X runtime.godebugDefault=multipathtcp=0,tlssha1=1 -checklinkname=0"

RUN apk add --no-cache git ca-certificates
WORKDIR /singbox
# 拉取指定版本源码（shallow clone 指定 tag，节省时间与体积）
RUN git clone --depth 1 --branch "v${SINGBOX_VERSION}" https://github.com/SagerNet/sing-box.git . \
    && go mod download

# 交叉编译 sing-box（静态编译，兼容 glibc 运行环境）
RUN case "${TARGETARCH}" in \
      amd64) export GOARCH=amd64 ;; \
      arm64) export GOARCH=arm64 ;; \
      arm)   export GOARCH=arm GOARM=7 ;; \
      *)     echo "unsupported TARGETARCH: ${TARGETARCH}"; exit 1 ;; \
    esac && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -tags "${SINGBOX_TAGS}" \
      -ldflags "${SINGBOX_LDFLAGS} -s -w" \
      -o /out/sing-box \
      ./cmd/sing-box

# ---------- 运行阶段 ----------
# 用 debian-slim（glibc）而非 alpine（musl）：自编译 sing-box 是 glibc 环境编译，
# 在 musl 上会报 "no such file or directory"（动态链接器缺失）。
FROM debian:bookworm-slim

# ca-certificates/tzdata：处理时区、entrypoint 依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH

# 内置 sing-box 内核二进制（单机部署必需：Agent 启动时内核即就位，无需运行时下载）。
# 从 build-singbox 阶段复制；放到 agent 期望的 bin/current 路径。
RUN mkdir -p /opt/singbox-agent/bin/current
COPY --from=build-singbox /out/sing-box "/opt/singbox-agent/bin/current/sing-box-linux-${TARGETARCH}"
RUN chmod +x "/opt/singbox-agent/bin/current/sing-box-linux-${TARGETARCH}"

# entrypoint：把镜像内置内核同步到数据卷（若卷内缺失），再启动 agent。
# 原因：compose 将 agent_data 卷挂载到 /opt/singbox-agent，会遮蔽镜像内置的
# bin/current 内容，故首次启动时从镜像内置路径复制一份到卷内。
COPY <<'ENTRY' /entrypoint.sh
#!/bin/sh
set -e
# 数据目录（卷挂载点）
DATADIR="${SBAGENT_AGENT_DATADIR:-/opt/singbox-agent}"
# 镜像内置内核位置（非卷路径，避免被挂载遮蔽）
BUILTIN_KERNEL="/opt/singbox-builtin-kernel"
if [ -d "$BUILTIN_KERNEL" ]; then
  mkdir -p "$DATADIR/bin/current"
  # 仅当卷内没有对应内核时才复制（幂等，保留卷内已激活版本）
  for k in "$BUILTIN_KERNEL"/sing-box-*; do
    [ -f "$k" ] || continue
    dst="$DATADIR/bin/current/$(basename "$k")"
    if [ ! -f "$dst" ]; then
      cp "$k" "$dst"
      chmod +x "$dst"
      echo "[entrypoint] 从镜像内置内核复制 $(basename "$k") 到数据卷"
    fi
  done
fi
exec edge-agent
ENTRY

# 把内置内核同时复制到非卷路径，供 entrypoint 首次启动时同步到卷
RUN mkdir -p /opt/singbox-builtin-kernel && \
    cp /opt/singbox-agent/bin/current/sing-box-* /opt/singbox-builtin-kernel/ 2>/dev/null || true

RUN chmod +x /entrypoint.sh

# 数据目录（bin/etc/run/logs 布局），运行时挂卷持久化（compose 中 agent_data）
WORKDIR /opt/singbox-agent

COPY --from=build-agent /out/edge-agent /usr/local/bin/edge-agent

# 默认数据目录；可被 compose 的 SBAGENT_AGENT_DATADIR 覆盖
ENV SBAGENT_AGENT_DATADIR=/opt/singbox-agent

# 以 root 运行：sing-box 子进程需绑定低位端口（如 443），并为 TUN 预留能力
# 若需 TUN 模式，请在 compose 中额外挂载 /dev/net/tun 并加 cap_add: NET_ADMIN
# 注：EXPOSE 仅为元数据；实际端口映射由 docker-compose.yml 的 ports 控制
EXPOSE 10000-10100

ENTRYPOINT ["/entrypoint.sh"]
