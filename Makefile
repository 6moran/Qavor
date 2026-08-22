# ============================================================
# Qavor Makefile
# 跨平台兼容：Windows (Git Bash / MinGW / MSYS / Cygwin) / Linux / macOS
# 用法示例：
#   make                       # 等价于 make build
#   make build VERSION=1.2.3   # 指定版本号（注入二进制）
#   make run CONFIG=configs/config.yaml
#   make migrate PGPASSWORD=xxx
#   make frontend-build NPM_CMD=npm
# ============================================================

# 强制使用 POSIX shell 解释 recipe，避免某些 Windows make 实现（如 GnuWin32）
# 默认调用 cmd.exe 导致 mkdir -p / rm -rf / grep 等 sh 语法报错。
SHELL := /bin/sh

# ---------- 目标声明 ----------
.PHONY: all build run test test-coverage clean deps migrate fmt vet lint check \
        frontend-install frontend-build frontend-dev frontend-lint help

# ---------- 基础变量 ----------
APP_NAME    := qavor-api
BUILD_DIR   := bin
MAIN_PATH   := cmd/server/main.go
CONFIG      ?= configs/config.yaml
VERSION     ?= 1.0.0

# Windows / MinGW / MSYS / Cygwin 下追加 .exe 后缀，其余平台不加
BIN_EXT :=
ifeq ($(OS),Windows_NT)
  BIN_EXT := .exe
endif
ifneq (,$(findstring MINGW,$(shell uname -s 2>/dev/null)))
  BIN_EXT := .exe
endif
ifneq (,$(findstring MSYS,$(shell uname -s 2>/dev/null)))
  BIN_EXT := .exe
endif
ifneq (,$(findstring CYGWIN,$(shell uname -s 2>/dev/null)))
  BIN_EXT := .exe
endif

# ---------- Go 命令 ----------
GOCMD   := go
GOBUILD := $(GOCMD) build
GORUN   := $(GOCMD) run
GOTEST  := $(GOCMD) test
GOMOD   := $(GOCMD) mod
RM      := rm -rf

# 构建时注入的版本信息（需 cmd/server/main.go 中定义对应变量）
BUILD_TIME := $(shell date '+%Y-%m-%d %H:%M:%S')
GO_VERSION := $(shell $(GOCMD) version | awk '{print $$3}')
LDFLAGS    := -X 'main.Version=$(VERSION)' \
              -X 'main.BuildTime=$(BUILD_TIME)' \
              -X 'main.GoVersion=$(GO_VERSION)'

# ---------- 前端变量 ----------
FRONTEND_DIR := frontend
NPM_CMD      ?= pnpm   # 可用 NPM_CMD=npm 覆盖

# ---------- PostgreSQL 迁移连接参数 ----------
# 覆盖方式：make migrate PGPASSWORD=xxx 或导出同名环境变量
PGHOST     ?= localhost
PGPORT     ?= 5432
PGUSER     ?= postgres
PGDATABASE ?= qavor
PGSSLMODE  ?= disable
MIGRATE_SQL := scripts/migrate.sql

# 默认目标：编译
all: build

## build: 编译项目，输出到 $(BUILD_DIR)/
build:
	@echo "Building $(APP_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)$(BIN_EXT) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)$(BIN_EXT)"

## run: 运行项目（可用 CONFIG=xxx 指定配置文件，默认 configs/config.yaml）
run:
	@echo "Running $(APP_NAME) with config $(CONFIG)..."
	$(GORUN) $(MAIN_PATH)

## test: 运行全部 Go 测试
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: 运行测试并生成 HTML 覆盖率报告 (coverage.html)
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: 清理构建与覆盖率产物
clean:
	@echo "Cleaning..."
	$(RM) $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

## deps: 整理并下载 Go 依赖
deps:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) download

## migrate: 执行 PostgreSQL 迁移脚本 (scripts/migrate.sql)
migrate:
	@echo "Migrating PostgreSQL: $(PGDATABASE)@$(PGHOST):$(PGPORT) (sslmode=$(PGSSLMODE))"
	PGPASSWORD="$(PGPASSWORD)" psql \
	  "host=$(PGHOST) port=$(PGPORT) user=$(PGUSER) dbname=$(PGDATABASE) sslmode=$(PGSSLMODE)" \
	  -f $(MIGRATE_SQL)
	@echo "Migration complete"

## fmt: 格式化 Go 代码
fmt:
	@echo "Formatting Go code..."
	$(GOCMD) fmt ./...

## vet: Go 静态检查
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## lint: golangci-lint 检查（需自行安装）
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run

## check: 代码格式 + 静态检查（不含需外部安装的 lint）
check: fmt vet

## frontend-install: 安装前端依赖
frontend-install:
	@echo "Installing frontend deps..."
	cd $(FRONTEND_DIR) && $(NPM_CMD) install

## frontend-build: 构建前端静态资源
frontend-build:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && $(NPM_CMD) build

## frontend-dev: 启动前端开发服务器
frontend-dev:
	@echo "Starting frontend dev server..."
	cd $(FRONTEND_DIR) && $(NPM_CMD) dev

## frontend-lint: 前端 lint
frontend-lint:
	@echo "Linting frontend..."
	cd $(FRONTEND_DIR) && $(NPM_CMD) lint

## help: 显示可用目标
help:
	@echo "可用命令 (make <target>):"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{ printf "  %-18s %s\n", $$1, $$2 }'
