package entity

import "time"

// ToolCall 工具调用实体
type ToolCall struct {
	ID                  uint      `gorm:"primarykey" json:"id"`
	MessageID           uint      `gorm:"not null;index;comment:所属消息ID" json:"message_id"`
	LanggraphToolCallID string    `gorm:"type:varchar(100);index;comment:LangGraph工具调用ID" json:"langgraph_tool_call_id,omitempty"`
	ToolName            string    `gorm:"type:varchar(100);not null;comment:工具名称" json:"tool_name"`
	ToolInput           JSON      `gorm:"type:json;comment:工具输入参数" json:"tool_input,omitempty"`
	ToolOutput          string    `gorm:"type:text;comment:工具执行结果" json:"tool_output,omitempty"`
	Status              string    `gorm:"type:varchar(20);default:pending;comment:状态：pending/success/error" json:"status"`
	ErrorMessage        string    `gorm:"type:text;comment:错误信息" json:"error_message,omitempty"`
	CreatedAt           time.Time `json:"created_at"`

	// 关联关系
	Message *Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName 指定表名
func (ToolCall) TableName() string {
	return "tool_calls"
}
