# ============================================================
# Qavor Frontend Dockerfile (多阶段构建)
#   docker build -t qavor-web ./frontend
# ============================================================

# ---------- Stage 1: Build ----------
FROM node:22-alpine AS builder

# 启用 pnpm (Node 22 自带 corepack)
RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /build

# 先拷依赖清单，利用缓存
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# 拷源码并构建
COPY . .
RUN pnpm build

# ---------- Stage 2: Nginx ----------
FROM nginx:alpine

# 拷 nginx 配置（反代 /api → qavor-api:8080）
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 拷前端构建产物
COPY --from=builder /build/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
