package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
	"k8s.io/klog/v2"
)

// Client Ollama 客户端（基于官方 SDK）
type Client struct {
	client *api.Client
	model  string
}

// NewClient 创建 Ollama 客户端
func NewClient(baseURL, model string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: timeout,
	}

	client := api.NewClient(u, httpClient)

	klog.InfoS("Ollama client created", "baseURL", baseURL, "model", model)
	return &Client{
		client: client,
		model:  model,
	}, nil
}

// Chat 发送聊天请求
func (c *Client) Chat(ctx context.Context, messages []api.Message, tools []api.Tool) (*api.ChatResponse, error) {
	stream := false
	req := &api.ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   &stream,
	}

	if len(tools) > 0 {
		req.Tools = tools
	}

	if reqJSON, err := json.MarshalIndent(req, "", "  "); err == nil {
		klog.V(3).InfoS("Ollama chat request", "req", string(reqJSON))
	}

	var resp api.ChatResponse
	err := c.client.Chat(ctx, req, func(r api.ChatResponse) error {
		resp = r
		return nil
	})
	if err != nil {
		klog.ErrorS(err, "Ollama chat failed")
		return nil, err
	}

	klog.V(3).InfoS("Ollama chat response",
		"role", resp.Message.Role,
		"content", resp.Message.Content,
		"toolCalls", len(resp.Message.ToolCalls))

	return &resp, nil
}

// Ping 检查 Ollama 服务是否可用
func (c *Client) Ping(ctx context.Context) error {
	// 使用 List 方法检查连接
	_, err := c.client.List(ctx)
	return err
}

// Embed 生成文本的嵌入向量
func (c *Client) Embed(ctx context.Context, model string, input string) ([]float32, error) {
	klog.V(3).InfoS("Ollama embed request", "model", model, "inputLen", len(input))

	req := &api.EmbedRequest{
		Model: model,
		Input: input,
	}

	resp, err := c.client.Embed(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Ollama embed failed")
		return nil, err
	}

	if len(resp.Embeddings) == 0 {
		return nil, nil
	}

	// 将 float64 转换为 float32
	embedding := make([]float32, len(resp.Embeddings[0]))
	for i, v := range resp.Embeddings[0] {
		embedding[i] = float32(v)
	}

	klog.V(3).InfoS("Ollama embed response",
		"model", model,
		"dimension", len(embedding))

	return embedding, nil
}
