package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/champly/ai-agent/pkg/agent"
	"k8s.io/klog/v2"
)

// Server HTTP API 服务器
type Server struct {
	agent  *agent.Agent
	server *http.Server
}

// NewServer 创建 API 服务器
func NewServer(addr string, ag *agent.Agent) *Server {
	s := &Server{
		agent: ag,
	}

	mux := http.NewServeMux()

	// 路由
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/tools", s.handleListTools)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// Start 启动服务器
func (s *Server) Start() error {
	klog.InfoS("HTTP API server starting", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	klog.InfoS("HTTP API server stopping")
	return s.server.Shutdown(ctx)
}

// handleChat 处理聊天请求
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	var req agent.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		klog.ErrorS(err, "Failed to decode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	klog.V(2).InfoS("Received chat request",
		"message", req.Message,
		"conversationID", req.ConversationID)

	// 处理请求
	resp, err := s.agent.Chat(r.Context(), &req)
	if err != nil {
		klog.ErrorS(err, "Chat failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		klog.ErrorS(err, "Failed to encode response")
	}
}

// handleListTools 列出所有工具
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	tools := s.agent.ListTools()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"tools": tools,
	}); err != nil {
		klog.ErrorS(err, "Failed to encode response")
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
