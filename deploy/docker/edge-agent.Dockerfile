# syntax=docker/dockerfile:1
# 边缘 Agent（Edge Agent）镜像：多阶段构建，静态编译 + 精简运行镜像。
# 构建上下文为仓库根目录（见 deploy/docker-compose.yml 的 build.context）。

# ---------- 构建阶段 ----------
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY edge-agent/go.mod edge-agent/go.sum ./
RUN go mod download

COPY edge-agent/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/edge-agent \
      ./cmd/agent

# ---------- 运行阶段 ----------
# 用 debian-slim（glibc）而非 alpine（musl）：sing-box 官方二进制是 glibc 编译，
# 在 musl 上会报 "no such file or directory"（动态链接器缺失）。
FROM debian:bookworm-slim

# ca-certificates/wget/tzdata：下载 sing-box 内核、处理时区、entrypoint 依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates wget tzdata \
    && rm -rf /var/lib/apt/lists/*

# 内置 sing-box 内核二进制（单机部署必需：Agent 启动时内核即就位，无需运行时下载）。
# 用 TARGETARCH 匹配多架构；放到 agent 期望的 bin/current 路径。
# 版本可用 SINGBOX_VERSION 构建参数覆盖（默认跟随最新稳定版）。
ARG TARGETARCH
ARG SINGBOX_VERSION=1.14.4
RUN mkdir -p /opt/singbox-agent/bin/current && \
    case "${TARGETARCH}" in \
      amd64) SB_ARCH="amd64" ;; \
      arm64) SB_ARCH="arm64" ;; \
      arm)   SB_ARCH="armv7" ;; \
      *)     echo "unsupported TARGETARCH: ${TARGETARCH}"; exit 1 ;; \
    esac && \
    wget -q "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${SB_ARCH}.tar.gz" -O /tmp/sing-box.tar.gz && \
    tar -xzf /tmp/sing-box.tar.gz -C /tmp && \
    mv "/tmp/sing-box-${SINGBOX_VERSION}-linux-${SB_ARCH}/sing-box" "/opt/singbox-agent/bin/current/sing-box-linux-${TARGETARCH}" && \
    chmod +x "/opt/singbox-agent/bin/current/sing-box-linux-${TARGETARCH}" && \
    rm -rf /tmp/sing-box.tar.gz "/tmp/sing-box-${SINGBOX_VERSION}-linux-${SB_ARCH}"

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

COPY --from=build /out/edge-agent /usr/local/bin/edge-agent

# 默认数据目录；可被 compose 的 SBAGENT_AGENT_DATADIR 覆盖
ENV SBAGENT_AGENT_DATADIR=/opt/singbox-agent

# 以 root 运行：sing-box 子进程需绑定低位端口（如 443），并为 TUN 预留能力
# 若需 TUN 模式，请在 compose 中额外挂载 /dev/net/tun 并加 cap_add: NET_ADMIN
# 注：EXPOSE 仅为元数据；实际端口映射由 docker-compose.yml 的 ports 控制
EXPOSE 10000-10100

ENTRYPOINT ["/entrypoint.sh"]
