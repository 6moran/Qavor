package agent

import (
	"context"
	"fmt"
	"time"

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
		NewApprovalMiddleware(), // 子智能体也需要审批能力（敏感工具如 execute 触发中断）
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
		// 内层：文件不存在（多为知识库文档被误当 workspace 文件读取）时，
		// 返回可恢复结果并引导模型改用 query_kb。放在最后=最内层，优先于上面的通用错误喂回。
		newToolErrorRecoveryMiddleware(),
	}

	// 子智能体没有 ask_user 工具（被替换为 report_need_input），告知模型可以用它向用户提问。
	instruction := spec.cfg.Instruction
	if spec.cfg.Instruction == "" {
		instruction = "你是一个智能助手，可以根据用户的问题调用可用的工具来提供帮助。请用中文回答用户的问题。"
	}
	// 注入当前日期时间，解决模型训练数据截止日期导致的年份错误
	now := time.Now()
	instruction += fmt.Sprintf("\n\n当前日期时间：%s（时区：%s）", now.Format("2006-01-02 15:04:05"), now.Location().String())
	instruction += "\n\n你可以使用 report_need_input 工具来向用户提出问题或请求决策。当你遇到信息不足、需要用户选择或确认时，请使用 report_need_input 工具。它等同于 ask_user 工具，但名称不同。\n\n重要：当你使用 report_need_input 向用户提问并收到回答后，请在最终输出中明确声明你已经向用户提问并获得了回答。例如在回复开头写「我已向用户提问，以下是基于用户回答的…」之类的说明，这样父智能体才能清楚整个交互过程。"

	return adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name:        spec.cfg.Name,
		Description: spec.cfg.Description,
		Instruction: instruction,
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
