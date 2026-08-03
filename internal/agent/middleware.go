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

// activatedSkillsKey context key 用于传递已激活的技能 slug 列表
type activatedSkillsKey struct{}

func WithActivatedSkills(ctx context.Context, slugs []string) context.Context {
	return context.WithValue(ctx, activatedSkillsKey{}, slugs)
}

func ActivatedSkillsFrom(ctx context.Context) []string {
	if v, ok := ctx.Value(activatedSkillsKey{}).([]string); ok {
		return v
	}
	return nil
}

type RetrievalConfig struct {
	Enabled   bool
	Threshold int
	TopK      int
}

type ToolFilterMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.Message]
	builtinTools []einotool.BaseTool            // 内置工具，始终保留
	mcpTools     []einotool.BaseTool            // Agent 配置的 MCP 工具
	skillTools   map[string][]einotool.BaseTool // slug → 该技能依赖的工具（初始隐藏）
	vectorizer   *mcp.ToolVectorizer
	retrieval    RetrievalConfig
}

func NewToolFilterMiddleware(
	builtinTools, mcpTools []einotool.BaseTool,
	skillTools map[string][]einotool.BaseTool,
	vectorizer *mcp.ToolVectorizer,
	retrieval RetrievalConfig,
) *ToolFilterMiddleware {
	if skillTools == nil {
		skillTools = make(map[string][]einotool.BaseTool)
	}
	return &ToolFilterMiddleware{
		builtinTools: builtinTools,
		mcpTools:     mcpTools,
		skillTools:   skillTools,
		vectorizer:   vectorizer,
		retrieval:    retrieval,
	}
}

func (m *ToolFilterMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext[*schema.Message]) (context.Context, *adk.ChatModelAgentContext[*schema.Message], error) {
	// 1. 内置工具始终保留
	result := make([]einotool.BaseTool, 0, len(m.builtinTools)+len(m.mcpTools))
	result = append(result, m.builtinTools...)

	// 2. MCP 工具：向量检索或直接透传
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

	// 3. 门控释放：已激活的 Skill 释放其依赖工具
	activated := ActivatedSkillsFrom(ctx)
	for _, slug := range activated {
		if tools, ok := m.skillTools[slug]; ok {
			result = append(result, tools...)
		}
	}

	nRunCtx := *runCtx
	nRunCtx.Tools = result
	return ctx, &nRunCtx, nil
}

// BeforeModelRewriteState 在每次模型调用前检查技能激活状态
// 技能在工具执行期间被激活，通过 RunLocalValue 传递，此时注入其依赖工具
func (m *ToolFilterMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.Message], _ *adk.TypedModelContext[*schema.Message]) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	// 从 agent 运行时上下文读取已激活的技能
	val, ok, _ := adk.GetRunLocalValue(ctx, "activated_skills")
	if !ok {
		return ctx, state, nil
	}
	activated, _ := val.([]string)
	if len(activated) == 0 {
		return ctx, state, nil
	}

	// 收集已激活技能的依赖工具名（避免重复）
	existing := make(map[string]bool, len(state.ToolInfos))
	for _, info := range state.ToolInfos {
		existing[info.Name] = true
	}

	for _, slug := range activated {
		tools, ok := m.skillTools[slug]
		if !ok {
			continue
		}
		for _, t := range tools {
			info, err := t.Info(ctx)
			if err != nil || existing[info.Name] {
				continue
			}
			state.ToolInfos = append(state.ToolInfos, info)
			existing[info.Name] = true
		}
	}

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
