# AIAgent 示例项目

AIAgent 是一个以 Go 编写的智能体演示项目，展示如何将本地运行的 Ollama 大模型与 Model Context Protocol（MCP）工具生态结合，通过 HTTP 接口输出可扩展的 Agent 能力。项目内置文件系统工具示例，并提供完备的对话与工具调度流程，方便二次开发。

## 核心特性

- 面向会话的对话编排，每个 `conversation_id` 保持完整上下文。
- 基于 Ollama 官方 Go SDK 的模型接入，支持健康检查、超时与模型切换。
- 统一的工具注册表，将本地与外部 MCP 工具无缝映射为模型可调用的函数。
- MCP 客户端管理器可按配置启动多个 stdio 工具服务器，并自动注册其能力。
- **RAG（检索增强生成）模块**，使用内存向量存储实现知识库检索增强。
- 提供 `/api/chat`、`/api/chat/rag`、`/api/rag/add`、`/api/rag/search`、`/api/tools`、`/health` 等 REST 接口，便于集成至业务系统。

## 环境依赖

- Go 1.25+（参考 `go.mod`）。
- 本地安装并启动 Ollama，且已拉取配置中默认的模型（默认 `qwen3-coder:480b-cloud`）。
- 需要拉取嵌入模型用于 RAG 功能（默认 `nomic-embed-text:latest`）。
- （可选）系统中部署其他 MCP Server，用于扩展工具能力。

## 快速上手

1. 克隆仓库并拉取依赖：

   ```bash
   go mod download
   ```

2. 编译示例：

   ```bash
   make build
   ```

3. 确保 Ollama 服务与目标模型已就绪：

   ```bash
   ollama serve
   ollama pull qwen3-coder:480b-cloud
   ollama pull nomic-embed-text:latest  # RAG 嵌入模型
   ```

4. 使用默认配置启动 Agent：

   ```bash
   ./bin/agent --config config.yaml
   ```

5. 发起一次对话请求体验工具增强推理：

   ```bash
   curl -X POST http://localhost:8080/api/chat \
     -H 'Content-Type: application/json' \
     -d '{"message":"请梳理项目目录结构并指出核心组件"}'
   ```

## RAG 功能

项目内置了 RAG（检索增强生成）模块，使用内存向量存储。启动时会自动从 `docs/rag` 目录加载所有 `.md` 文件作为知识库。

### 添加知识库文档

将你的文档以 `.md` 格式放入 `docs/rag` 目录即可，Agent 启动时会自动加载。

### RAG 接口对比

1. **不带 RAG 的普通聊天** (`/api/chat`)：

   ```bash
   curl -X POST http://localhost:8080/api/chat \
     -H 'Content-Type: application/json' \
     -d '{"message":"云巢是什么？"}'
   ```

2. **带 RAG 增强的聊天** (`/api/chat/rag`)：

   ```bash
   curl -X POST http://localhost:8080/api/chat/rag \
     -H 'Content-Type: application/json' \
     -d '{"message":"云巢是什么？"}'
   ```

3. **搜索 RAG 知识库** (`/api/rag/search`)：

   ```bash
   curl -X POST http://localhost:8080/api/rag/search \
     -H 'Content-Type: application/json' \
     -d '{"query":"云巢平台架构"}'
   ```

4. **添加文档到 RAG 知识库** (`/api/rag/add`)：

   ```bash
   curl -X POST http://localhost:8080/api/rag/add \
     -H 'Content-Type: application/json' \
     -d '{"id":"my-doc", "content":"这是我的文档内容..."}'
   ```

## 配置说明

编辑 `config.yaml` 可调整：

- `server.listen`：HTTP 服务监听地址。
- `ollama.model`：默认使用的模型名称。
- `mcp_servers`：需要启动的 MCP Server 列表，可按需新增工具来源。
- `rag.embed_model`：RAG 使用的嵌入模型（默认 `nomic-embed-text:latest`）。
- `rag.chunk_size`：文档分块大小。
- `rag.chunk_overlap`：文档分块重叠大小。
- `rag.top_k`：检索返回的结果数量。
- `rag.documents_dir`：RAG 文档目录（默认 `docs/rag`，支持 `.md` 文件）。

## 目录结构

- `cmd/agent`：HTTP 桥接模式入口程序。
- `cmd/mcp-server`：内置文件系统 MCP Server。
- `pkg/agent`：Agent 核心逻辑（对话管理、工具调度、Ollama 封装）。
- `pkg/mcpserver`：内置 MCP 工具实现。
- `pkg/rag`：RAG 模块（内存向量存储、检索增强）。
- `pkg/server`：REST API 服务实现。
- `docs/`：架构设计文档与流程说明。

更多运行机制请阅读 `docs/design.md`。
