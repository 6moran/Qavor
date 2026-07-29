package entity

import "time"

// AgentRun Agent运行任务实体
type AgentRun struct {
	ID                       string     `gorm:"type:varchar(64);primarykey;comment:Run ID（UUID）" json:"id"`
	ConversationThreadID     string     `gorm:"type:varchar(64);not null;index;comment:对话线程ID" json:"conversation_thread_id"`
	AgentSlug                string     `gorm:"type:varchar(64);not null;index;comment:Agent slug" json:"agent_slug"`
	Status                   string     `gorm:"type:varchar(32);not null;index;default:pending;comment:状态：pending/running/completed/failed/cancelled等" json:"status"`
	RequestID                string     `gorm:"type:varchar(64);uniqueIndex;not null;index;comment:幂等性请求ID" json:"request_id"`
	ConversationID           *uint      `gorm:"index;comment:对话ID" json:"conversation_id,omitempty"`
	CreatedByRunID           string     `gorm:"type:varchar(64);index;comment:父Run ID" json:"created_by_run_id,omitempty"`
	SubagentThreadRelationID *uint      `gorm:"index;comment:子智能体线程关系ID" json:"subagent_thread_relation_id,omitempty"`
	RunType                  string     `gorm:"type:varchar(32);not null;default:chat;comment:运行类型：chat/resume/subagent" json:"run_type"`
	InputMessageID           *uint      `gorm:"comment:输入消息ID" json:"input_message_id,omitempty"`
	OutputMessageID          *uint      `gorm:"comment:输出消息ID" json:"output_message_id,omitempty"`
	LastEventID              string     `gorm:"type:varchar(64);comment:Redis Stream最后事件ID" json:"last_event_id,omitempty"`
	InputPayload             JSON       `gorm:"type:json;not null;default:{};comment:原始输入payload" json:"input_payload"`
	ErrorType                string     `gorm:"type:varchar(64);comment:错误类型" json:"error_type,omitempty"`
	ErrorMessage             string     `gorm:"type:text;comment:错误信息" json:"error_message,omitempty"`
	StartedAt                *time.Time `gorm:"comment:开始时间" json:"started_at,omitempty"`
	FinishedAt               *time.Time `gorm:"comment:完成时间" json:"finished_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`

	// 关联关系
	Conversation           *Conversation   `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
	SubagentThreadRelation *SubagentThread `gorm:"foreignKey:SubagentThreadRelationID" json:"subagent_thread_relation,omitempty"`
}

// TableName 指定表名
func (AgentRun) TableName() string {
	return "agent_runs"
}

// IsTerminal 判断是否为终态
func (r *AgentRun) IsTerminal() bool {
	terminalStatuses := []string{"completed", "failed", "cancelled", "interrupted"}
	for _, status := range terminalStatuses {
		if r.Status == status {
			return true
		}
	}
	return false
}
