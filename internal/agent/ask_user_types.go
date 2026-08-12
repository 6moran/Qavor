package agent

import (
	"github.com/cloudwego/eino/schema"
)

// AskUserQuestion 单个问题定义
type AskUserQuestion struct {
	QuestionID  string   `json:"question_id"`
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	AllowOther  bool     `json:"allow_other,omitempty"`
}

// AskUserRequest StatefulInterrupt 的 info，向外层传递问题列表
type AskUserRequest struct {
	Questions []AskUserQuestion `json:"questions"`
}

// askUserState 持久化的内部状态（仅存问题列表，恢复时不需要额外状态）
type askUserState struct {
	Questions []AskUserQuestion
}

func init() {
	schema.RegisterName[*AskUserRequest]("agent.AskUserRequest")
	schema.RegisterName[askUserState]("agent.askUserState")
}