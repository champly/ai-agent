package agent

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolExecutor 工具执行器接口
type ToolExecutor interface {
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// ToolInfo 工具信息
type ToolInfo struct {
	Name     string
	Source   string // "local_mcp", "external_mcp", etc.
	MCPTool  *mcp.Tool
	Executor ToolExecutor
}

// ToolRegistry 工具注册表
type ToolRegistry struct {
	tools map[string]*ToolInfo
	mu    sync.RWMutex
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*ToolInfo),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(tool *ToolInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Get 获取工具
func (r *ToolRegistry) Get(name string) *ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List 列出所有工具
func (r *ToolRegistry) List() []*ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ToolInfo, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// Count 获取工具数量
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
