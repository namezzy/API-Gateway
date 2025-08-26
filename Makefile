# API Gateway Makefile

.PHONY: help build run test clean docker-build docker-run docker-stop deps lint fmt

# 默认目标
.DEFAULT_GOAL := help

# Go相关变量
GO_VERSION := 1.21
APP_NAME := api-gateway
BINARY_NAME := gateway
BUILD_DIR := build
CONFIG_FILE := configs/config.yaml

# Docker相关变量
DOCKER_IMAGE := $(APP_NAME):latest
DOCKER_COMPOSE_FILE := docker-compose.yml

# 颜色定义
RED    := \033[31m
GREEN  := \033[32m
YELLOW := \033[33m
BLUE   := \033[34m
RESET  := \033[0m

help: ## 显示帮助信息
	@echo "$(BLUE)API Gateway 构建工具$(RESET)"
	@echo ""
	@echo "$(YELLOW)可用命令:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## 安装依赖
	@echo "$(BLUE)安装Go依赖...$(RESET)"
	go mod download
	go mod tidy

build: ## 构建二进制文件
	@echo "$(BLUE)构建 $(APP_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) cmd/gateway/main.go
	@echo "$(GREEN)构建完成: $(BUILD_DIR)/$(BINARY_NAME)$(RESET)"

build-all: ## 构建多平台二进制文件
	@echo "$(BLUE)构建多平台二进制文件...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 cmd/gateway/main.go
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 cmd/gateway/main.go
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe cmd/gateway/main.go
	@echo "$(GREEN)多平台构建完成$(RESET)"

run: build ## 运行应用
	@echo "$(BLUE)启动 $(APP_NAME)...$(RESET)"
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG_FILE)

dev: ## 开发模式运行
	@echo "$(BLUE)开发模式启动...$(RESET)"
	go run cmd/gateway/main.go -config $(CONFIG_FILE)

test: ## 运行测试
	@echo "$(BLUE)运行测试...$(RESET)"
	go test -v ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "$(BLUE)生成测试覆盖率报告...$(RESET)"
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)覆盖率报告生成: coverage.html$(RESET)"

benchmark: ## 运行基准测试
	@echo "$(BLUE)运行基准测试...$(RESET)"
	go test -bench=. -benchmem ./...

lint: ## 代码检查
	@echo "$(BLUE)运行代码检查...$(RESET)"
	golangci-lint run ./...

fmt: ## 格式化代码
	@echo "$(BLUE)格式化代码...$(RESET)"
	go fmt ./...
	goimports -w .

clean: ## 清理构建文件
	@echo "$(BLUE)清理构建文件...$(RESET)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "$(GREEN)清理完成$(RESET)"

docker-build: ## 构建Docker镜像
	@echo "$(BLUE)构建Docker镜像...$(RESET)"
	docker build -t $(DOCKER_IMAGE) .
	@echo "$(GREEN)Docker镜像构建完成: $(DOCKER_IMAGE)$(RESET)"

docker-run: ## 运行Docker容器
	@echo "$(BLUE)启动Docker容器...$(RESET)"
	docker run -p 8080:8080 -p 9090:9090 --name $(APP_NAME)-container $(DOCKER_IMAGE)

docker-stop: ## 停止Docker容器
	@echo "$(BLUE)停止Docker容器...$(RESET)"
	docker stop $(APP_NAME)-container || true
	docker rm $(APP_NAME)-container || true

compose-up: ## 启动完整的Docker Compose环境
	@echo "$(BLUE)启动Docker Compose环境...$(RESET)"
	docker-compose -f $(DOCKER_COMPOSE_FILE) up -d
	@echo "$(GREEN)Docker Compose环境启动完成$(RESET)"
	@echo "$(YELLOW)服务地址:$(RESET)"
	@echo "  API Gateway: http://localhost:8080"
	@echo "  Prometheus:  http://localhost:9091"
	@echo "  Grafana:     http://localhost:3000 (admin/admin)"

compose-down: ## 停止Docker Compose环境
	@echo "$(BLUE)停止Docker Compose环境...$(RESET)"
	docker-compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "$(GREEN)Docker Compose环境已停止$(RESET)"

compose-logs: ## 查看Docker Compose日志
	@echo "$(BLUE)查看Docker Compose日志...$(RESET)"
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f

demo: ## 运行演示脚本
	@echo "$(BLUE)运行API Gateway演示...$(RESET)"
	./scripts/demo.sh

compose-demo: ## 一键Compose演示 (含登录/指标输出)
	@echo "$(BLUE)启动Compose演示脚本...$(RESET)"
	bash scripts/compose-demo.sh

openapi-validate: ## 验证 OpenAPI 规范 (需要安装 speccy 或 swagger-cli)
	@echo "$(BLUE)验证 OpenAPI 规范...$(RESET)"
	@if command -v swagger-cli >/dev/null 2>&1; then \
		swagger-cli validate openapi/openapi.yaml; \
	elif command -v speccy >/dev/null 2>&1; then \
		speccy lint openapi/openapi.yaml; \
	else \
		echo "$(YELLOW)未找到 swagger-cli 或 speccy，跳过验证$(RESET)"; \
	fi

install-tools: ## 安装开发工具
	@echo "$(BLUE)安装开发工具...$(RESET)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

check-env: ## 检查环境
	@echo "$(BLUE)检查开发环境...$(RESET)"
	@echo "Go版本:"
	@go version
	@echo ""
	@echo "Docker版本:"
	@docker --version
	@echo ""
	@echo "Docker Compose版本:"
	@docker-compose --version

config-validate: ## 验证配置文件
	@echo "$(BLUE)验证配置文件...$(RESET)"
	@if [ -f $(CONFIG_FILE) ]; then \
		echo "$(GREEN)配置文件存在: $(CONFIG_FILE)$(RESET)"; \
	else \
		echo "$(RED)配置文件不存在: $(CONFIG_FILE)$(RESET)"; \
		exit 1; \
	fi

logs: ## 查看应用日志
	@echo "$(BLUE)查看应用日志...$(RESET)"
	@if [ -f logs/gateway.log ]; then \
		tail -f logs/gateway.log; \
	else \
		echo "$(YELLOW)日志文件不存在，请先启动应用$(RESET)"; \
	fi

all: deps fmt lint test build ## 执行完整的构建流程

release: all docker-build ## 准备发布版本
	@echo "$(GREEN)发布准备完成$(RESET)"

.PHONY: install
install: build ## 安装到系统
	@echo "$(BLUE)安装到系统...$(RESET)"
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "$(GREEN)安装完成$(RESET)"
