# sing-box 企业级代理面板 —— Docker 一键部署

对标 s-ui 的部署体验：在任意 Linux 服务器上，一条命令部署 **面板 + 单机节点 + Agent**，得到一个可访问的面板 URL。**不预置任何节点协议**，入站/出站协议由你在面板中手动配置。

---

## 1. 一条命令部署

```bash
curl -sSL https://raw.githubusercontent.com/sle58083/yc-box/main/deploy/install.sh | bash
```

> `GITHUB_REPO` 已内置于 `install.sh` 顶部（`sle58083/yc-box`），可直接一条命令部署；
> 若需从其它仓库下载，用 `GITHUB_REPO=<owner/repo>` 覆盖。

脚本会自动完成：

1. 检查 Docker / Docker Compose（未安装时自动安装 Docker Engine 与 Compose v2 插件）；
2. 下载 `docker-compose.yml` 与 `.env` 模板；
3. 生成随机 `CRYPTO_KEY` / `JWT_SECRET` / `ADMIN_PASSWORD`；
4. 启动 `control-plane` / `web`（数据落 SQLite 单文件，无需外部数据库）；
5. 等待健康检查通过；
6. 自动在面板创建单机节点并回填 `AGENT_TOKEN`，拉起 `edge-agent`；
7. 输出访问地址与初始账号密码。

部署完成后输出形如：

```
面板地址 : http://<服务器IP>
初始账号 : admin
初始密码 : <随机生成>
配置目录 : /opt/singbox-panel
```

## 2. 环境要求

- Linux 服务器（amd64 / arm64），建议使用 root 执行安装；
- **无需预装 Docker**：脚本会**自动检测并安装 Docker Engine 与 `docker compose` 插件（Compose v2）**（需可访问 `https://get.docker.com` 与 `https://github.com`）。若已安装则直接复用、不会重复安装；
- 自动安装失败时，请按脚本提示手动安装 Docker（Docker Engine ≥ 20.10，且已安装 `docker compose` 插件）；
- 对外开放端口：`80`（面板），以及 `NODE_PORT_RANGE`（默认 `10000-10100`，用于 sing-box 入站）。

## 3. 部署后如何使用

1. 浏览器访问 `http://<服务器IP>`，用初始账号密码登录；
2. **立即修改初始密码**；
3. 在「节点管理」中确认 `singbox-local` 节点已上线（脚本已自动创建并纳管）；
4. 在「入站 / 出站 / 路由」中配置你需要的节点协议（VLESS / VMess / Trojan / SS / Hysteria2 / TUIC 等）；
5. 点击「下发配置」，Agent 会自动拉取 sing-box 内核、写入配置并启动。
   - **入站端口必须落在 `NODE_PORT_RANGE`（默认 `10000-10100`）范围内**，否则宿主机不会映射该端口。

> 若需使用 443 等端口，编辑 `.env` 中 `NODE_PORT_RANGE=443:443` 后执行
> `docker compose up -d edge-agent`（多端口可用逗号/范围，如 `10000-10100,443:443`）。

## 4. 配置与密钥管理（重要）

所有可配置项见 `deploy/.env`（首次由 `install.sh` 基于 `.env.example` 生成）：

| 变量 | 说明 |
|---|---|
| `PUBLIC_PORT` | 前端对外端口（默认 80） |
| `DATABASE_PATH` | 容器内 SQLite 数据文件路径（默认 `/opt/singbox-panel/data/panel.db`） |
| `CRYPTO_KEY` | AES-256 密钥（32 字节 hex），**生成后不可更换** |
| `JWT_SECRET` | JWT 签名密钥 |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | 初始管理员账号密码 |
| `AGENT_TOKEN` | 单机节点 Agent 令牌（脚本自动回填） |
| `NODE_PORT_RANGE` | sing-box 入站端口映射范围 |
| `IMAGE_PREFIX` / `IMAGE_TAG` | 预构建镜像仓库与标签（可选覆盖，默认见 docker-compose.yml） |
| `LOG_LEVEL` / `LOG_FORMAT` | 日志级别与格式 |

**务必备份 `.env`**：`CRYPTO_KEY` 更换后将无法解密数据库中已加密的私钥 / UUID / 令牌，等同数据丢失。

## 5. 升级 / 卸载 / 重纳管

```bash
# 升级到最新镜像并重启（保留 .env 与数据）
curl -sSL https://raw.githubusercontent.com/sle58083/yc-box/main/deploy/install.sh | bash -s -- --update

# 停止并删除容器（保留数据卷与 .env）
curl -sSL .../install.sh | bash -s -- --uninstall

# 彻底卸载（删除数据卷与 .env，不可恢复）
curl -sSL .../install.sh | bash -s -- --uninstall --purge

# 重新创建单机节点并回填 AGENT_TOKEN（例如在面板误删节点后）
curl -sSL .../install.sh | bash -s -- --reprovision
```

## 6. 从源码构建（开发者）

```bash
git clone https://github.com/sle58083/yc-box
cd yc-box
docker compose -f deploy/docker-compose.yml up -d --build
```

> 构建上下文为仓库根目录；根目录 `.dockerignore` 已排除 `web/node_modules`、`.env` 等。

## 7. 架构与端口

```
                    ┌─────────────────────────────────────────────┐
  浏览器/Agent ──►  │ web (nginx:80)                               │
                    │   /           → SPA 静态资源                  │
                    │   /api/       → control-plane:8080           │
                    │   /ws/agent   → control-plane:8080 (WS)      │
                    └──────────────┬───────────────────────────────┘
                                   │ compose 内网
                    ┌──────────────┼───────────────┬───────────────┐
                    │ control-plane│ (SQLite 数据卷) │ edge-agent    │
                    │ :8080 (内网)  │               │ ws→control    │
                    └──────────────┴───────────────┴───┬───────────┘
                                                        │ 托管 sing-box
                                             宿主机映射 NODE_PORT_RANGE
```

- 仅 `web` 对外暴露 `80`；`control-plane:8080` 仅 compose 内网互通（可选在 compose 中打开 127.0.0.1:8080 便于调试）。SQLite 数据保存在 `panel_data` 数据卷中。
- Agent 主动出站 `ws://control-plane:8080/ws/agent`，无需给 Agent 开入站端口。
- nginx 对 `/ws/` 配置了 WebSocket `Upgrade`/`Connection` 头，这是 Agent 长连接能建立的关键。

## 8. 发布镜像（GitHub Release CI）

推送 `v*` tag 会触发 `.github/workflows/release.yml`，构建三个镜像（linux/amd64 + linux/arm64）推送到 GHCR，并将 `install.sh` 附加到 Release。

**发布前需替换的占位符**：

| 位置 | 占位符 | 替换为 |
|---|---|---|
| `deploy/docker-compose.yml` | `YOUR_GITHUB_ORG`（镜像前缀默认值） | `sle58083`（已替换） |
| `deploy/install.sh` | `YOUR_GITHUB_ORG/singbox-panel`（GITHUB_REPO 默认值） | `sle58083/yc-box`（已替换） |
| 本文档与 install.sh 里的示例 URL | `<你的仓库>` | `sle58083/yc-box`（已替换） |

> 镜像前缀只需改 `docker-compose.yml` 一处默认值；`.env.example` 中的 `IMAGE_PREFIX` 为可选覆盖，默认注释。

镜像命名约定（与 `release.yml` 中 `github.repository_owner` 自动对齐）：

```
ghcr.io/<owner>/singbox-panel-control-plane:<tag>
ghcr.io/<owner>/singbox-panel-edge-agent:<tag>
ghcr.io/<owner>/singbox-panel-web:<tag>
```

## 9. 常见问题

- **Agent 显示离线**：确认 `AGENT_TOKEN` 与面板节点令牌一致；可用 `--reprovision` 重建节点。
- **入站端口不通**：面板入站端口必须落在 `NODE_PORT_RANGE`；改后需 `docker compose up -d edge-agent`。
- **健康检查失败**：`docker compose logs control-plane` 查看，多为 `CRYPTO_KEY` 格式错误（须为 64 个 hex 字符）或 SQLite 数据卷权限问题。
- **无法 pull 镜像**：确认已发布镜像且 `IMAGE_PREFIX` 正确；或改用第 6 节源码构建。
