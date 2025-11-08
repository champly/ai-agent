// Package mcpserver 实现 MCP Server 功能，提供文件系统工具
package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/klog/v2"
)

// ReadFileInput 读取文件的输入
type ReadFileInput struct {
	Path string `json:"path" jsonschema:"文件路径（绝对路径）"`
}

// ReadFileOutput 读取文件的输出
type ReadFileOutput struct {
	Content string `json:"content" jsonschema:"文件内容"`
}

// WriteFileInput 写入文件的输入
type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"文件路径（绝对路径）"`
	Content string `json:"content" jsonschema:"要写入的文件内容"`
}

// WriteFileOutput 写入文件的输出
type WriteFileOutput struct {
	Message string `json:"message" jsonschema:"操作结果消息"`
}

// ListDirectoryInput 列出目录的输入
type ListDirectoryInput struct {
	Path string `json:"path" jsonschema:"目录路径（绝对路径）"`
}

// ListDirectoryOutput 列出目录的输出
type ListDirectoryOutput struct {
	Entries []DirectoryEntry `json:"entries" jsonschema:"目录条目列表"`
}

// DirectoryEntry 目录条目
type DirectoryEntry struct {
	Name string `json:"name" jsonschema:"名称"`
	Type string `json:"type" jsonschema:"类型: directory 或 file"`
}

// MCPServer MCP 服务器实现
type MCPServer struct {
	server    *mcp.Server
	allowRoot string // 允许访问的根目录
}

// NewMCPServer 创建 MCP 服务器
func NewMCPServer(allowRoot string) (*MCPServer, error) {
	if allowRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory failed: %w", err)
		}
		allowRoot = cwd
	}

	// 确保目录存在
	if _, err := os.Stat(allowRoot); err != nil {
		return nil, fmt.Errorf("allow root directory not found: %s", allowRoot)
	}

	s := &MCPServer{
		allowRoot: allowRoot,
	}

	// 创建 MCP Server
	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "ai-agent-mcp-server",
		Version: "v1.0.0",
	}, &mcp.ServerOptions{
		HasTools: true,
	})

	// 注册工具
	s.registerTools()

	klog.InfoS("MCP Server created", "allowRoot", allowRoot)
	return s, nil
}

// registerTools 注册所有工具
func (s *MCPServer) registerTools() {
	// 注册 read_file 工具
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "read_file",
		Description: "读取文件内容",
	}, s.handleReadFile)

	// 注册 write_file 工具
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "write_file",
		Description: "写入文件内容",
	}, s.handleWriteFile)

	// 注册 list_directory 工具
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_directory",
		Description: "列出目录内容",
	}, s.handleListDirectory)
}

// Start 启动 MCP 服务器
func (s *MCPServer) Start(ctx context.Context, transport mcp.Transport) error {
	klog.InfoS("Starting MCP Server")
	return s.server.Run(ctx, transport)
}

// handleReadFile 处理文件读取请求
func (s *MCPServer) handleReadFile(ctx context.Context, req *mcp.CallToolRequest, input ReadFileInput) (*mcp.CallToolResult, ReadFileOutput, error) {
	klog.InfoS("MCP tool called: read_file", "path", input.Path)

	// 构建完整路径
	fullPath := filepath.Join(s.allowRoot, input.Path)

	// 安全检查：确保路径在允许的根目录下
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, ReadFileOutput{}, fmt.Errorf("resolve path failed: %w", err)
	}

	allowedPath, err := filepath.Abs(s.allowRoot)
	if err != nil {
		return nil, ReadFileOutput{}, fmt.Errorf("resolve allow root failed: %w", err)
	}

	relPath, err := filepath.Rel(allowedPath, absPath)
	if err != nil || (len(relPath) > 0 && relPath[0] == '.' && len(relPath) > 1 && relPath[1] == '.') {
		return nil, ReadFileOutput{}, fmt.Errorf("access denied: path outside allowed root")
	}

	klog.V(3).InfoS("Reading file", "path", absPath)

	// 读取文件
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, ReadFileOutput{}, fmt.Errorf("read file failed: %w", err)
	}

	return nil, ReadFileOutput{Content: string(content)}, nil
}

// handleWriteFile 处理文件写入请求
func (s *MCPServer) handleWriteFile(ctx context.Context, req *mcp.CallToolRequest, input WriteFileInput) (*mcp.CallToolResult, WriteFileOutput, error) {
	klog.InfoS("MCP tool called: write_file", "path", input.Path, "contentLength", len(input.Content))

	// 构建完整路径
	fullPath := filepath.Join(s.allowRoot, input.Path)

	// 安全检查
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, WriteFileOutput{}, fmt.Errorf("resolve path failed: %w", err)
	}

	allowedPath, err := filepath.Abs(s.allowRoot)
	if err != nil {
		return nil, WriteFileOutput{}, fmt.Errorf("resolve allow root failed: %w", err)
	}

	relPath, err := filepath.Rel(allowedPath, absPath)
	if err != nil || (len(relPath) > 0 && relPath[0] == '.' && len(relPath) > 1 && relPath[1] == '.') {
		return nil, WriteFileOutput{}, fmt.Errorf("access denied: path outside allowed root")
	}

	klog.V(3).InfoS("Writing file", "path", absPath, "size", len(input.Content))

	// 确保目录存在
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, WriteFileOutput{}, fmt.Errorf("create directory failed: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(absPath, []byte(input.Content), 0o644); err != nil {
		return nil, WriteFileOutput{}, fmt.Errorf("write file failed: %w", err)
	}

	msg := fmt.Sprintf("Successfully wrote %d bytes to %s", len(input.Content), input.Path)
	return nil, WriteFileOutput{Message: msg}, nil
}

// handleListDirectory 处理目录列表请求
func (s *MCPServer) handleListDirectory(ctx context.Context, req *mcp.CallToolRequest, input ListDirectoryInput) (*mcp.CallToolResult, ListDirectoryOutput, error) {
	klog.InfoS("MCP tool called: list_directory", "path", input.Path)

	// 构建完整路径
	fullPath := filepath.Join(s.allowRoot, input.Path)

	// 安全检查
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, ListDirectoryOutput{}, fmt.Errorf("resolve path failed: %w", err)
	}

	allowedPath, err := filepath.Abs(s.allowRoot)
	if err != nil {
		return nil, ListDirectoryOutput{}, fmt.Errorf("resolve allow root failed: %w", err)
	}

	relPath, err := filepath.Rel(allowedPath, absPath)
	if err != nil || (len(relPath) > 0 && relPath[0] == '.' && len(relPath) > 1 && relPath[1] == '.') {
		return nil, ListDirectoryOutput{}, fmt.Errorf("access denied: path outside allowed root")
	}

	klog.V(3).InfoS("Listing directory", "path", absPath)

	// 读取目录
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, ListDirectoryOutput{}, fmt.Errorf("read directory failed: %w", err)
	}

	// 构建结果，区分文件和目录
	result := make([]DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		entryType := "file"
		if entry.IsDir() {
			entryType = "directory"
		}
		result = append(result, DirectoryEntry{
			Name: entry.Name(),
			Type: entryType,
		})
	}

	return nil, ListDirectoryOutput{Entries: result}, nil
}
