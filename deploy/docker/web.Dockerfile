# syntax=docker/dockerfile:1
# 前端（Web）镜像：node 构建 Vite 产物 → nginx 托管 + 反向代理。
# 构建上下文为仓库根目录（见 deploy/docker-compose.yml 的 build.context）。

# ---------- 构建阶段 ----------
FROM node:18-alpine AS build

WORKDIR /app

# 先拷贝依赖清单以利用缓存
COPY web/package.json web/package-lock.json ./
RUN npm ci

# 拷贝源码并构建（tsc + vite build，产物在 web/dist）
COPY web/ ./
RUN npm run build

# ---------- 运行阶段 ----------
FROM nginx:alpine

# 自定义 nginx 配置模板 + 运行时入口脚本（envsubst 生成最终配置，支持 PANEL_PATH 随机化）
COPY deploy/docker/nginx.conf.template /etc/nginx/templates/default.conf.template
COPY deploy/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

ENTRYPOINT ["/entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
