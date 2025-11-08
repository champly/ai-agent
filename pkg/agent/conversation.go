package agent

import (
	"sync"

	"github.com/ollama/ollama/api"
)

// Conversation 对话
type Conversation struct {
	ID       string
	messages []api.Message
	mu       sync.RWMutex
}

// NewConversation 创建对话
func NewConversation(id string) *Conversation {
	return &Conversation{
		ID:       id,
		messages: make([]api.Message, 0),
	}
}

// AddMessage 添加消息
func (c *Conversation) AddMessage(msg api.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
}

// GetMessages 获取所有消息
func (c *Conversation) GetMessages() []api.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 返回副本
	result := make([]api.Message, len(c.messages))
	copy(result, c.messages)
	return result
}
