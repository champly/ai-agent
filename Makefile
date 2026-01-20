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

# 导入 RAG 文档（需要先启动 agent）
rag-import:
	@echo "Importing RAG documents from docs/rag..."
	@curl -s -X POST http://localhost:8080/api/rag/import \
		-H "Content-Type: application/json" \
		-d '{"dir": "docs/rag"}' | jq .
	@echo "✓ RAG import complete"

# 帮助信息
help:
	@echo "AIAgent Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build      - Build all binaries"
	@echo "  make run        - Build and run AIAgent"
	@echo "  make rag-import - Import RAG documents from docs/rag (requires running agent)"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make help       - Show this help"
