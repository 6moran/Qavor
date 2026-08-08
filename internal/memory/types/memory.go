package memory

import "time"

// Record 记忆记录（公共类型）
type Record struct {
	ID                   string            `json:"id"`
	Category             string            `json:"category"`
	Content              string            `json:"content"`
	Importance           float64           `json:"importance"`
	Confidence           float64           `json:"confidence"`
	SourceConversationID uint              `json:"source_conversation_id"`
	SourceMessages       []string          `json:"source_messages"`
	Metadata             map[string]string `json:"metadata"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// Category 记忆类别
const (
	CategoryPreference  = "preference"
	CategoryIdentity    = "identity"
	CategoryEnvironment = "environment"
	CategoryKnowledge   = "knowledge"
	CategoryTask        = "sustainable_task"
	CategoryDecision    = "decision"
)

// ValidCategories 所有有效的记忆类别
var ValidCategories = map[string]bool{
	CategoryPreference:  true,
	CategoryIdentity:    true,
	CategoryEnvironment: true,
	CategoryKnowledge:   true,
	CategoryTask:        true,
	CategoryDecision:    true,
}

// IsValidCategory 检查类别是否有效
func IsValidCategory(category string) bool {
	return ValidCategories[category]
}
