package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/ollama/ollama/api"
	"k8s.io/klog/v2"

	"github.com/champly/ai-agent/pkg/config"
	"github.com/champly/ai-agent/pkg/ollama"
	"github.com/champly/ai-agent/pkg/rag"
)

// Agent AI 代理
type Agent struct {
	cfg    *config.Config
	ollama *ollama.Client

	// 对话管理
	conversations sync.Map // map[string]*Conversation

	// 工具管理
	toolRegistry *ToolRegistry

	// 外部 MCP 客户端管理器
	mcpClient *MCPClient

	// RAG 模块
	rag *rag.RAG
}

// New 创建 AI 代理
func New(cfg *config.Config) (*Agent, error) {
	agent := &Agent{
		cfg:          cfg,
		toolRegistry: NewToolRegistry(),
	}

	// 初始化 Ollama 客户端
	client, err := ollama.NewClient(
		cfg.Ollama.Host,
		cfg.Ollama.Model,
		cfg.Ollama.Timeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}
	agent.ollama = client

	// 初始化 RAG 模块
	ragCfg := &rag.Config{
		EmbedModel:   cfg.RAG.EmbedModel,
		ChunkSize:    cfg.RAG.ChunkSize,
		ChunkOverlap: cfg.RAG.ChunkOverlap,
	}
	agent.rag = rag.New(ragCfg, func(ctx context.Context, text string) ([]float32, error) {
		return client.Embed(ctx, cfg.RAG.EmbedModel, text)
	})

	klog.InfoS("Ollama client initialized",
		"host", cfg.Ollama.Host,
		"model", cfg.Ollama.Model)
	klog.InfoS("RAG module initialized",
		"embedModel", cfg.RAG.EmbedModel,
		"chunkSize", cfg.RAG.ChunkSize)

	return agent, nil
}

// Start 启动代理
func (a *Agent) Start(ctx context.Context) error {
	klog.InfoS("Starting AIAgent",
		"name", a.cfg.Server.Name,
		"version", a.cfg.Server.Version)

	// 检查 Ollama 连接
	if err := a.ollama.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	klog.InfoS("Successfully connected to Ollama", "host", a.cfg.Ollama.Host)

	// 启动外部 MCP 客户端管理器
	if len(a.cfg.MCPServers) > 0 {
		a.mcpClient = NewMCPClient(a.cfg.MCPServers)
		if err := a.mcpClient.Start(ctx); err != nil {
			return fmt.Errorf("failed to start MCP manager: %w", err)
		}

		// 注册外部 MCP 工具
		externalTools := a.mcpClient.GetAllTools()
		for _, tool := range externalTools {
			a.toolRegistry.Register(tool)
		}
		klog.InfoS("External MCP tools registered", "count", len(externalTools))
	}

	totalTools := a.toolRegistry.Count()
	klog.InfoS("AIAgent started successfully", "totalTools", totalTools)

	return nil
}

// Stop 停止代理
func (a *Agent) Stop(ctx context.Context) error {
	klog.InfoS("Stopping AIAgent")

	// 停止 MCP 管理器
	if a.mcpClient != nil {
		if err := a.mcpClient.Stop(ctx); err != nil {
			klog.ErrorS(err, "Failed to stop MCP manager")
		}
	}

	klog.InfoS("AIAgent stopped")
	return nil
}

// ListTools 列出所有工具
func (a *Agent) ListTools() []map[string]string {
	tools := a.toolRegistry.List()
	result := make([]map[string]string, 0, len(tools))

	for _, tool := range tools {
		result = append(result, map[string]string{
			"name":        tool.Name,
			"description": tool.MCPTool.Description,
			"source":      tool.Source,
		})
	}

	return result
}

// Chat 处理聊天请求
func (a *Agent) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// 获取或创建对话
	conv := a.getOrCreateConversation(req.ConversationID)

	// 添加用户消息
	conv.AddMessage(api.Message{
		Role:    "user",
		Content: req.Message,
	})

	// 获取所有可用工具
	tools := a.getAllOllamaTools()

	// 开始对话循环
	return a.conversationLoop(ctx, conv, tools, req.Model)
}

// conversationLoop 对话循环（处理工具调用）
func (a *Agent) conversationLoop(ctx context.Context, conv *Conversation, tools []api.Tool, model string) (*ChatResponse, error) {
	if model == "" {
		model = a.cfg.Ollama.Model
	}

	maxIterations := 100 // 防止无限循环
	var toolCalls []ToolCallInfo

	for range maxIterations {
		// 获取对话消息
		messages := conv.GetMessages()

		// 仅在第一轮时注入系统提示和工具列表
		// var requestTools []api.Tool
		// if i == 0 && len(messages) > 0 {
		// 	systemMsg := api.Message{
		// 		Role:    "system",
		// 		Content: a.cfg.Ollama.SystemPrompt,
		// 	}
		// 	messages = append([]api.Message{systemMsg}, messages...)
		// 	// // 第一轮传递工具
		// 	// requestTools = tools
		// 	// klog.V(2).InfoS("First turn: injecting system prompt and tools", "tools", tools)
		// }

		// 调用 Ollama
		resp, err := a.ollama.Chat(ctx, messages, tools)
		if err != nil {
			return nil, fmt.Errorf("ollama chat failed: %w", err)
		}

		// 添加助手消息到历史
		conv.AddMessage(resp.Message)

		// 如果没有工具调用，返回结果
		if len(resp.Message.ToolCalls) == 0 {
			return &ChatResponse{
				Response:       resp.Message.Content,
				ToolCalls:      toolCalls,
				ConversationID: conv.ID,
			}, nil
		}

		// 处理工具调用
		klog.V(2).InfoS("Processing tool calls", "count", len(resp.Message.ToolCalls))
		for _, tc := range resp.Message.ToolCalls {
			result, err := a.executeToolCall(ctx, tc)
			if err != nil {
				klog.ErrorS(err, "Tool call failed", "tool", tc.Function.Name)
				result = fmt.Sprintf("Error: %v", err)
			}

			// 记录工具调用
			toolCalls = append(toolCalls, ToolCallInfo{
				Tool:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Result:    result,
			})

			// 添加工具结果到历史
			conv.AddMessage(api.Message{
				Role:    "tool",
				Content: result,
			})
		}
	}

	return nil, fmt.Errorf("max iterations reached")
}

// executeToolCall 执行工具调用
func (a *Agent) executeToolCall(ctx context.Context, tc api.ToolCall) (string, error) {
	toolName := tc.Function.Name

	// 检查工具是否存在
	tool := a.toolRegistry.Get(toolName)
	if tool == nil {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// 执行工具
	return tool.Executor.Execute(ctx, tc.Function.Arguments)
}

// getAllOllamaTools 获取所有工具的 Ollama Tool 定义
func (a *Agent) getAllOllamaTools() []api.Tool {
	var tools []api.Tool

	for _, tool := range a.toolRegistry.List() {
		ollamaTool := MCPToolToOllamaTool(tool.MCPTool)
		tools = append(tools, ollamaTool)
	}
	klog.InfoS("All tools", "tools", tools)

	return tools
}

// getOrCreateConversation 获取或创建对话
func (a *Agent) getOrCreateConversation(id string) *Conversation {
	if id == "" {
		id = generateConversationID()
	}

	val, ok := a.conversations.Load(id)
	if ok {
		return val.(*Conversation)
	}

	conv := NewConversation(id)
	a.conversations.Store(id, conv)
	return conv
}

func generateConversationID() string {
	return uuid.New().String()
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
	Model          string `json:"model,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Response       string         `json:"response"`
	ToolCalls      []ToolCallInfo `json:"tool_calls,omitempty"`
	ConversationID string         `json:"conversation_id"`
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
	Result    string         `json:"result"`
}

// AddRAGDocument 添加 RAG 文档
func (a *Agent) AddRAGDocument(ctx context.Context, id, content string, metadata map[string]string) error {
	return a.rag.AddDocument(ctx, id, content, metadata)
}

// AddRAGDocumentChunks 添加已分块的 RAG 文档
func (a *Agent) AddRAGDocumentChunks(ctx context.Context, id string, chunks []string, metadata map[string]string) error {
	return a.rag.AddDocumentWithChunks(ctx, id, chunks, metadata)
}

// ChatWithRAG 带 RAG 增强的聊天
func (a *Agent) ChatWithRAG(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// 获取 RAG 上下文（使用配置中的 TopK）
	ragContext, err := a.rag.GetContext(ctx, req.Message, a.cfg.RAG.TopK)
	if err != nil {
		klog.ErrorS(err, "Failed to get RAG context")
		// 即使 RAG 失败，也继续处理（降级到普通聊天）
	}

	// 获取或创建对话
	conv := a.getOrCreateConversation(req.ConversationID)

	// 如果有 RAG 上下文，添加到消息中
	enhancedMessage := req.Message
	if ragContext != "" {
		enhancedMessage = ragContext + "\n用户问题：" + req.Message
	}

	// 添加增强后的用户消息
	conv.AddMessage(api.Message{
		Role:    "user",
		Content: enhancedMessage,
	})

	// 获取所有可用工具
	tools := a.getAllOllamaTools()

	// 开始对话循环
	return a.conversationLoop(ctx, conv, tools, req.Model)
}

// RAGDocumentCount 返回 RAG 文档数量
func (a *Agent) RAGDocumentCount() int {
	return a.rag.DocumentCount()
}

// SearchRAG 搜索 RAG 文档
func (a *Agent) SearchRAG(ctx context.Context, query string) ([]rag.SearchResult, error) {
	return a.rag.Search(ctx, query, a.cfg.RAG.TopK)
}

// LoadRAGDocumentsFromDir 从目录加载所有 md 文件作为 RAG 文档
func (a *Agent) LoadRAGDocumentsFromDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理 .md 文件
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			klog.ErrorS(err, "Failed to read file", "file", filePath)
			continue
		}

		// 使用文件名（不含扩展名）作为文档 ID
		docID := entry.Name()[:len(entry.Name())-3]

		err = a.rag.AddDocument(ctx, docID, string(content), map[string]string{
			"source": filePath,
			"file":   entry.Name(),
		})
		if err != nil {
			klog.ErrorS(err, "Failed to add document", "file", filePath)
			continue
		}
		loadedCount++
	}

	klog.InfoS("RAG documents loaded", "dir", dir, "files", loadedCount, "totalChunks", a.rag.DocumentCount())
	return nil
}
