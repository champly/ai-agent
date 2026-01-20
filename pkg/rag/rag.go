package rag

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"k8s.io/klog/v2"
)

// Document 文档结构
type Document struct {
	ID        string    // 文档ID
	Content   string    // 文档内容
	Embedding []float32 // 嵌入向量
	Metadata  map[string]string
}

// SearchResult 搜索结果
type SearchResult struct {
	Document   *Document
	Score      float32 // 相似度得分 (余弦相似度)
	ChunkIndex int
}

// EmbeddingFunc 嵌入函数类型
type EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

// RAG 检索增强生成模块
type RAG struct {
	mu           sync.RWMutex
	documents    []*Document
	embedFunc    EmbeddingFunc
	embedModel   string
	chunkSize    int // 分块大小
	chunkOverlap int // 分块重叠
}

// Config RAG 配置
type Config struct {
	EmbedModel   string // 嵌入模型名称
	ChunkSize    int    // 分块大小（字符数）
	ChunkOverlap int    // 分块重叠（字符数）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		EmbedModel:   "nomic-embed-text:latest",
		ChunkSize:    500,
		ChunkOverlap: 50,
	}
}

// New 创建 RAG 实例
func New(cfg *Config, embedFunc EmbeddingFunc) *RAG {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &RAG{
		documents:    make([]*Document, 0),
		embedFunc:    embedFunc,
		embedModel:   cfg.EmbedModel,
		chunkSize:    cfg.ChunkSize,
		chunkOverlap: cfg.ChunkOverlap,
	}
}

// AddDocument 添加文档
func (r *RAG) AddDocument(ctx context.Context, id, content string, metadata map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 分块处理
	chunks := r.splitText(content)

	for i, chunk := range chunks {
		// 生成嵌入向量
		embedding, err := r.embedFunc(ctx, chunk)
		if err != nil {
			return fmt.Errorf("failed to embed chunk %d: %w", i, err)
		}

		doc := &Document{
			ID:        fmt.Sprintf("%s_chunk_%d", id, i),
			Content:   chunk,
			Embedding: embedding,
			Metadata:  metadata,
		}
		r.documents = append(r.documents, doc)
	}

	klog.InfoS("Document added", "id", id, "chunks", len(chunks))
	return nil
}

// AddDocumentWithChunks 直接添加已分块的文档
func (r *RAG) AddDocumentWithChunks(ctx context.Context, id string, chunks []string, metadata map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	klog.InfoS("Adding document with pre-split chunks", "id", id, "chunks", len(chunks))

	for i, chunk := range chunks {
		embedding, err := r.embedFunc(ctx, chunk)
		if err != nil {
			return fmt.Errorf("failed to embed chunk %d: %w", i, err)
		}

		doc := &Document{
			ID:        fmt.Sprintf("%s_chunk_%d", id, i),
			Content:   chunk,
			Embedding: embedding,
			Metadata:  metadata,
		}
		r.documents = append(r.documents, doc)
	}

	klog.InfoS("Document chunks added successfully", "id", id, "totalChunks", len(chunks))
	return nil
}

// Search 搜索相关文档
func (r *RAG) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.documents) == 0 {
		return nil, nil
	}

	// 生成查询的嵌入向量
	queryEmbedding, err := r.embedFunc(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 计算相似度
	results := make([]SearchResult, 0, len(r.documents))
	for _, doc := range r.documents {
		score := cosineSimilarity(queryEmbedding, doc.Embedding)
		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 返回 top-K 结果
	if topK > len(results) {
		topK = len(results)
	}

	klog.V(2).InfoS("Search completed",
		"query", query,
		"totalDocs", len(r.documents),
		"topK", topK,
		"topScore", results[0].Score)

	return results[:topK], nil
}

// GetContext 获取增强上下文
func (r *RAG) GetContext(ctx context.Context, query string, topK int) (string, error) {
	results, err := r.Search(ctx, query, topK)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", nil
	}

	// 构建上下文
	var sb strings.Builder
	sb.WriteString("以下是与问题相关的参考资料：\n\n")

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("【参考资料 %d】(相关度: %.2f)\n", i+1, result.Score))
		sb.WriteString(result.Document.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("请基于以上参考资料回答用户问题。如果参考资料中没有相关信息，请明确说明。\n\n")

	return sb.String(), nil
}

// splitText 文本分块
func (r *RAG) splitText(text string) []string {
	// 使用 rune 来正确处理中文字符
	runes := []rune(text)

	// 防止无效配置
	chunkSize := r.chunkSize
	if chunkSize <= 0 {
		chunkSize = 500
	}
	chunkOverlap := r.chunkOverlap
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 10
	}

	// 简单的按字符分块，考虑重叠
	if len(runes) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		// 尝试在句号、问号、感叹号处断开（只在块结尾附近寻找）
		if end < len(runes) {
			// 只在最后 20% 范围内寻找句子结束符
			searchStart := start + chunkSize*4/5
			bestEnd := end
			for i := end; i > searchStart; i-- {
				c := runes[i]
				if c == '。' || c == '！' || c == '？' ||
					c == '.' || c == '!' || c == '?' ||
					c == '\n' {
					bestEnd = i + 1
					break
				}
			}
			end = bestEnd
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		// 下一块的起始位置：当前结束位置减去重叠
		// 确保至少前进 (chunkSize - chunkOverlap)
		minStep := chunkSize - chunkOverlap
		if minStep < 1 {
			minStep = 1
		}
		newStart := start + minStep
		// 如果 end 距离 start 超过 minStep，可以用 end - overlap
		if end-start > minStep {
			newStart = end - chunkOverlap
		}
		if newStart <= start {
			newStart = start + 1
		}
		start = newStart
	}

	return chunks
}

// DocumentCount 返回文档数量
func (r *RAG) DocumentCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.documents)
}

// Clear 清空所有文档
func (r *RAG) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.documents = make([]*Document, 0)
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}
