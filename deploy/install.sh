#!/usr/bin/env bash
#
# sing-box 企业级代理面板 —— 一键部署 / 升级 / 卸载脚本（对标 s-ui 体验）
# 未检测到 Docker 时，会自动安装 Docker Engine 与 docker compose 插件（幂等，已安装则跳过）。
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
# 注意：空/非法 JSON 直接返回空串，绝不抛异常中断脚本（修复 JSONDecodeError 卡死）。
json_field() {
  local json="$1" path="$2" leaf val
  [ -n "$json" ] || { printf ''; return 0; }
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$json" | jq -r "$path" 2>/dev/null || true
    return 0
  fi
  if command -v python3 >/dev/null 2>&1; then
    # 注意：不能用 heredoc 传代码（会与管道 stdin 冲突），改用 -c 传脚本，
    # JSON 经 stdin 传入，路径经 argv 传入。
    printf '%s' "$json" | python3 -c '
import sys, json, re
raw = sys.stdin.read().strip()
if not raw:
    sys.exit(0)
try:
    data = json.loads(raw)
except Exception:
    sys.exit(0)
cur = data
for part in sys.argv[1].split("."):
    if not part:
        continue
    # 支持 key[index] 形式（如 data[0]）与 [index] 形式
    m = re.match(r"^([^\[\]]*)(\[[0-9]+\])?$", part)
    if not m:
        sys.exit(0)
    key, idx = m.group(1), m.group(2)
    if key:
        if not isinstance(cur, dict) or key not in cur:
            sys.exit(0)
        cur = cur[key]
    if idx:
        i = int(idx[1:-1])
        if not isinstance(cur, list) or i >= len(cur):
            sys.exit(0)
        cur = cur[i]
if isinstance(cur, str):
    print(cur)
else:
    print(json.dumps(cur))
' "$path"
    return 0
  fi
  leaf="${path##*.}"
  printf '%s' "$json" | sed -n 's/.*"'"$leaf"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

# 检测端口是否被占用（ss 优先，netstat/lsof 兜底；精确匹配整端口，避免 8080 误判 80808）
is_port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | awk '{print $4}' | grep -E "[:.]${port}[[:space:]]*$" >/dev/null 2>&1 && return 0
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -lnt 2>/dev/null | awk '{print $4}' | grep -E "[:.]${port}[[:space:]]*$" >/dev/null 2>&1 && return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 && return 0
  fi
  return 1
}

# 在范围内挑选一个空闲端口（优先返回原端口；被占用则在 40001-60000 随机挑选，尝试 N 次）
pick_port() {
  local preferred="$1" attempt port
  if ! is_port_in_use "$preferred"; then
    printf '%s' "$preferred"; return 0
  fi
  for attempt in $(seq 1 30); do
    port=$(( RANDOM % 20000 + 40001 ))   # 40001-60000 随机
    if ! is_port_in_use "$port"; then
      printf '%s' "$port"; return 0
    fi
  done
  printf '%s' "$preferred"   # 实在找不到空闲端口，回退原端口（由 docker 报错提示）
  return 1
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

# ------------------------------------------------------------------------------
# Docker 环境保障：检测 → 自动安装 → 启动 daemon → 安装 compose 插件 → 验证
# ------------------------------------------------------------------------------

# 从 /etc/os-release 读取单个键值（去掉首尾引号）
os_release_get() {
  local key="$1" val
  val="$(sed -n "s/^${key}=//p" /etc/os-release 2>/dev/null | head -1 || true)"
  val="${val#\"}"; val="${val%\"}"
  val="${val#\'}"; val="${val%\'}"
  printf '%s' "$val"
}

# 识别发行版，写入全局变量 DISTRO_ID / DISTRO_LIKE / DISTRO_NAME
detect_distro() {
  DISTRO_ID=""; DISTRO_LIKE=""; DISTRO_NAME="Linux"
  [ -r /etc/os-release ] || return 0
  DISTRO_ID="$(os_release_get ID)"
  DISTRO_LIKE="$(os_release_get ID_LIKE)"
  DISTRO_NAME="$(os_release_get PRETTY_NAME)"
  [ -n "$DISTRO_NAME" ] || DISTRO_NAME="$(os_release_get NAME)"
  [ -n "$DISTRO_NAME" ] || DISTRO_NAME="Linux"
}

# 启动 docker daemon（优先 systemd，回退 sysvinit）
start_docker() {
  if command -v systemctl >/dev/null 2>&1; then
    if systemctl enable --now docker >/dev/null 2>&1; then
      return 0
    fi
  fi
  if command -v service >/dev/null 2>&1; then
    if service docker start >/dev/null 2>&1; then
      return 0
    fi
  fi
  return 1
}

# 下载并执行 get.docker.com 官方脚本安装 Docker
install_docker() {
  local tmp
  tmp="$(mktemp)"
  info "下载 Docker 官方安装脚本（https://get.docker.com）..."
  if ! curl -fsSL --retry 3 https://get.docker.com -o "$tmp"; then
    rm -f "$tmp"
    err "下载 get.docker.com 失败（需可访问 https://get.docker.com）"
    return 1
  fi

  info "执行 Docker 官方安装脚本（约 1~3 分钟，请耐心等待）..."
  if ! sh "$tmp"; then
    rm -f "$tmp"
    err "Docker 官方安装脚本执行失败"
    return 1
  fi
  rm -f "$tmp"
  return 0
}

# 下载并安装 docker compose 插件（Compose v2）
install_compose_plugin() {
  local arch latest url tmp dir
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)  arch="x86_64" ;;
    aarch64|arm64) arch="aarch64" ;;
    armv7l)        arch="armv7" ;;
    *)             return 1 ;;
  esac

  latest="$(curl -fsSL --max-time 10 https://api.github.com/repos/docker/compose/releases/latest 2>/dev/null \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1 || true)"
  [ -n "$latest" ] || latest="v2.24.7"

  if [ "$(id -u)" = "0" ]; then
    dir="/usr/local/lib/docker/cli-plugins"
  else
    dir="${DOCKER_CONFIG:-$HOME/.docker}/cli-plugins"
  fi
  mkdir -p "$dir" || return 1

  tmp="$(mktemp)"
  url="https://github.com/docker/compose/releases/download/${latest}/docker-compose-linux-${arch}"
  info "下载 docker compose 插件（${latest}）..."
  if ! curl -fsSL --retry 3 "$url" -o "$tmp"; then
    rm -f "$tmp"
    err "下载 docker compose 插件失败（${url}）"
    return 1
  fi

  if command -v install >/dev/null 2>&1; then
    install -m 0755 "$tmp" "$dir/docker-compose" || { rm -f "$tmp"; return 1; }
  else
    cp "$tmp" "$dir/docker-compose" && chmod 0755 "$dir/docker-compose" || { rm -f "$tmp"; return 1; }
  fi
  rm -f "$tmp"
  return 0
}

# Docker 自动安装失败时的手动指引（发行版感知）
docker_install_hint() {
  local id="${DISTRO_ID:-}" like="${DISTRO_LIKE:-}"
  err "Docker 自动安装失败。请手动安装并启动 Docker 后重试本脚本："
  case "$id $like" in
    *debian*|*ubuntu*)
      err "  Debian/Ubuntu: apt-get update && apt-get install -y docker.io docker-compose-plugin && systemctl enable --now docker" ;;
    *centos*|*rhel*|*fedora*|*rocky*|*almalinux*)
      err "  RHEL/CentOS/Fedora: yum install -y docker-ce docker-compose-plugin && systemctl enable --now docker" ;;
    *)
      err "  通用方式: curl -fsSL https://get.docker.com | sh && systemctl enable --now docker" ;;
  esac
  err "  官方文档: https://docs.docker.com/engine/install/"
  exit 1
}

# 确保 Docker 可用（幂等：已安装且 daemon 运行则跳过；否则自动安装并启动）
ensure_docker() {
  local docker_err=""
  if docker version >/dev/null 2>&1; then
    info "检测到 Docker 且 daemon 运行中"
    return 0
  fi

  # 已安装但 daemon 未运行（或当前用户无权限）的情况
  if command -v docker >/dev/null 2>&1; then
    docker_err="$(docker version 2>&1 || true)"
    if printf '%s' "$docker_err" | grep -qi "permission denied"; then
      die "当前用户无权限访问 Docker。请以 root 运行本脚本，或将当前用户加入 docker 组后重新登录"
    fi
    warn "Docker 已安装但 daemon 未运行，尝试启动 ..."
    if start_docker && docker version >/dev/null 2>&1; then
      info "Docker daemon 启动成功"
      return 0
    fi
    warn "Docker daemon 启动失败，尝试重新安装 Docker ..."
  fi

  detect_distro
  info "未检测到可用的 Docker，将在「${DISTRO_NAME}」上自动安装 Docker ..."

  if ! install_docker; then
    docker_install_hint
  fi

  info "Docker 安装完成，启动 daemon ..."
  start_docker || warn "无法自动启动 Docker daemon，将尝试继续验证"

  if ! docker version >/dev/null 2>&1; then
    docker_install_hint
  fi
  info "Docker 安装并启动成功"
}

# 确保 docker compose 可用（幂等：已有则跳过；缺失则安装 Compose v2 插件）
ensure_compose() {
  if detect_compose; then
    return 0
  fi
  info "未检测到 docker compose，尝试自动安装 Compose v2 插件 ..."
  if install_compose_plugin && detect_compose; then
    info "docker compose 插件安装成功"
    return 0
  fi
  die "未检测到 Docker Compose。请安装 docker compose 插件（https://docs.docker.com/compose/install/）"
}

# ------------------------------------------------------------------------------
# Swap 保障：小内存机器（< 1.5GB 且无有效 swap）自动创建 1GB swap 文件，避免 OOM
# ------------------------------------------------------------------------------

# 确保存在本脚本创建的 swap：小内存机器创建 → mkswap → swapon → 写入 /etc/fstab。
# 幂等：已有系统 swap 或本脚本 swap 已启用时跳过；内存 >= 1536MB 时跳过。
ensure_swap() {
  local mem_mb swapfile="${INSTALL_DIR}/swapfile"
  mem_mb="$(awk '/MemTotal:/{printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo 0)"

  # 内存 >= 1536MB 时无需额外 swap
  if [ "${mem_mb:-0}" -ge 1536 ]; then
    info "内存充足（${mem_mb}MB），无需自动创建 swap"
    return 0
  fi

  # 已有有效 swap（含系统既有 swap，/proc/swaps 首行为表头）时不再重复创建
  if [ "$(awk 'END{print NR}' /proc/swaps 2>/dev/null || echo 1)" -gt 1 ]; then
    info "系统已有有效 swap，跳过自动创建"
    return 0
  fi

  warn "内存仅 ${mem_mb}MB 且无有效 swap，自动创建 1GB swap 文件（${swapfile}）以降低 OOM 风险 ..."
  mkdir -p "$INSTALL_DIR" || { warn "创建安装目录失败，跳过 swap"; return 0; }

  # 优先 fallocate（快速分配），回退 dd（兼容不支持 fallocate 的文件系统）
  if [ ! -s "$swapfile" ]; then
    if command -v fallocate >/dev/null 2>&1; then
      fallocate -l 1G "$swapfile" 2>/dev/null || rm -f "$swapfile"
    fi
    if [ ! -s "$swapfile" ]; then
      dd if=/dev/zero of="$swapfile" bs=1M count=1024 status=none 2>/dev/null \
        || { rm -f "$swapfile"; warn "创建 swap 文件失败，跳过 swap"; return 0; }
    fi
  fi

  chmod 600 "$swapfile" 2>/dev/null || true
  mkswap "$swapfile" >/dev/null 2>&1 || { warn "mkswap 失败，跳过 swap"; return 0; }
  swapon "$swapfile" 2>/dev/null || { warn "swapon 失败，跳过 swap"; return 0; }

  # 写入 /etc/fstab（带标记，purge 时按标记识别并清理）
  if ! grep -qF "$swapfile" /etc/fstab 2>/dev/null; then
    if [ "$(id -u)" = "0" ]; then
      printf '\n# yc-box-swap: 由 install.sh 自动创建，purge 时自动清理\n%s none swap sw 0 0\n' "$swapfile" >> /etc/fstab 2>/dev/null \
        || warn "写入 /etc/fstab 失败（重启后 swap 不会自动挂载）"
    else
      warn "非 root，跳过写入 /etc/fstab（重启后 swap 不会自动挂载）"
    fi
  fi

  info "swap 创建并启用完成（1GB @ ${swapfile}）"
}

# 清理本脚本创建的 swap（仅按 # yc-box-swap 标记匹配 fstab 行，不回滚系统既有 swap）。
# 供 --uninstall --purge 调用。
cleanup_swap() {
  local swapfile="${INSTALL_DIR}/swapfile"
  [ -f /etc/fstab ] || { rm -f "$swapfile" 2>/dev/null; return 0; }

  # 仅在存在本脚本标记时处理，避免误删系统既有 swap
  if grep -q '# yc-box-swap' /etc/fstab 2>/dev/null; then
    # 提取标记行下一行（挂载行）的设备路径，即本脚本写入的 swap 文件路径
    swapfile="$(awk '/# yc-box-swap/{getline; print $1; exit}' /etc/fstab 2>/dev/null || true)"
    [ -n "$swapfile" ] || swapfile="${INSTALL_DIR}/swapfile"

    # 若该文件当前已启用，先关闭（忽略未启用/关闭失败）
    if awk 'NR>1{print $1}' /proc/swaps 2>/dev/null | grep -qFx "$swapfile"; then
      swapoff "$swapfile" 2>/dev/null || warn "swapoff 失败：${swapfile}"
    fi

    # 移除标记行及其紧邻的挂载行
    if [ "$(id -u)" = "0" ]; then
      sed -i '/# yc-box-swap/,+1d' /etc/fstab 2>/dev/null || warn "更新 /etc/fstab 失败"
    else
      warn "非 root，跳过更新 /etc/fstab（请手动移除 # yc-box-swap 相关行）"
    fi
  fi

  # 删除本脚本创建的 swap 文件（幂等）
  if [ -f "$swapfile" ]; then
    rm -f "$swapfile" 2>/dev/null && info "已删除本脚本创建的 swap 文件（${swapfile}）"
  fi
}

# ------------------------------------------------------------------------------
# 防火墙保障：自动放行面板端口 + 节点入站端口范围（ufw / firewalld），
# 使节点开箱即用，无需用户手动开放端口。
# ------------------------------------------------------------------------------

# 解析节点入站端口范围（NODE_PORT_RANGE），输出形如 "10000-10100" 或 "10000"。
# 支持 "10000-10100"、"443:443"、"10000-10100,443" 等；取端口段的首尾用于放行。
node_port_span() {
  local range="${1:-10000-10100}" part first last
  # 去掉 "443:443" 中的冒号宿主侧，只保留容器内端口；多个逗号段取第一个
  part="${range%%,*}"
  part="${part##*:}"
  first="${part%%-*}"
  last="${part##*-}"
  [ -n "$last" ] || last="$first"
  printf '%s %s' "$first" "$last"
}

# 放行端口（tcp + udp）。firewalld 用 --permanent 持久化；ufw 直接 allow。
open_firewall() {
  local public_port="${1:-80}" node_range="${2:-10000-10100}" first last
  read -r first last <<EOF
$(node_port_span "$node_range")
EOF

  # firewalld
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    info "检测到 firewalld，放行面板端口 ${public_port}/tcp 与节点端口 ${first}-${last}（tcp+udp）..."
    firewall-cmd --permanent --add-port="${public_port}/tcp" >/dev/null 2>&1 || true
    if [ "$first" = "$last" ]; then
      firewall-cmd --permanent --add-port="${first}/tcp" >/dev/null 2>&1 || true
      firewall-cmd --permanent --add-port="${first}/udp" >/dev/null 2>&1 || true
    else
      firewall-cmd --permanent --add-port="${first}-${last}/tcp" >/dev/null 2>&1 || true
      firewall-cmd --permanent --add-port="${first}-${last}/udp" >/dev/null 2>&1 || true
    fi
    firewall-cmd --reload >/dev/null 2>&1 || true
    return 0
  fi

  # ufw
  if command -v ufw >/dev/null 2>&1; then
    info "检测到 ufw，放行面板端口 ${public_port}/tcp 与节点端口 ${first}-${last}（tcp+udp）..."
    ufw allow "${public_port}/tcp" >/dev/null 2>&1 || true
    if [ "$first" = "$last" ]; then
      ufw allow "${first}/tcp" >/dev/null 2>&1 || true
      ufw allow "${first}/udp" >/dev/null 2>&1 || true
    else
      ufw allow "${first}:${last}/tcp" >/dev/null 2>&1 || true
      ufw allow "${first}:${last}/udp" >/dev/null 2>&1 || true
    fi
    return 0
  fi

  # 无 firewalld/ufw：不拦截（i ptables 裸规则或已禁用防火墙），提示用户自行确认
  warn "未检测到 ufw/firewalld，跳过防火墙放行；请确认服务器防火墙已开放面板端口 ${public_port} 与节点端口 ${first}-${last}"
  return 0
}

check_deps() {
  command -v curl >/dev/null 2>&1 || die "未检测到 curl，请先安装（apt-get install curl / yum install curl）"
  ensure_docker
  ensure_compose
}

# 镜像拉取失败时，依次尝试国内加速源（ghcr.io → 国内镜像加速前缀）
# 通过改写 IMAGE_PREFIX 环境变量实现，不改动 docker-compose.yml。
pull_with_mirror() {
  local prefix tag services mirror mirrors resolved_prefix i service
  prefix="$(env_get IMAGE_PREFIX)"; [ -n "$prefix" ] || prefix="ghcr.io/sle58083"
  tag="$(env_get IMAGE_TAG)"; [ -n "$tag" ] || tag="latest"

  # 仅当镜像前缀指向 ghcr.io 时才尝试加速（自建仓库无需加速）
  case "$prefix" in
    ghcr.io/*) : ;;
    *) warn "IMAGE_PREFIX 非 ghcr.io，跳过国内加速源回退"; return 1 ;;
  esac

  # 国内 ghcr 加速镜像前缀（ghcr.io/<owner> → <mirror>/<owner>）
  mirrors="ghcr.dockerproxy.com ghcr.nju.edu.cn ghcr.io.dockerpull.org"

  # 读取 compose 中定义的三个服务名
  services="control-plane web edge-agent"

  for mirror in $mirrors; do
    resolved_prefix="$(printf '%s' "$prefix" | sed "s|^ghcr.io/|${mirror}/|")"
    info "尝试国内加速源：${resolved_prefix} ..."
    for service in $services; do
      if ! docker pull "${resolved_prefix}/singbox-panel-${service}:${tag}" >/dev/null 2>&1; then
        break
      fi
    done
    # 三个服务都拉取成功
    if [ "$service" = "edge-agent" ]; then
      info "国内加速源 ${mirror} 拉取成功，重打标签为原镜像名"
      for service in $services; do
        docker tag "${resolved_prefix}/singbox-panel-${service}:${tag}" "${prefix}/singbox-panel-${service}:${tag}" >/dev/null 2>&1 || true
      done
      return 0
    fi
  done

  warn "所有国内加速源均失败"
  return 1
}

# 定位部署目录与文件（本地源码优先，否则下载）
# 设置全局 LOCAL_SRC=1 表示本地源码模式（应本地 build 而非 pull 预构建镜像）。
ensure_files() {
  if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/docker-compose.yml" ] && [ -f "$SCRIPT_DIR/.env.example" ]; then
    # 本地源码模式：直接使用仓库内 deploy/ 目录（build.context=仓库根目录）
    DEPLOY_DIR="$SCRIPT_DIR"
    LOCAL_SRC=1
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
  local crypto jwt adminpass
  crypto="$(rand_hex 32)"
  jwt="$(rand_hex 32)"
  adminpass="$(rand_hex 8)"

  env_set CRYPTO_KEY "$crypto"
  env_set JWT_SECRET "$jwt"
  if [ -z "$(env_get ADMIN_PASSWORD)" ]; then
    env_set ADMIN_PASSWORD "$adminpass"
  fi
  # 面板路径随机化：首次部署生成 8 字节随机 hex 隐藏面板入口；已存在则保留（幂等）
  if [ -z "$(env_get PANEL_PATH)" ]; then
    env_set PANEL_PATH "$(rand_hex 8)"
  fi
  chmod 600 "$ENV_FILE"
  info "已生成 ${ENV_FILE}（含随机 CRYPTO_KEY / JWT_SECRET / ADMIN_PASSWORD / PANEL_PATH）"
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

  # 登录重试：control-plane 刚 healthy 时 admin seed / API 可能尚未完全就绪，
  # 轮询重试（最多 30 次 × 2s），避免首启时序导致登录失败。
  local i
  token=""
  for i in $(seq 1 30); do
    login_json="$(curl -fsS --max-time 10 -X POST "$api/auth/login" \
      -H 'Content-Type: application/json' \
      -d "{\"username\":\"$admin_user\",\"password\":\"$admin_pass\"}" 2>/dev/null)" || login_json=""
    token="$(json_field "$login_json" '.data.access_token')"
    if [ -n "$token" ] && [ "$token" != "null" ]; then
      break
    fi
    sleep 2
  done
  { [ -n "$token" ] && [ "$token" != "null" ]; } || { warn "登录失败（已重试）：$(json_field "$login_json" '.message')"; return 1; }

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
  ensure_swap
  check_deps
  ensure_files
  ensure_env
  cd "$DEPLOY_DIR"

  local public_port resolved_port panel_path
  public_port="$(env_get PUBLIC_PORT)"; [ -n "$public_port" ] || public_port=80

  # 端口冲突检测：PUBLIC_PORT 被占用时自动迁移到空闲端口并回写 .env（对齐 s-ui 体验）
  if is_port_in_use "$public_port"; then
    resolved_port="$(pick_port "$public_port")"
    if [ "$resolved_port" != "$public_port" ]; then
      warn "端口 ${public_port} 已被占用，自动迁移面板端口为 ${resolved_port}"
      env_set PUBLIC_PORT "$resolved_port"
      public_port="$resolved_port"
    else
      warn "端口 ${public_port} 已被占用，且未找到空闲端口；请手动修改 ${ENV_FILE} 中的 PUBLIC_PORT"
    fi
  fi

  # 防火墙放行：面板端口 + 节点入站端口范围（ufw/firewalld），使节点开箱即用
  open_firewall "$public_port" "$(env_get NODE_PORT_RANGE)"

  # 1) 镜像策略：
  #    - 本地源码模式（LOCAL_SRC=1）：直接 build（当前源码），绝不 pull，避免拉到 GHCR 旧镜像覆盖新代码
  #    - 下载模式：优先 pull 预构建镜像；失败依次尝试国内加速源，最后回退本地 build
  if [ "${LOCAL_SRC:-0}" = "1" ]; then
    info "本地源码模式：直接构建镜像（跳过拉取预构建镜像）..."
    $COMPOSE build >/dev/null 2>&1 || warn "本地构建镜像失败，请检查源码与构建上下文"
  else
    info "拉取镜像（若已发布）..."
    if ! $COMPOSE pull >/dev/null 2>&1; then
      if ! pull_with_mirror; then
        warn "拉取预构建镜像失败（含国内加速源），将回退到本地构建（请确认 IMAGE_PREFIX 已指向已发布的镜像仓库，或本目录存在源码）"
      fi
    fi
  fi

  # 2) 先启动核心服务（不含 edge-agent），等待健康
  info "启动核心服务（control-plane/web）..."
  $COMPOSE up -d control-plane web

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
  panel_path="$(env_get PANEL_PATH)"
  if [ -n "$panel_path" ]; then
    echo "  面板地址 : http://$(get_public_ip):${public_port}/${panel_path}/"
    echo "  （已启用面板路径随机化，请收藏完整地址；根路径访问会返回 404）"
  else
    echo "  面板地址 : http://$(get_public_ip):${public_port}"
  fi
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
  LOCAL_SRC=0
  if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/docker-compose.yml" ]; then
    DEPLOY_DIR="$SCRIPT_DIR"
    LOCAL_SRC=1
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
    cleanup_swap
    rm -f "$DEPLOY_DIR/.env"
    # 仅当走下载模式（DEPLOY_DIR=INSTALL_DIR）时才删除下载的 compose/env 模板；
    # 本地源码模式下 docker-compose.yml/.env.example 是仓库源码，不得删除。
    if [ "${LOCAL_SRC:-0}" != "1" ]; then
      rm -f "$DEPLOY_DIR/docker-compose.yml" "$DEPLOY_DIR/.env.example"
    fi
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
