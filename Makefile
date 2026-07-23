.PHONY: all build run test clean migrate help

# 变量定义
APP_NAME=qavor-api
BUILD_DIR=bin
MAIN_PATH=cmd/server/main.go

# Go 参数
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get

all: clean build

## build: 编译项目
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME).exe $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME).exe"

## run: 运行项目
run:
	@echo "Running $(APP_NAME)..."
	$(GORUN) $(MAIN_PATH)

## test: 运行测试
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: 运行测试并生成覆盖率报告
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## clean: 清理构建文件
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

## deps: 下载依赖
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) tidy
	$(GOMOD) download

## migrate: 运行数据库迁移
migrate:
	@echo "Running database migrations..."
	@mysql -u root -p < scripts/migrate.sql

## fmt: 格式化代码
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: 代码静态检查
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## lint: 代码检查（需要安装 golangci-lint）
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run

## help: 显示帮助信息
help:
	@echo "可用命令:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
