# sing-box 企业级代理管理面板 — 启动与部署指南

> 本文档是「如何把项目跑起来」的操作手册，配合 `docs/system_design.md`（单一事实源）阅读。

## 1. 项目结构

```
E:/sing-box面板开发/
├── control-plane/     # Go 控制面（中心管理端）
├── edge-agent/        # Go 边缘 Agent（部署在每台 sing-box 节点服务器上）
├── web/               # React 前端（响应式管理界面）
└── docs/              # 设计文档（PRD / system_design / 跨平台 / 协议矩阵）
```

## 2. 依赖环境

| 依赖 | 版本 | 用途 | 必需 |
|------|------|------|------|
| Go | ≥ 1.22 | 编译控制面 + Agent | 是 |
| SQLite | 内嵌（glebarez/sqlite） | 控制面主数据库（单文件，无外部服务） | 是 |
| Node.js | ≥ 18 | 前端构建/开发 | 是 |
| sing-box | 最新 stable | 节点内核（外部二进制，Agent 托管） | 是（跑节点需要） |

## 3. 控制面（Control Plane）启动

### 3.1 数据库（无需手动准备）

控制面使用 SQLite 单文件数据库（`database.path`，默认 `data/panel.db`），首次启动时自动创建数据目录与表结构，**无需安装或启动 PostgreSQL / Redis**。

### 3.2 配置文件

复制 `control-plane/config.example.yaml` 为 `control-plane/config.yaml`，修改关键项：

```yaml
server:
  port: 8080

database:
  path: data/panel.db   # SQLite 数据文件路径（相对/绝对），首次启动自动创建目录

jwt:
  secret: "换成你自己的长随机字符串"   # 生产务必替换

crypto:
  key: "<openssl rand -hex 32 生成>"   # 32 字节 AES-256 密钥，生产务必替换

bootstrap:
  admin_username: admin                 # 首个超级管理员用户名
  admin_password: "你的初始密码"        # 留空则不自动创建
```

> **关键**：`crypto.key` 是 AES-256 密钥（hex 编码 64 字符），用 `openssl rand -hex 32` 生成。它负责加密落库的 TLS 私钥、代理用户 UUID/密码、订阅令牌等敏感信息，**一旦设置后不可更换**（更换会导致已加密数据无法解密）。

### 3.3 编译并启动

```bash
cd control-plane
go build -o bin/server ./cmd/server
./bin/server   # 或 go run ./cmd/server
```

启动时会自动：
1. 连接 SQLite 并 **AutoMigrate**（自动建表）
2. 幂等初始化内置三角色（super_admin / admin / viewer）
3. 若配置了 `bootstrap.admin_password` 且面板无用户，**创建首个超级管理员**
4. 监听 `:8080`

验证：`curl http://localhost:8080/healthz` 返回 `{"code":0,...}`。

> **首次登录**：用 `bootstrap.admin_username` / `bootstrap.admin_password` 登录前端，**登录后请立即修改密码**。

### 3.4 环境变量（可选覆盖）

所有配置项支持环境变量覆盖，前缀 `SBP_`（点号转下划线）：

```bash
SBP_SERVER_PORT=8080
SBP_DATABASE_PATH=data/panel.db
SBP_BOOTSTRAP_ADMIN_USERNAME=admin
SBP_BOOTSTRAP_ADMIN_PASSWORD=xxx
SBP_CRYPTO_KEY=xxx
```

## 4. 前端（Web）启动

### 4.1 开发模式

```bash
cd web
npm install
npm run dev      # 启动 Vite，端口 3000
```

Vite 已配置将 `/api` 代理到 `http://localhost:8080`（控制面），无需额外 CORS 配置。

浏览器访问 `http://localhost:3000`。

### 4.2 生产构建

```bash
cd web
npm run build    # tsc + vite build，产物在 dist/
```

生产部署时将 `dist/` 交给 Nginx 等静态服务器，并把 `/api` 反向代理到控制面。

## 5. 边缘 Agent 启动

Agent 部署在每台承载 sing-box 的节点服务器上（可与控制面不同系统，支持 Linux/Windows/macOS/嵌入式）。

### 5.1 配置文件

复制 `edge-agent/agent.example.yaml` 为 `edge-agent/agent.yaml`：

```yaml
agent:
  datadir: /opt/singbox-agent          # Windows: C:\ProgramData\singbox-agent
  kernel:
    version: ""                        # 留空则跟随控制面下发
  proc:
    stop_grace: 10s
    health_interval: 3s

control:
  server_url: "ws://控制面IP:8080/ws/agent"   # 生产用 wss://
  token: "控制面创建节点时返回的令牌"
  heartbeat: 5s

log:
  level: info
```

### 5.2 编译并启动

```bash
cd edge-agent
go build -o bin/agent ./cmd/agent
./bin/agent       # 或 go run ./cmd/agent
```

> **Agent 令牌获取**：在控制面前端「节点」页创建节点时，令牌会**明文一次性返回**（落库仅存 SHA-256 哈希），务必当场保存，填入 Agent 的 `control.token`。

### 5.3 跨平台编译

Agent 支持交叉编译（CGO_ENABLED=0 静态编译）：

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o bin/agent.exe ./cmd/agent
# Linux ARM64（树莓派/ARM VPS）
GOOS=linux GOARCH=arm64 go build -o bin/agent ./cmd/agent
# Linux ARMv7（老树莓派/软路由）
GOOS=linux GOARCH=arm GOARM=7 go build -o bin/agent ./cmd/agent
```

或用 `Makefile cross` / `.goreleaser.yaml` 一次产出五类二进制。

## 6. sing-box 内核准备

Agent 通过 `kernel.manager` 管理 sing-box 二进制，目录布局：

```
<datadir>/
├── bin/
│   ├── current/sing-box-<os>-<arch>   # 当前激活的内核
│   └── pool/sing-box-<v>-<os>-<arch>  # 版本池（不可变）
├── etc/
│   ├── config.json                     # 当前生效配置
│   ├── config.json.bak                 # 回滚备份
│   └── secrets/                        # 本地敏感信息（加密）
└── run/
    ├── sing-box.pid
    └── agent.lock                      # 防双实例
```

- **内核下载**：Agent 配置 `agent.kernel.download_url`，下载后校验 `X-Sha256` 响应头；控制面侧的 `kernel_release` 表与 `/api/v1/kernels/*` 下载接口为 P1 待实现，MVP 可预置内核到 `bin/pool/` 或提供外部下载 URL。
- **Windows 说明**：sing-box 在 Windows 下不支持 SIGHUP 热重载，Agent 自动降级为「优雅重启」（`graceful=false`），功能等价。

## 7. 端到端启动顺序（推荐）

1. 启动控制面（自动迁移 + 建管理员，SQLite 单文件无需外部数据库）
2. 启动前端 `npm run dev`
3. 浏览器登录（初始管理员账号）→ 修改密码
4. 前端创建分组（已自动建 `default`）→ 创建节点 → 复制令牌
5. 在节点服务器配置 Agent（填令牌 + 控制面地址）→ 启动 Agent
6. Agent 自动注册上线 → 前端「节点」页确认在线
7. 配置 inbound/outbound/routing → 预览 → 下发 → 观察热更新
8. 创建代理用户 → 复制订阅链接 → 客户端导入测试

## 8. 关键端口一览

| 组件 | 端口 | 说明 |
|------|------|------|
| 控制面 HTTP | 8080 | REST API + WS + 健康检查 |
| 前端 Vite dev | 3000 | 开发服务器 |
| Agent → 控制面 | 出站 | Agent 主动连控制面，无需开放入站端口 |
| sing-box 节点 | 各 inbound 配置 | 代理数据面端口 |

## 9. 已知边界（MVP）

- **全链路 e2e 未实机验证**：本机无真实 sing-box 环境，代码已通过 `go build/vet/test` 与交叉编译验证，但「注册→下发→热重载→孤儿回收」端到端链路需在集成环境实测一次。
- **协议元数据**：前端本地枚举（`web/src/utils/protocols.ts`），后端 `GET /protocols` 下发为 P1。
- **Reality/Clash 订阅**：Reality 的 private_key→public_key 推导、完整 Clash YAML 为 P1，MVP 订阅生成普通 TLS + SNI。
- **stats 采集**：sing-box stats API 采集点为预留 stub，流量当前走心跳摘要通道。
- **自定义角色**：MVP 仅支持内置三角色。
- **前端体积**：主 chunk 约 1.2MB，建议后续 code-split。

## 10. 常见问题

| 问题 | 处理 |
|------|------|
| 首次启动后登录不上 | 检查 `config.yaml` 的 `bootstrap.admin_password` 是否已设置；若之前启动过（已有用户）则不会重建管理员 |
| 迁移报错 | 确认 `database.path` 配置正确、其父目录可写；`group` 是保留字，迁移已用引号处理 |
| Agent 连不上控制面 | 检查 `control.server_url`（Agent 主动出站，需能访问控制面）、`control.token` 是否与节点创建时返回的令牌一致 |
| 下发后 reload 失败 | 看 Agent 日志；Windows 下为重启式降级属正常，`graceful=false` 可区分 |
| crypto 密钥丢失 | 已加密的敏感数据无法解密，务必备份 `crypto.key` 或密钥文件 |
