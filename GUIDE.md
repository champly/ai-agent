# AIAgent 使用指南

本文档将帮助你准备运行环境、启动示例智能体，并通过更贴近真实场景的高级案例展示 AIAgent 的能力。

## 1. 环境准备

- 安装 Go 1.25 及以上版本（可前往 [go.dev](https://go.dev/dl/) 下载）。
- 安装 Ollama（参考 [ollama.com](https://ollama.com/download)），确保 `ollama serve` 可在本地运行。
- 拉取配置中使用的默认模型 `qwen3:8b`：

  ```bash
  ollama pull qwen3:8b
  ```

## 2. 编译项目

在仓库根目录执行：

```bash
make build
```

命令完成后会生成 `bin/agent` 与 `bin/mcp-server`。

## 3. 检查配置

打开 `config.yaml`，确认以下内容符合本地环境：

- `server.listen`：HTTP 服务监听地址。
- `ollama.host`：指向本地 Ollama 服务的 URL。
- `mcp_servers`：需要加载的 MCP Server 列表。示例中默认启用了内置文件系统工具。

## 4. 启动 Agent

使用默认配置启动：

```bash
./bin/agent --config config.yaml
```

启动后 Agent 会：

- Ping Ollama，校验模型可用性。
- 启动并注册所有启用的 MCP 工具。
- 监听配置的 HTTP 地址，等待请求。

## 5. 高级体验：让 Agent 进行自我诊断

以下示例演示如何通过连续对话让 Agent 主动调用 MCP 工具，自助分析项目状态并生成报告。

1. 请求 Agent 列出可用工具：

   ```bash
   curl -s http://localhost:8080/api/tools | jq
   ```

   你将看到内置文件工具以及配置的其他 MCP 能力（如有）。

     > 提示：建议在 `config.yaml` 中将内置文件系统 MCP 的 `--allow-root` 参数设置为项目根目录（例如 `/Users/xxx/go/src/github.com/champly/ai-agent`），便于模型读取实际源码。

1. 构造一个多步骤诊断任务：

   ```bash
   curl -X POST http://localhost:8080/api/chat \
     -H 'Content-Type: application/json' \
     -d '{
           "message": "请分析 /tmp/ai-agent-analysis/pkg/agent 目录，统计以下信息并生成报告到 /tmp/ai-agent-analysis/PROJECT_REPORT.md：1) 项目结构（目录和主要文件）2) Go 代码文件数量和总行数 3) 配置文件列表 4) 文档文件列表 5) 项目组件说明（根据目录结构）"
         }'
   ```

1. Agent 会根据任务需要自动：

- 查询工具列表，判断是否需要文件读取能力。
- 调用 `read_file` 或其他 MCP 工具获取源码片段。
- 综合上下文，输出包含目录摘要、关键组件说明与运行检查项的报告。

1. 如需继续追问，可复用相同的 `conversation_id` 进行深入分析，例如：

   ```bash
   curl -X POST http://localhost:8080/api/chat \
     -H 'Content-Type: application/json' \
     -d '{
           "message": "根据上一步的报告，给出一段 shell 脚本，自动验证配置文件中引用的 MCP Server 是否存在。",
           "conversation_id": "diag-demo"
         }'
   ```

   Agent 会基于已有上下文继续推理，并在需要时再次调用工具补充信息。

该流程体现了 AIAgent 的两个优势：

- **具备记忆的多轮对话**：一个 `conversation_id` 内的所有消息都会参与后续推理。
- **面向目标的工具使用**：模型自主决定何时调用 MCP 工具，并将工具结果融入最终答案。

## 6. 关闭服务

按 `Ctrl+C` 或发送终止信号即可触发优雅退出，Agent 会依次停止 HTTP 服务与 MCP 客户端。

## 排障建议

- 遇到 `failed to connect to Ollama` 时，确认 `ollama serve` 正在运行且 `ollama.host` 设置正确。
- 使用 `ollama list` 检查模型是否已拉取，必要时重新执行 `ollama pull`。
- MCP 工具调用失败通常与进程权限或路径设置相关，可查看 Agent 日志定位问题。
- 若需更多调试信息，可在配置中设置 `server.debug: true`，或启动时附加 `-v=3` 参数。
