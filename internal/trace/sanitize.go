package trace

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// sensitiveKeys 需要脱敏的键名（包含匹配，大小写不敏感）
var sensitiveKeys = []string{
	"authorization", "token", "api_key", "apikey", "password", "secret", "cookie",
}

// bearerRe 匹配 Bearer <token> 模式
var bearerRe = regexp.MustCompile(`(?i)Bearer\s+\S+`)

// Sanitizer 内容安全策略：截断 + 敏感键脱敏
// 用于保护敏感信息（API Key、密码等）和控制存储成本（避免超大 prompt 入库）
//
// 两种模式：
//   - "summary"：脱敏 + 截断（默认，适合生产环境）
//   - "none"：不存任何内容（适合内容合规要求严格的部署，只存结构信息）
//
// 截断按 Unicode rune 计算（中文友好，不会截出半个字），默认 500 字符
type Sanitizer struct {
	Mode     string // 内容模式："summary"（截断+脱敏）或 "none"（不存任何内容）
	MaxRunes int    // 最大字符数（默认 500），超出部分被截断
}

// Text 处理普通文本：none 模式返回空串，summary 模式按 rune 截断 + Bearer 脱敏
func (s Sanitizer) Text(value string) string {
	if s.Mode == "none" {
		return ""
	}
	value = bearerRe.ReplaceAllString(value, "Bearer [REDACTED]")
	return truncateRunes(value, s.MaxRunes)
}

// JSON 处理 JSON 字符串：none 模式返回空串，summary 模式递归脱敏敏感键 + 截断
// 非法 JSON 回退到 Text 处理
func (s Sanitizer) JSON(value string) string {
	if s.Mode == "none" {
		return ""
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	// 尝试解析为 JSON
	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		// 非法 JSON 走文本路径
		return s.Text(trimmed)
	}
	redacted := redactSensitive(raw)
	out, err := json.Marshal(redacted)
	if err != nil {
		return s.Text(trimmed)
	}
	return truncateRunes(string(out), s.MaxRunes)
}

// truncateRunes 按 Unicode rune 截断
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// redactSensitive 递归遍历 JSON 结构，替换敏感键的值
func redactSensitive(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			if isSensitiveKey(k) {
				result[k] = "[REDACTED]"
			} else {
				result[k] = redactSensitive(v)
			}
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = redactSensitive(v)
		}
		return result
	default:
		return v
	}
}

// isSensitiveKey 判断键名是否包含敏感词（大小写不敏感）
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// MessageTextWithoutReasoning 提取消息文本（Content + MultiContent 的 text part）
// 不读取 reasoning part
func MessageTextWithoutReasoning(m *schema.Message) string {
	if m == nil {
		return ""
	}
	if m.Content != "" {
		return m.Content
	}
	var sb strings.Builder
	for _, part := range m.MultiContent {
		if part.Type == schema.ChatMessagePartTypeText {
			sb.WriteString(part.Text)
		}
	}
	return sb.String()
}
