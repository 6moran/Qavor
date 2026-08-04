package shortterm

import (
	"strings"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// SessionStateManager 会话状态管理器
type SessionStateManager struct {
	logger *zap.Logger
}

// NewSessionStateManager 创建会话状态管理器
func NewSessionStateManager(logger *zap.Logger) *SessionStateManager {
	return &SessionStateManager{
		logger: logger,
	}
}

// UpdateState 更新会话状态（简单规则）
func (m *SessionStateManager) UpdateState(state *SessionState, message *schema.Message) {
	if state == nil {
		return
	}

	content := message.Content

	// 1. 更新用户意图（简单规则：识别问句）
	if message.Role == schema.User {
		if strings.Contains(content, "？") || strings.Contains(content, "?") {
			state.UserIntent = "question"
		} else if strings.Contains(content, "帮我") || strings.Contains(content, "请") {
			state.UserIntent = "request"
		} else {
			state.UserIntent = "statement"
		}
	}

	// 2. 更新关键实体（简单规则：提取引号内容）
	m.extractEntities(state, content)

	// 3. 更新主题（简单规则：使用最新的关键词）
	if message.Role == schema.User && len(content) > 0 {
		// 简单提取前10个字符作为主题
		if len(content) > 10 {
			state.Topic = content[:10] + "..."
		} else {
			state.Topic = content
		}
	}
}

// isQuote 判断字符是否为引号（支持 ASCII、中文左/右双引号）
func isQuote(ch rune) bool {
	return ch == '"' || ch == '“' || ch == '”'
}

// extractEntities 提取关键实体
func (m *SessionStateManager) extractEntities(state *SessionState, content string) {
	// 简单规则：提取引号内容
	// 后续可接入 NER 进行实体提取
	start := 0
	for i, ch := range content {
		if isQuote(ch) {
			if start == 0 {
				start = i + 1
			} else {
				entity := content[start:i]
				if entity != "" {
					// 检查是否已存在
					exists := false
					for _, e := range state.KeyEntities {
						if e == entity {
							exists = true
							break
						}
					}
					if !exists {
						state.KeyEntities = append(state.KeyEntities, entity)
					}
				}
				start = 0
			}
		}
	}

	// 限制实体数量
	if len(state.KeyEntities) > 10 {
		state.KeyEntities = state.KeyEntities[len(state.KeyEntities)-10:]
	}
}

// NewSessionState 创建新的会话状态
func NewSessionState() *SessionState {
	return &SessionState{
		Topic:       "",
		UserIntent:  "",
		KeyEntities: make([]string, 0),
		Metadata:    make(map[string]string),
	}
}
