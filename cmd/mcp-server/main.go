package main

import (
	"context"
	"flag"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/klog/v2"

	"github.com/champly/ai-agent/pkg/mcpserver"
)

var allowRoot = flag.String("allow-root", "/tmp", "允许访问的根目录")

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// 创建 MCP Server
	server, err := mcpserver.NewMCPServer(*allowRoot)
	if err != nil {
		klog.ErrorS(err, "Failed to create MCP server")
		os.Exit(1)
	}

	// 使用 stdio 传输
	transport := &mcp.StdioTransport{}

	klog.InfoS("Starting builtin MCP Server", "allowRoot", *allowRoot)

	// 启动 MCP Server（阻塞）
	ctx := context.Background()
	if err := server.Start(ctx, transport); err != nil {
		klog.ErrorS(err, "MCP server failed")
		os.Exit(1)
	}
}
