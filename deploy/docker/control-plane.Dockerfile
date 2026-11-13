# syntax=docker/dockerfile:1
# 控制面（Control Plane）镜像：多阶段构建，静态编译 + 精简运行镜像。
# 构建上下文为仓库根目录（见 deploy/docker-compose.yml 的 build.context）。

# ---------- 构建阶段 ----------
FROM golang:1.22-alpine AS build

WORKDIR /src

# 先拷贝依赖清单，充分利用 Docker 层缓存（源码变化不触发重新下载依赖）
COPY control-plane/go.mod control-plane/go.sum ./
RUN go mod download

# 拷贝控制面源码并静态编译（CGO_ENABLED=0，产出单二进制）
COPY control-plane/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/control-plane \
      ./cmd/server

# ---------- 运行阶段 ----------
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=build /out/control-plane /app/control-plane
# main.go 以相对路径 "singbox-schema" 加载 JSON Schema；WORKDIR=/app 时即 /app/singbox-schema
COPY --from=build /src/singbox-schema /app/singbox-schema

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/control-plane"]
