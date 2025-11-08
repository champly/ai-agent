package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server     ServerConfig      `yaml:"server"`
	Ollama     OllamaConfig      `yaml:"ollama"`
	MCPServers []MCPServerConfig `yaml:"mcp_servers"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Listen  string `yaml:"listen"`
	Debug   bool   `yaml:"debug"`
}

// OllamaConfig Ollama 配置
type OllamaConfig struct {
	Host       string        `yaml:"host"`
	Model      string        `yaml:"model"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
	// 系统提示，用于优化模型行为和减少 token 消耗
	SystemPrompt string `yaml:"system_prompt"`
}

// MCPServerConfig 外部 MCP 服务器配置
type MCPServerConfig struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	Env       map[string]string `yaml:"env"`
	Transport string            `yaml:"transport"` // stdio
	Enabled   bool              `yaml:"enabled"`
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// 设置默认值
	cfg.setDefaults()

	// 验证配置
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.Server.Name == "" {
		c.Server.Name = "AIAgent"
	}
	if c.Server.Version == "" {
		c.Server.Version = "v1.0.0"
	}
	if c.Server.Listen == "" {
		c.Server.Listen = "localhost:8080"
	}

	if c.Ollama.Host == "" {
		c.Ollama.Host = "http://localhost:11434"
	}
	if c.Ollama.Model == "" {
		c.Ollama.Model = "qwen3:8b"
	}
	if c.Ollama.Timeout == 0 {
		c.Ollama.Timeout = 120 * time.Second
	}
	if c.Ollama.MaxRetries == 0 {
		c.Ollama.MaxRetries = 3
	}
	if c.Ollama.SystemPrompt == "" {
		c.Ollama.SystemPrompt = defaultSystemPrompt
	}
}

// validate 验证配置
func (c *Config) validate() error {
	// 验证 Ollama 配置
	if c.Ollama.Host == "" {
		return fmt.Errorf("ollama host is required")
	}
	if c.Ollama.Model == "" {
		return fmt.Errorf("ollama model is required")
	}

	return nil
}

// defaultSystemPrompt 默认系统提示，用于优化模型行为和减少 token 消耗
const defaultSystemPrompt = `你是一个高效的AI助手，具备以下特性：
- 深度理解用户需求，避免不必要的重复工具调用
- 优先查看对话历史，利用已有信息回答问题
- 只在确实需要时才调用工具，避免盲目探索
- 支持批量工具调用，提高执行效率
- 提供清晰、准确的最终回答，简要说明工具使用情况
- 分析项目的时候需要读取项目中的每一个文件(递归遍历，特别是项目代码文件)`
