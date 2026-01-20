package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/champly/ai-agent/pkg/agent"
	"github.com/champly/ai-agent/pkg/rag"
	"k8s.io/klog/v2"
)

// ensure rag package is imported
var _ = rag.SearchResult{}

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
	mux.HandleFunc("/api/chat/rag", s.handleChatWithRAG)
	mux.HandleFunc("/api/rag/add", s.handleRAGAdd)
	mux.HandleFunc("/api/rag/import", s.handleRAGImport)
	mux.HandleFunc("/api/rag/search", s.handleRAGSearch)
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

// handleChatWithRAG 带 RAG 增强的聊天请求
func (s *Server) handleChatWithRAG(w http.ResponseWriter, r *http.Request) {
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

	klog.V(2).InfoS("Received RAG chat request",
		"message", req.Message,
		"conversationID", req.ConversationID)

	// 处理请求（top_k 从配置中获取）
	resp, err := s.agent.ChatWithRAG(r.Context(), &req)
	if err != nil {
		klog.ErrorS(err, "RAG Chat failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		klog.ErrorS(err, "Failed to encode response")
	}
}

// handleRAGAdd 添加 RAG 文档
func (s *Server) handleRAGAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       string            `json:"id"`
		Content  string            `json:"content"`
		Chunks   []string          `json:"chunks,omitempty"` // 可选：预分块的内容
		Metadata map[string]string `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		klog.ErrorS(err, "Failed to decode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	var err error
	if len(req.Chunks) > 0 {
		// 使用预分块的内容
		err = s.agent.AddRAGDocumentChunks(r.Context(), req.ID, req.Chunks, req.Metadata)
	} else if req.Content != "" {
		// 自动分块
		err = s.agent.AddRAGDocument(r.Context(), req.ID, req.Content, req.Metadata)
	} else {
		http.Error(w, "Content or chunks is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		klog.ErrorS(err, "Failed to add RAG document")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"document_count": s.agent.RAGDocumentCount(),
	})
}

// handleRAGSearch 搜索 RAG 文档
func (s *Server) handleRAGSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		klog.ErrorS(err, "Failed to decode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	// top_k 从配置中获取
	results, err := s.agent.SearchRAG(r.Context(), req.Query)
	if err != nil {
		klog.ErrorS(err, "Failed to search RAG")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 构建响应
	type searchResult struct {
		ID       string            `json:"id"`
		Content  string            `json:"content"`
		Score    float32           `json:"score"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}

	respResults := make([]searchResult, 0, len(results))
	for _, r := range results {
		respResults = append(respResults, searchResult{
			ID:       r.Document.ID,
			Content:  r.Document.Content,
			Score:    r.Score,
			Metadata: r.Document.Metadata,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"results": respResults,
		"count":   len(respResults),
	})
}

// handleRAGImport 从文件夹导入 RAG 文档
func (s *Server) handleRAGImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		klog.ErrorS(err, "Failed to decode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Dir == "" {
		http.Error(w, "Directory path is required", http.StatusBadRequest)
		return
	}

	klog.InfoS("Importing RAG documents from directory", "dir", req.Dir)

	if err := s.agent.LoadRAGDocumentsFromDir(r.Context(), req.Dir); err != nil {
		klog.ErrorS(err, "Failed to import RAG documents")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"document_count": s.agent.RAGDocumentCount(),
	})
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
