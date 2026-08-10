package builtin

import (
	"context"
	"errors"
	"strings"

	"Qavor/internal/service"
	"Qavor/internal/tool"
)

// QueryKBTool 检索当前智能体已配置知识库中的相关原文。
type QueryKBTool struct {
	ragSvc service.RAGService
}

// NewQueryKBTool 创建知识库检索工具。
func NewQueryKBTool(ragSvc service.RAGService) *QueryKBTool {
	return &QueryKBTool{ragSvc: ragSvc}
}

// Meta 返回工具元数据。
func (t *QueryKBTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        tool.QueryKBToolName,
		Label:       "查询知识库",
		Description: "检索当前智能体已配置知识库中的相关原文。返回内容来自知识库而不是 Agent workspace；document_name 不是本地文件路径，不要传给 read_file。需要事实依据或内部资料时使用。",
		Category:    tool.CategoryKnowledge,
		Args: []tool.ArgDef{
			{Name: "query_text", Type: "string", Description: "用于检索知识库的问题或关键词", Required: true},
			{Name: "top_k", Type: "integer", Description: "最多返回的相关片段数量，范围 1-20", Required: false},
		},
	}
}

// Execute 执行知识库检索。知识库范围只能来自运行上下文，LLM 无法指定。
func (t *QueryKBTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	if t == nil || t.ragSvc == nil {
		return nil, errors.New("rag service is not configured")
	}
	kbIDs := tool.KnowledgeBaseIDsFromContext(ctx)
	if len(kbIDs) == 0 {
		return nil, errors.New("knowledge base scope is not bound to context")
	}

	query, ok := args["query_text"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return nil, errors.New("query_text must be a non-empty string")
	}

	// 默认 top_k，当 LLM 传入值超出有效范围时使用
	const defaultTopK = 10

	topK := defaultTopK
	if raw, exists := args["top_k"]; exists {
		parsed := parseTopK(raw)
		if parsed >= 1 && parsed <= 20 {
			topK = parsed
		}
		// 无效值（超出范围或非整数）静默使用默认值，不报错
	}

	result, err := t.ragSvc.Retrieve(ctx, kbIDs, query, topK)
	if err != nil {
		return nil, err
	}
	chunks := result.Chunks
	if chunks == nil {
		chunks = make([]service.RAGChunk, 0)
	}
	for i := range chunks {
		chunks[i].DocumentName = chunks[i].Filename
		if chunks[i].KBID != "" && chunks[i].FileID != "" {
			chunks[i].ResourceURI = "knowledge://" + chunks[i].KBID + "/" + chunks[i].FileID
		}
	}
	return map[string]any{
		"query_text": result.QueryText,
		"chunks":     chunks,
	}, nil
}

// parseTopK 将 LLM 传入的参数转为整数；非整数或非数值返回 -1。
func parseTopK(raw any) int {
	switch v := raw.(type) {
	case float64:
		if v != float64(int(v)) {
			return -1
		}
		return int(v)
	case int:
		return v
	default:
		return -1
	}
}
