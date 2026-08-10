package agent

import (
	"context"

	"Qavor/internal/agent/localfs/security"
	"Qavor/internal/mcp"
	"Qavor/internal/skill"
	"Qavor/internal/tool"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// SubagentLLMResolver 为子智能体解析独立 LLM。
// 由 service 层实现（复用 ModelService.ResolveChatModel），避免 agent 包反向依赖 service。
type SubagentLLMResolver interface {
	ResolveChatModel(ctx context.Context, modelID uint) (model.ToolCallingChatModel, error)
}

// subagentSpec 子智能体构造规格：配置 + 已解析的 LLM。
// 由 AgentManager 组装（查配置、解析 LLM），NewAgent 消费构造 eino agent。
type subagentSpec struct {
	cfg *AgentConfig
	llm model.ToolCallingChatModel
}

// buildSubagentHandlers 构造子智能体的中间件列表。
// ownBuiltin/ownMCP 为子智能体自己的工具（分开传以支持 ToolFilter 的向量检索）。
// parentFSMW 为主智能体的 fsMW（backend/shell 已绑定，追加后子智能体共享主工作区）。
// vectorizer 可空（未启用工具检索时）。
func buildSubagentHandlers(
	spec *subagentSpec,
	ownBuiltin []einotool.BaseTool,
	ownMCP []einotool.BaseTool,
	parentFSMW adk.TypedChatModelAgentMiddleware[*schema.Message],
	vectorizer *mcp.ToolVectorizer,
) []adk.TypedChatModelAgentMiddleware[*schema.Message] {

	vectorCfg := RetrievalConfig{
		Enabled:   spec.cfg.ToolRetrievalEnabled,
		Threshold: spec.cfg.ToolRetrievalThreshold,
		TopK:      spec.cfg.ToolRetrievalTopK,
	}
	handlers := []adk.TypedChatModelAgentMiddleware[*schema.Message]{
		NewToolFilterMiddleware(ownBuiltin, ownMCP, vectorizer, vectorCfg),
	}
	if parentFSMW != nil {
		handlers = append(handlers, parentFSMW)
	}
	return handlers
}

// buildSubagentInstance 构造完整的子智能体 eino 实例（ChatModelAgent）。
// 复用子智能体分支的轻量构造逻辑，handlers 含子自己的工具过滤 + 主 fsMW + 子 skills。
// 子智能体专注执行，不携带 deep 的 write_todos/task/general 子编排工具。
func buildSubagentInstance(
	spec *subagentSpec,
	parentFSMW adk.TypedChatModelAgentMiddleware[*schema.Message],
	mcpManager *mcp.MCPManager,
	toolRegistry *tool.Registry,
	vectorizer *mcp.ToolVectorizer,
	skillsMiddleware *skill.SkillsMiddleware,
) (adk.TypedAgent[*schema.Message], error) {

	// 子智能体自己的工具（builtin + MCP 分开传，支持向量检索）
	subBuiltin, subMCP := resolveAgentTools(spec.cfg, mcpManager, toolRegistry)

	handlers := buildSubagentHandlers(spec, subBuiltin, subMCP, parentFSMW, vectorizer)
	// 子智能体的 skills 中间件
	if skillsMiddleware != nil {
		handlers = append(handlers, skillsMiddleware)
	}

	maxIteration := 20
	if spec.cfg.MaxIteration > 0 {
		maxIteration = spec.cfg.MaxIteration
	}

	toolErrorMW := []compose.ToolMiddleware{
		{
			Name: "qavor_tool_error",
			Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
				return WrapToolError(next, security.ErrDenied)
			},
			Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
				return WrapStreamToolError(next, security.ErrDenied)
			},
		},
	}

	return adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name:        spec.cfg.Name,
		Description: spec.cfg.Description,
		Instruction: spec.cfg.Instruction,
		Model:       spec.llm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               nil,
				ToolCallMiddlewares: toolErrorMW,
			},
		},
		MaxIterations: maxIteration,
		Handlers:      handlers,
	})
}
