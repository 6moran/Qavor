package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"Qavor/internal/mcp"
)

// queryKey context key 用于传递用户查询
type queryKey struct{}

func WithQuery(ctx context.Context, query string) context.Context {
	return context.WithValue(ctx, queryKey{}, query)
}

func QueryFrom(ctx context.Context) string {
	if v, ok := ctx.Value(queryKey{}).(string); ok {
		return v
	}
	return ""
}

type RetrievalConfig struct {
	Enabled   bool
	Threshold int
	TopK      int
}

type ToolFilterMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.Message]
	builtinTools []einotool.BaseTool // 内置工具，始终保留
	mcpTools     []einotool.BaseTool // Agent 配置的 MCP 工具
	vectorizer   *mcp.ToolVectorizer
	retrieval    RetrievalConfig
}

func NewToolFilterMiddleware(
	builtinTools, mcpTools []einotool.BaseTool,
	vectorizer *mcp.ToolVectorizer,
	retrieval RetrievalConfig,
) *ToolFilterMiddleware {
	return &ToolFilterMiddleware{
		builtinTools: builtinTools,
		mcpTools:     mcpTools,
		vectorizer:   vectorizer,
		retrieval:    retrieval,
	}
}

func (m *ToolFilterMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext[*schema.Message]) (context.Context, *adk.ChatModelAgentContext[*schema.Message], error) {
	// MCP 工具名集合：从传入工具中排除，稍后按检索注入，避免重复/覆盖
	mcpNameSet := make(map[string]bool, len(m.mcpTools))
	for _, t := range m.mcpTools {
		if info, err := t.Info(ctx); err == nil {
			mcpNameSet[info.Name] = true
		}
	}

	// 1. 内置工具始终保留
	result := make([]einotool.BaseTool, 0, len(runCtx.Tools)+len(m.mcpTools))
	result = append(result, m.builtinTools...)
	seen := make(map[string]bool, len(m.builtinTools))
	for _, t := range m.builtinTools {
		if info, err := t.Info(ctx); err == nil {
			seen[info.Name] = true
		}
	}

	// 2. 保留 runCtx 传入的已有工具（如 deep 的 filesystem 工具 read_file/execute 等），
	//    排除本中间件管理的 MCP 工具与已注入的内置工具
	for _, t := range runCtx.Tools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		if mcpNameSet[info.Name] {
			continue
		}
		if seen[info.Name] {
			continue
		}
		seen[info.Name] = true
		result = append(result, t)
	}

	// 3. MCP 工具：向量检索或直接透传
	mcpTools := m.mcpTools
	query := QueryFrom(ctx)
	if m.vectorizer != nil && m.retrieval.Enabled && len(m.mcpTools) > m.retrieval.Threshold && query != "" {
		names := m.vectorizer.SelectTools(ctx, query, m.retrieval.TopK)
		if names != nil {
			nameSet := make(map[string]bool, len(names))
			for _, n := range names {
				nameSet[n] = true
			}
			var filtered []einotool.BaseTool
			for _, t := range m.mcpTools {
				info, err := t.Info(ctx)
				if err != nil {
					continue
				}
				if nameSet[info.Name] {
					filtered = append(filtered, t)
				}
			}
			mcpTools = filtered
		}
	}
	result = append(result, mcpTools...)

	nRunCtx := *runCtx
	nRunCtx.Tools = result
	return ctx, &nRunCtx, nil
}

// BeforeModelRewriteState 在每次模型调用前检查技能激活状态（暂无操作）
func (m *ToolFilterMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.Message], _ *adk.TypedModelContext[*schema.Message]) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	return ctx, state, nil
}

func staticFilter(ctx context.Context, tools []einotool.BaseTool, enabledNames []string, disabledNames []string) []einotool.BaseTool {
	disabledSet := make(map[string]bool, len(disabledNames))
	for _, name := range disabledNames {
		disabledSet[name] = true
	}
	enabledSet := make(map[string]bool, len(enabledNames))
	for _, name := range enabledNames {
		enabledSet[name] = true
	}

	var result []einotool.BaseTool
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		if disabledSet[info.Name] {
			continue
		}
		if len(enabledSet) > 0 && !enabledSet[info.Name] {
			continue
		}
		result = append(result, t)
	}
	return result
}
