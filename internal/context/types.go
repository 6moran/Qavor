package context

import (
	"time"

	"github.com/cloudwego/eino/schema"
)

// ContextConfig 上下文配置
type ContextConfig struct {
	MaxTokens      int    // 模型最大 Token 限制（如 4096、8192、128000）
	ReserveTokens  int    // 预留给回复的 Token 数量
	SystemPrompt   string // 系统提示词
	TokenizerModel string // Token 计数使用的模型标识
}

// ContextWindow 上下文窗口
type ContextWindow struct {
	Messages         []*schema.Message // 裁剪后的消息列表
	TotalTokens      int               // 消息总 Token 数
	TrimmedCount     int               // 被裁剪的消息数量
	HasSystem        bool              // 是否包含系统消息
	ShortTermSummary string            // 任务可恢复摘要（Agent 中断后继续工作所需信息）
	ShortTermState   string            // 任务状态（目标/进度/技术上下文，由短期记忆模块抽取）
	MemoryContext    string            // 长期记忆上下文（可选）
	RAGContext       string            // RAG 检索结果（可选）
	ToolDefinitions  []interface{}     // 工具定义（可选）
	TokenUsage       *TokenUsage       // Token 用量统计（可选）
}

// TokenUsage Token 用量统计
type TokenUsage struct {
	ConversationID uint          // 会话 ID
	InputTokens    int           // 输入 Token 数
	OutputTokens   int           // 输出 Token 数
	TrimmedCount   int           // 被裁剪的消息数
	ProcessingTime time.Duration // 处理耗时
	Timestamp      time.Time     // 统计时间
}

// ContextHistoryQuery 历史查询参数
type ContextHistoryQuery struct {
	ConversationID     uint     // 会话 ID
	Limit              int      // 最大消息数量
	BeforeID           uint     // 分页：此 ID 之前的消息（游标分页）
	Roles              []string // 过滤角色（可选）
	SkipLongTermMemory bool     // 跳过长期记忆召回（用于 UI 状态查询等无需 LLM Prompt 的场景）
}

// AgentTokenUsage Agent状态面板的Token用量详情
type AgentTokenUsage struct {
	SummaryActive                 bool `json:"summary_active"`
	SummaryMessageTokens          int  `json:"summary_message_tokens"`
	LlmMessagesTokens             int  `json:"llm_messages_tokens"`
	LlmContentMessageTokens       int  `json:"llm_content_message_tokens"`
	LlmToolMessageTokens          int  `json:"llm_tool_message_tokens"`
	StateMessagesTokensBeforeCall int  `json:"state_messages_tokens_before_call"`
	StateMessageCountBeforeCall   int  `json:"state_message_count_before_call"`
	LlmMessageCount               int  `json:"llm_message_count"`
	LlmContentMessageCount        int  `json:"llm_content_message_count"`
	LlmToolMessageCount           int  `json:"llm_tool_message_count"`
	SystemTokens                  int  `json:"system_tokens"`
	ToolsTokens                   int  `json:"tools_tokens"`
	ToolCount                     int  `json:"tool_count"`
	LlmInputTokens                int  `json:"llm_input_tokens"`
	SummaryTriggerTokens          int  `json:"summary_trigger_tokens"`
	ContextWindow                 int  `json:"context_window"`
	RemainingContextTokens        int  `json:"remaining_context_tokens"`
}

// AgentState Agent状态面板数据（对应前端 agent_state）
type AgentState struct {
	TokenUsage    *AgentTokenUsage     `json:"token_usage"`
	Todos         []AgentTodo          `json:"todos"`
	Files         map[string]AgentFile `json:"files"`
	SubagentRuns  []AgentSubagentRun   `json:"subagent_runs"`
	Artifacts     []string             `json:"artifacts"`
	MemoryMetrics interface{}          `json:"memory_metrics"`
}

// AgentTodo 待办项
type AgentTodo struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"` // pending/in_progress/completed/cancelled
}

// AgentFile 文件信息
type AgentFile struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
	Status   string `json:"status"`
}

// AgentSubagentRun 子Agent运行状态
type AgentSubagentRun struct {
	ID            string `json:"id"`
	RunID         string `json:"run_id"`
	ChildThreadID string `json:"child_thread_id"`
	SubagentSlug  string `json:"subagent_slug"`
	Status        string `json:"status"` // running/completed/failed
	Description   string `json:"description"`
}
