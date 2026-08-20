package shortterm

import (
	"time"
)

// SessionContext 会话短期上下文（Redis 中的顶层对象）
type SessionContext struct {
	ConversationID uint       `json:"conversation_id"`
	Messages       []Message  `json:"messages"`   // 最近消息（滚动窗口）
	Summary        string     `json:"summary"`    // 任务可恢复摘要
	TaskState      *TaskState `json:"task_state"` // 任务状态
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Message 最近消息（轻量结构，不存 importance）
type Message struct {
	Role      string    `json:"role"`      // user / assistant
	Content   string    `json:"content"`   // 消息内容
	Timestamp time.Time `json:"timestamp"` // 时间戳
	Tokens    int       `json:"tokens"`    // 估算 Token 数
}

// TaskState 任务状态（回答"Agent 继续干活需要知道什么"）
type TaskState struct {
	Goal           string    `json:"goal"`             // 当前任务目标
	CompletedSteps []string  `json:"completed_steps"`  // 已完成的步骤
	PendingSteps   []string  `json:"pending_steps"`    // 待完成的步骤
	TechContext    []string  `json:"tech_context"`     // 相关技术上下文（文件/工具/框架）
	LastActivityAt time.Time `json:"last_activity_at"` // 最后活跃时间
}

// SummaryConfig 摘要配置
type SummaryConfig struct {
	MessageThreshold int  // 消息数量阈值
	TokenThreshold   int  // Token 阈值
	EnableAsync      bool // 是否启用异步生成
}

// DefaultSummaryConfig 默认摘要配置
// TokenThreshold 应小于上下文窗口的 MaxTokens，确保摘要先于裁剪生成
func DefaultSummaryConfig() *SummaryConfig {
	return &SummaryConfig{
		MessageThreshold: 20,
		TokenThreshold:   26000,
		EnableAsync:      true,
	}
}

// NewTaskState 创建空的任务状态
func NewTaskState() *TaskState {
	return &TaskState{
		Goal:           "",
		CompletedSteps: make([]string, 0),
		PendingSteps:   make([]string, 0),
		TechContext:    make([]string, 0),
		LastActivityAt: time.Now(),
	}
}
