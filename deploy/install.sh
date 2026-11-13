#!/usr/bin/env bash
#
# sing-box 企业级代理面板 —— 一键部署 / 升级 / 卸载脚本（对标 s-ui 体验）
#
# 用法（一条命令）：
#   curl -sSL https://raw.githubusercontent.com/sle58083/yc-box/main/deploy/install.sh | bash
#
# 子命令：
#   （默认）       安装/启动：生成 .env、拉起全部服务、自动纳管单机节点
#   --update       升级：重新拉取 docker-compose.yml 与镜像并重启（保留 .env 与数据）
#   --uninstall    卸载：停止并删除容器（默认保留数据卷与 .env）
#   --purge        与 --uninstall 连用：同时删除数据卷、.env 与安装目录
#   --reprovision  重新在面板创建单机节点并回填 AGENT_TOKEN
#   -h|--help      显示帮助
#
# 可用环境变量覆盖：
#   GITHUB_REPO / REF / BASE_URL   下载源（默认 raw.githubusercontent）
#   INSTALL_DIR                    安装目录（默认 /opt/singbox-panel）
#   SERVER_IP                      展示用的服务器 IP（默认自动探测）

set -Eeuo pipefail

# ------------------------------------------------------------------------------
# 可配置常量
# ------------------------------------------------------------------------------
GITHUB_REPO="${GITHUB_REPO:-sle58083/yc-box}"
REF="${REF:-main}"
BASE_URL="${BASE_URL:-https://raw.githubusercontent.com/${GITHUB_REPO}/${REF}/deploy}"
INSTALL_DIR="${INSTALL_DIR:-/opt/singbox-panel}"
NODE_NAME="${NODE_NAME:-singbox-local}"

# 解析脚本所在目录（支持 curl|bash 与本地文件两种执行方式）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)"

# 颜色输出（非交互终端自动退化为纯文本）
if [ -t 1 ]; then
  C_GREEN='\033[0;32m'; C_YELLOW='\033[0;33m'; C_RED='\033[0;31m'; C_NC='\033[0m'
else
  C_GREEN=''; C_YELLOW=''; C_RED=''; C_NC=''
fi

info()  { printf "${C_GREEN}[信息]${C_NC} %s\n" "$*"; }
warn()  { printf "${C_YELLOW}[警告]${C_NC} %s\n" "$*"; }
err()   { printf "${C_RED}[错误]${C_NC} %s\n" "$*" >&2; }
die()   { err "$*"; exit 1; }

# ------------------------------------------------------------------------------
# 基础工具
# ------------------------------------------------------------------------------

# 检测 docker compose 命令；成功返回可执行字符串，失败返回非零
detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE="docker compose"
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE="docker-compose"
    return 0
  fi
  return 1
}

# 生成 N 字节随机 hex（优先 openssl，回退 /dev/urandom + od）
rand_hex() {
  local bytes="${1:-32}"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$bytes" 2>/dev/null || true
    return
  fi
  od -An -N"$bytes" -tx1 /dev/urandom 2>/dev/null | tr -d ' \n' || true
}

# 读取 .env 中某键的值（空键返回空串）
env_get() {
  local key="$1" v
  v="$(grep -E "^${key}=" "$ENV_FILE" 2>/dev/null | tail -1 | cut -d= -f2- || true)"
  v="${v#\"}"; v="${v%\"}"
  v="${v#\'}"; v="${v%\'}"
  printf '%s' "$v"
}

# 写入 .env 中某键（存在则替换，不存在则追加）；值均为 hex，无特殊字符
env_set() {
  local key="$1" val="$2" tmp
  tmp="$(mktemp)"
  if grep -qE "^${key}=" "$ENV_FILE" 2>/dev/null; then
    sed "s|^${key}=.*|${key}=${val}|" "$ENV_FILE" > "$tmp"
  else
    cat "$ENV_FILE" > "$tmp" 2>/dev/null || true
    printf '%s=%s\n' "$key" "$val" >> "$tmp"
  fi
  mv "$tmp" "$ENV_FILE"
}

# 从 JSON 中取字段（jq 优先，python3 次之，sed 兜底仅支持 data.<field>）
json_field() {
  local json="$1" path="$2" leaf val
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$json" | jq -r "$path" 2>/dev/null || true
    return 0
  fi
  if command -v python3 >/dev/null 2>&1; then
    printf '%s' "$json" | python3 - "$path" <<'PY'
import sys, json
data = json.load(sys.stdin)
cur = data
for part in sys.argv[1].split('.'):
    if part.startswith('[') and part.endswith(']'):
        cur = cur[int(part[1:-1])]
    else:
        cur = cur[part]
if isinstance(cur, str):
    print(cur)
else:
    print(json.dumps(cur))
PY
    return 0
  fi
  leaf="${path##*.}"
  printf '%s' "$json" | sed -n 's/.*"'"$leaf"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

# 探测服务器对外 IP（优先环境变量，其次公网探测，最后网卡地址）
get_public_ip() {
  local ip=""
  [ -n "${SERVER_IP:-}" ] && { printf '%s' "$SERVER_IP"; return 0; }
  ip="$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  [ -z "$ip" ] && ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  printf '%s' "${ip:-127.0.0.1}"
}

# 轮询等待某 URL 可用
wait_http() {
  local url="$1" tries="${2:-90}" delay="${3:-2}" i
  for i in $(seq 1 "$tries"); do
    if curl -fsS --max-time 3 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done
  return 1
}

# ------------------------------------------------------------------------------
# 环境准备
# ------------------------------------------------------------------------------

check_deps() {
  command -v curl >/dev/null 2>&1 || die "未检测到 curl，请先安装（apt-get install curl / yum install curl）"
  docker version >/dev/null 2>&1 || die "未检测到 Docker 或 Docker daemon 未运行。请先安装并启动 Docker（https://docs.docker.com/engine/install/）"
  detect_compose || die "未检测到 Docker Compose。请安装 docker compose 插件（https://docs.docker.com/compose/install/）"
}

# 定位部署目录与文件（本地源码优先，否则下载）
ensure_files() {
  if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/docker-compose.yml" ] && [ -f "$SCRIPT_DIR/.env.example" ]; then
    # 本地源码模式：直接使用仓库内 deploy/ 目录（build.context=仓库根目录）
    DEPLOY_DIR="$SCRIPT_DIR"
    info "检测到本地部署文件（${DEPLOY_DIR}），使用本地源码模式"
    return 0
  fi

  DEPLOY_DIR="$INSTALL_DIR"
  mkdir -p "$DEPLOY_DIR"
  info "下载部署文件到 ${DEPLOY_DIR} ..."
  curl -fsSL --retry 3 "$BASE_URL/docker-compose.yml" -o "$DEPLOY_DIR/docker-compose.yml" \
    || die "下载 docker-compose.yml 失败（BASE_URL=${BASE_URL}）"
  # 仅当 .env 不存在时才需要模板；已存在则保留（幂等，密钥不重生成）
  if [ ! -f "$DEPLOY_DIR/.env" ]; then
    curl -fsSL --retry 3 "$BASE_URL/.env.example" -o "$DEPLOY_DIR/.env.example" \
      || die "下载 .env.example 失败（BASE_URL=${BASE_URL}）"
  fi
}

# 生成 .env（幂等：已存在则跳过，绝不覆盖密钥）
ensure_env() {
  ENV_FILE="$DEPLOY_DIR/.env"
  if [ -f "$ENV_FILE" ]; then
    info "已存在 ${ENV_FILE}，跳过生成（幂等，保留现有密钥）"
    return 0
  fi

  local template="$DEPLOY_DIR/.env.example"
  [ -f "$template" ] || die "缺少 .env.example 模板"
  cp "$template" "$ENV_FILE"

  # 自动生成强随机值（仅当模板中对应键为空）
  local dbpass crypto jwt adminpass
  dbpass="$(rand_hex 16)"
  crypto="$(rand_hex 32)"
  jwt="$(rand_hex 32)"
  adminpass="$(rand_hex 8)"

  env_set DB_PASSWORD "$dbpass"
  env_set CRYPTO_KEY "$crypto"
  env_set JWT_SECRET "$jwt"
  if [ -z "$(env_get ADMIN_PASSWORD)" ]; then
    env_set ADMIN_PASSWORD "$adminpass"
  fi
  chmod 600 "$ENV_FILE"
  info "已生成 ${ENV_FILE}（含随机 DB_PASSWORD / CRYPTO_KEY / JWT_SECRET / ADMIN_PASSWORD）"
}

# ------------------------------------------------------------------------------
# 单机节点纳管：面板登录 → 取默认分组 → 创建节点 → 回填 AGENT_TOKEN
# ------------------------------------------------------------------------------

provision_node() {
  local api public_port admin_user admin_pass login_json token groups_json group_id node_json agent_token
  public_port="$(env_get PUBLIC_PORT)"; [ -n "$public_port" ] || public_port=80
  api="http://127.0.0.1:${public_port}/api/v1"

  admin_user="$(env_get ADMIN_USERNAME)"; [ -n "$admin_user" ] || admin_user=admin
  admin_pass="$(env_get ADMIN_PASSWORD)"
  [ -n "$admin_pass" ] || { warn "ADMIN_PASSWORD 为空，跳过节点自动纳管"; return 1; }

  info "登录面板并纳管单机节点 ..."

  login_json="$(curl -fsS --max-time 10 -X POST "$api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$admin_user\",\"password\":\"$admin_pass\"}")" || return 1
  token="$(json_field "$login_json" '.data.access_token')"
  { [ -n "$token" ] && [ "$token" != "null" ]; } || { warn "登录失败：$(json_field "$login_json" '.message')"; return 1; }

  groups_json="$(curl -fsS --max-time 10 "$api/groups" -H "Authorization: Bearer $token")" || return 1
  group_id="$(json_field "$groups_json" '.data[0].id')"
  { [ -n "$group_id" ] && [ "$group_id" != "null" ]; } || { warn "未找到默认分组"; return 1; }

  node_json="$(curl -fsS --max-time 10 -X POST "$api/nodes" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $token" \
    -d "{\"name\":\"$NODE_NAME\",\"group_id\":\"$group_id\",\"address\":\"127.0.0.1\"}")" || return 1
  agent_token="$(json_field "$node_json" '.data.agent_token')"
  { [ -n "$agent_token" ] && [ "$agent_token" != "null" ]; } || { warn "创建节点失败：$(json_field "$node_json" '.message')"; return 1; }

  env_set AGENT_TOKEN "$agent_token"
  info "已创建节点 ${NODE_NAME} 并回填 AGENT_TOKEN"
  return 0
}

# ------------------------------------------------------------------------------
# 各子命令
# ------------------------------------------------------------------------------

do_install() {
  check_deps
  ensure_files
  ensure_env
  cd "$DEPLOY_DIR"

  local public_port
  public_port="$(env_get PUBLIC_PORT)"; [ -n "$public_port" ] || public_port=80

  # 1) 优先拉取预构建镜像（已发布时）；失败静默，交由本地 build 兜底
  info "拉取镜像（若已发布）..."
  $COMPOSE pull >/dev/null 2>&1 || warn "拉取预构建镜像失败，将回退到本地构建（请确认 IMAGE_PREFIX 已指向已发布的镜像仓库，或本目录存在源码）"

  # 2) 先启动核心服务（不含 edge-agent），等待健康
  info "启动核心服务（postgres/redis/control-plane/web）..."
  $COMPOSE up -d postgres redis control-plane web

  info "等待面板健康检查（http://127.0.0.1:${public_port}/healthz）..."
  if ! wait_http "http://127.0.0.1:${public_port}/healthz"; then
    warn "健康检查超时，请查看日志：$COMPOSE logs control-plane web"
  fi

  # 3) 纳管单机节点（幂等：AGENT_TOKEN 已存在则跳过）
  if [ -z "$(env_get AGENT_TOKEN)" ]; then
    if ! provision_node; then
      warn "节点自动纳管失败。可稍后在面板「节点管理」创建节点，将 agent_token 填入 ${ENV_FILE} 后重跑本脚本"
      # 填入占位 token，避免 edge-agent 因空令牌直接退出（保持运行并等待正确令牌）
      env_set AGENT_TOKEN "$(rand_hex 32)"
    fi
  else
    info "AGENT_TOKEN 已存在，跳过节点纳管"
  fi

  # 4) 拉起全部服务（含 edge-agent）
  $COMPOSE up -d

  # 5) 输出访问信息
  echo ""
  info "部署完成！"
  echo "  面板地址 : http://$(get_public_ip):${public_port}"
  echo "  初始账号 : $(env_get ADMIN_USERNAME)"
  echo "  初始密码 : $(env_get ADMIN_PASSWORD)"
  echo "  配置目录 : ${DEPLOY_DIR}"
  echo ""
  warn "请立即登录并修改初始密码；请务必备份 ${ENV_FILE}（含加密密钥，不可更换）"
  local node_range
  node_range="$(env_get NODE_PORT_RANGE)"; [ -n "$node_range" ] || node_range="10000-10100"
  echo "  节点入站端口范围 : ${node_range}（面板配置入站协议时请使用该范围内端口）"
}

do_update() {
  check_deps
  ensure_files
  ensure_env
  cd "$DEPLOY_DIR"

  info "拉取最新镜像..."
  $COMPOSE pull || warn "拉取镜像失败，将尝试本地构建"
  info "重启服务..."
  $COMPOSE up -d
  info "升级完成"
}

do_uninstall() {
  check_deps
  # 定位部署目录（本地源码优先，否则默认安装目录）；不下载文件
  if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/docker-compose.yml" ]; then
    DEPLOY_DIR="$SCRIPT_DIR"
  else
    DEPLOY_DIR="$INSTALL_DIR"
  fi
  if [ ! -f "$DEPLOY_DIR/docker-compose.yml" ]; then
    info "未发现部署文件（${DEPLOY_DIR}/docker-compose.yml），无需卸载"
    return 0
  fi
  cd "$DEPLOY_DIR"

  info "停止并删除容器..."
  $COMPOSE down || true

  if [ "${PURGE:-0}" = "1" ]; then
    warn "清除数据卷、.env 与安装目录（不可恢复）..."
    $COMPOSE down -v || true
    rm -f "$DEPLOY_DIR/.env"
    rm -f "$DEPLOY_DIR/docker-compose.yml" "$DEPLOY_DIR/.env.example"
    info "已清理 ${DEPLOY_DIR}（数据卷已删除）"
  else
    info "已停止并删除容器。数据卷与 ${DEPLOY_DIR}/.env 已保留；如需彻底清除请使用 --uninstall --purge"
  fi
}

usage() {
  cat <<'EOF'
用法:
  install.sh                安装/启动（幂等）
  install.sh --update       升级到最新镜像并重启
  install.sh --uninstall    停止并删除容器（保留数据卷与 .env）
  install.sh --uninstall --purge   彻底卸载（删除数据卷与 .env）
  install.sh --reprovision  重新创建单机节点并回填 AGENT_TOKEN
  install.sh -h|--help      显示帮助

环境变量:
  GITHUB_REPO / REF / BASE_URL   下载源（默认 raw.githubusercontent.com/sle58083/yc-box/main/deploy）
  INSTALL_DIR                    安装目录（默认 /opt/singbox-panel）
  SERVER_IP                      展示用的服务器 IP（默认自动探测）
EOF
}

do_reprovision() {
  check_deps
  ensure_files
  ensure_env
  cd "$DEPLOY_DIR"
  env_set AGENT_TOKEN ""
  if provision_node; then
    $COMPOSE up -d edge-agent
    info "节点重新纳管完成"
  else
    die "重新纳管失败，请检查控制面是否运行、管理员凭据是否正确"
  fi
}

# ------------------------------------------------------------------------------
# 入口
# ------------------------------------------------------------------------------

PURGE=0
CMD="install"

while [ $# -gt 0 ]; do
  case "$1" in
    --update)     CMD="update"; shift ;;
    --uninstall)  CMD="uninstall"; shift ;;
    --purge)      PURGE=1; shift ;;
    --reprovision) CMD="reprovision"; shift ;;
    -h|--help)    usage; exit 0 ;;
    *)            warn "未知参数：$1"; usage; exit 2 ;;
  esac
done

case "$CMD" in
  install)     do_install ;;
  update)      do_update ;;
  uninstall)   do_uninstall ;;
  reprovision) do_reprovision ;;
esac
