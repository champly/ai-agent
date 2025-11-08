package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/klog/v2"

	"github.com/champly/ai-agent/pkg/config"
)

// MCPClient MCP 客户端管理器（连接到外部 MCP 服务器）
type MCPClient struct {
	configs []config.MCPServerConfig
	clients map[string]*MCPClientInfo
	mu      sync.RWMutex
}

// MCPClientInfo MCP 客户端信息
type MCPClientInfo struct {
	Name    string
	Client  *mcp.Client
	Session *mcp.ClientSession
	Cmd     *exec.Cmd
	Tools   []*mcp.Tool
}

// NewMCPClient 创建 MCP 客户端管理器
func NewMCPClient(configs []config.MCPServerConfig) *MCPClient {
	return &MCPClient{
		configs: configs,
		clients: make(map[string]*MCPClientInfo),
	}
}

// Start 启动所有 MCP 客户端
func (m *MCPClient) Start(ctx context.Context) error {
	for _, cfg := range m.configs {
		if !cfg.Enabled {
			klog.V(2).InfoS("Skipping disabled MCP server", "name", cfg.Name)
			continue
		}

		if err := m.startClient(ctx, cfg); err != nil {
			klog.ErrorS(err, "Failed to start MCP client", "name", cfg.Name)
			continue
		}
	}

	klog.InfoS("MCP Manager started", "clients", len(m.clients))
	return nil
}

// startClient 启动单个 MCP 客户端
func (m *MCPClient) startClient(ctx context.Context, cfg config.MCPServerConfig) error {
	klog.InfoS("Starting MCP client", "name", cfg.Name, "command", cfg.Command, "args", cfg.Args)

	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "ai-agent",
		Version: "v1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	toolsResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		session.Close()
		return fmt.Errorf("list tools failed: %w", err)
	}

	klog.InfoS("MCP client connected", "name", cfg.Name, "tools", len(toolsResult.Tools))

	m.mu.Lock()
	m.clients[cfg.Name] = &MCPClientInfo{
		Name:    cfg.Name,
		Client:  client,
		Session: session,
		Cmd:     cmd,
		Tools:   toolsResult.Tools,
	}
	m.mu.Unlock()

	return nil
}

// Stop 停止所有 MCP 客户端
func (m *MCPClient) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		klog.V(2).InfoS("Stopping MCP client", "name", name)
		if client.Session != nil {
			client.Session.Close()
		}
		if client.Cmd != nil && client.Cmd.Process != nil {
			client.Cmd.Process.Kill()
		}
	}

	klog.InfoS("MCP Manager stopped")
	return nil
}

// GetAllTools 获取所有外部 MCP 工具
func (m *MCPClient) GetAllTools() []*ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []*ToolInfo
	for _, client := range m.clients {
		for _, tool := range client.Tools {
			tools = append(tools, &ToolInfo{
				Name:    tool.Name,
				Source:  fmt.Sprintf("mcp:%s", client.Name),
				MCPTool: tool,
				Executor: &MCPToolExecutor{
					manager:    m,
					serverName: client.Name,
					toolName:   tool.Name,
				},
			})
		}
	}

	return tools
}

// CallTool 调用外部 MCP 工具
func (m *MCPClient) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	client, ok := m.clients[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server not found: %s", serverName)
	}

	klog.InfoS("MCP client calling tool", "server", serverName, "tool", toolName, "args", formatArgs(args))

	// 记录调用耗时
	startTime := time.Now()
	result, err := client.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	duration := time.Since(startTime)

	if err != nil {
		klog.ErrorS(err, "MCP tool call failed", "server", serverName, "tool", toolName, "duration", duration.Milliseconds(), "args", formatArgs(args))
		return nil, fmt.Errorf("call tool failed: %w", err)
	}

	klog.InfoS("MCP tool call completed", "server", serverName, "tool", toolName, "duration", duration.Milliseconds(), "durationMs", fmt.Sprintf("%.2fms", duration.Seconds()*1000))

	return result, nil
}

// MCPToolExecutor 外部 MCP 工具执行器
type MCPToolExecutor struct {
	manager    *MCPClient
	serverName string
	toolName   string
}

// Execute 执行工具
func (e *MCPToolExecutor) Execute(ctx context.Context, args map[string]any) (string, error) {
	result, err := e.manager.CallTool(ctx, e.serverName, e.toolName, args)
	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "", fmt.Errorf("no content in result")
}

func formatArgs(args map[string]any) string {
	data, _ := json.Marshal(args)
	return string(data)
}
