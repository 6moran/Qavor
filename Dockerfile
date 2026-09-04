# ============================================================
# Qavor Backend Dockerfile (多阶段构建)
#   docker build -t qavor-api .
#   CGO_ENABLED=0 → 纯静态二进制，可跑在 alpine
# ============================================================

# ---------- Stage 1: Build ----------
FROM golang:1.25-alpine AS builder

# git: go mod download 需要 (私有仓库 / 版本信息)
# ca-certificates: 调用外部 API (OpenAI / 火山引擎) 需要 TLS
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# 先拷依赖清单，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# 拷源码
COPY . .

# 构建参数：版本号注入
ARG VERSION=1.0.0
ARG BUILD_TIME

# 静态编译：CGO_ENABLED=0，产物可跑在 scratch/alpine
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags "-s -w \
        -X main.Version=${VERSION} \
        -X main.BuildTime=${BUILD_TIME}" \
      -o /out/qavor-api \
      ./cmd/server

# ---------- Stage 2: Runtime ----------
FROM alpine:3.20

# ca-certificates: TLS；tzdata: 时区；wget: 健康检查
RUN apk add --no-cache ca-certificates tzdata wget

# 非 root 用户
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

# 拷二进制
COPY --from=builder /out/qavor-api .

# 拷配置目录（config.docker.yaml 会被 compose 挂载覆盖，这里给个兜底）
COPY configs/ ./configs/

# 拷迁移脚本（容器内可手动执行）
COPY scripts/migrate.sql ./scripts/

# 数据目录：工作空间 / 日志
RUN mkdir -p data/workspaces logs && chown -R app:app /app

USER app

EXPOSE 8080

# 健康检查：/health 端点（需要后端有这个路由，没有则改成 TCP 检测）
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["./qavor-api"]
