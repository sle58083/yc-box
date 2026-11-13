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
FROM alpine:3.19

# ca-certificates/tzdata：Agent 通过 HTTPS 下载 sing-box 内核、处理时区
RUN apk add --no-cache ca-certificates tzdata

# 数据目录（bin/etc/run/logs 布局），运行时挂卷持久化（compose 中 agent_data）
WORKDIR /opt/singbox-agent

COPY --from=build /out/edge-agent /usr/local/bin/edge-agent

# 默认数据目录；可被 compose 的 SBAGENT_AGENT_DATADIR 覆盖
ENV SBAGENT_AGENT_DATADIR=/opt/singbox-agent

# 以 root 运行：sing-box 子进程需绑定低位端口（如 443），并为 TUN 预留能力
# 若需 TUN 模式，请在 compose 中额外挂载 /dev/net/tun 并加 cap_add: NET_ADMIN
# 注：EXPOSE 仅为元数据；实际端口映射由 docker-compose.yml 的 ports 控制
EXPOSE 10000-10100

ENTRYPOINT ["edge-agent"]
