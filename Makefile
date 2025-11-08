# AIAgent Makefile

.PHONY: all build run clean help

# 默认目标
all: build

# 构建所有二进制文件
build:
	@echo "Building AIAgent..."
	@go build -o bin/agent ./cmd/agent
	@go build -o bin/mcp-server ./cmd/mcp-server
	@echo "✓ Build complete"

# 运行 AIAgent
run: build
	@echo "Starting AIAgent..."
	./bin/agent --config config.yaml -v=3

# 清理构建产物
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "✓ Clean complete"

# 帮助信息
help:
	@echo "AIAgent Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build    - Build all binaries"
	@echo "  make run      - Build and run AIAgent"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make help     - Show this help"
