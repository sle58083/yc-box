#!/bin/sh
set -eu

# 面板路径随机化入口脚本：
#   1) 读取 PANEL_PATH（docker-compose 注入，随机 hex 或空）
#   2) 用 envsubst 从 nginx.conf.template 生成运行时 nginx 配置
#   3) 将面板路径注入 index.html，供前端 react-router basename 使用
#   4) 启动 nginx

TEMPLATE="${NGINX_TEMPLATE:-/etc/nginx/templates/default.conf.template}"
TARGET="${NGINX_TARGET:-/etc/nginx/conf.d/default.conf}"
HTML="${NGINX_HTML:-/usr/share/nginx/html/index.html}"

# 规范化：去空白、去斜杠，仅保留随机 hex 本体（如 "a1b2c3"）
PANEL_PATH_RAW="$(printf '%s' "${PANEL_PATH:-}" | tr -d '[:space:]' | tr -d '/')"

# 前端路由 basename（'/a1b2c3' 或 ''=根路径）
BASENAME=""

mkdir -p "$(dirname "$TARGET")"

if [ -n "$PANEL_PATH_RAW" ]; then
  # 隐藏入口模式：面板挂载到 /<PANEL_PATH>/，根路径返回 404
  BASENAME="/${PANEL_PATH_RAW}"
  export PANEL_PATH="/${PANEL_PATH_RAW}/"
  # 不带尾斜杠的路径（用于精确匹配重定向 /a1b2c3 -> /a1b2c3/）
  export PANEL_PATH_BARE="/${PANEL_PATH_RAW}"
  envsubst '${PANEL_PATH} ${PANEL_PATH_BARE}' < "$TEMPLATE" > "$TARGET"
  echo "[entrypoint] 面板路径已启用：${BASENAME}"
else
  # 兼容模式：PANEL_PATH 为空 → 根路径直接提供 SPA
  # 删除「根路径 404」与「不带尾斜杠重定向」两条规则（均为隐藏入口模式专用）
  export PANEL_PATH="/"
  export PANEL_PATH_BARE="__UNUSED__"
  sed -e '/location = \/ {/,/}/d' \
      -e '/location = ${PANEL_PATH_BARE} {/,/}/d' \
      "$TEMPLATE" | envsubst '${PANEL_PATH} ${PANEL_PATH_BARE}' > "$TARGET"
  echo "[entrypoint] PANEL_PATH 为空，使用根路径（兼容旧实例）"
fi

# 注入面板路径到 index.html，供前端 react-router basename 使用
if [ -f "$HTML" ]; then
  snippet="<script>window.__PANEL_PATH__ = \"${BASENAME}\";</script>"
  if grep -q '<head>' "$HTML"; then
    sed -i "s|<head>|<head>${snippet}|" "$HTML"
  else
    sed -i "1i${snippet}" "$HTML"
  fi
fi

exec "$@"
