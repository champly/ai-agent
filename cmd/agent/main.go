package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/champly/ai-agent/pkg/agent"
	"github.com/champly/ai-agent/pkg/config"
	"github.com/champly/ai-agent/pkg/server"
	"k8s.io/klog/v2"
)

var configFile = flag.String("config", "config.yaml", "配置文件路径")

func main() {
	// 初始化 klog
	klog.InitFlags(nil)
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		klog.ErrorS(err, "Failed to load config", "file", *configFile)
		os.Exit(1)
	}

	// 设置日志级别
	if cfg.Server.Debug {
		flag.Set("v", "3")
	}

	klog.InfoS("Starting AIAgent",
		"name", cfg.Server.Name,
		"version", cfg.Server.Version)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 Bridge（HTTP API 服务器）
	runBridge(ctx, cfg)
}

// runBridge 运行 Bridge 模式
func runBridge(ctx context.Context, cfg *config.Config) {
	// 创建代理
	ag, err := agent.New(cfg)
	if err != nil {
		klog.ErrorS(err, "Failed to create agent")
		os.Exit(1)
	}

	// 启动代理
	if err := ag.Start(ctx); err != nil {
		klog.ErrorS(err, "Failed to start agent")
		os.Exit(1)
	}

	// 创建 HTTP API 服务器
	apiServer := server.NewServer(cfg.Server.Listen, ag)

	// 启动服务器（在 goroutine 中）
	go func() {
		if err := apiServer.Start(); err != nil {
			klog.ErrorS(err, "HTTP server failed")
		}
	}()

	klog.InfoS("AIAgent ready", "listen", cfg.Server.Listen)

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	klog.InfoS("Received signal, shutting down", "signal", sig)

	// 优雅关闭
	if err := apiServer.Stop(ctx); err != nil {
		klog.ErrorS(err, "Failed to stop server")
	}

	if err := ag.Stop(ctx); err != nil {
		klog.ErrorS(err, "Failed to stop agent")
	}

	klog.InfoS("AIAgent shutdown complete")
	klog.Flush()

	fmt.Println("Goodbye!")
}
