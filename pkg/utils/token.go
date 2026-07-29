package utils

import (
	"github.com/cloudwego/eino/schema"
	"github.com/pkoukk/tiktoken-go"
)

// tokenizer 缓存 tokenizer 实例，避免重复创建
var tokenizer *tiktoken.Tiktoken

// init 初始化 tokenizer
func init() {
	// 使用 cl100k_base 编码（GPT-3.5/GPT-4 使用）
	var err error
	tokenizer, err = tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		// 如果初始化失败，使用简单的估算方法作为备选
		tokenizer = nil
	}
}

// CountTokens 计算文本的 token 数量
// 优先使用 tiktoken 精确计算，如果不可用则使用简单估算
func CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// 优先使用 tiktoken 精确计算
	if tokenizer != nil {
		return len(tokenizer.Encode(text, nil, nil))
	}

	// 备选方案：简单估算（英文按空格分词，中文按字分词）
	return estimateTokens(text)
}

// estimateTokens 简单估算 token 数量（备选方案）
func estimateTokens(text string) int {
	count := 0
	runes := []rune(text)
	i := 0

	for i < len(runes) {
		r := runes[i]

		// 中文字符
		if r >= 0x4E00 && r <= 0x9FFF {
			count++
			i++
			continue
		}

		// 英文单词
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			// 读取完整单词
			for i < len(runes) && ((runes[i] >= 'a' && runes[i] <= 'z') ||
				(runes[i] >= 'A' && runes[i] <= 'Z') || (runes[i] >= '0' && runes[i] <= '9')) {
				i++
			}
			count++
			continue
		}

		// 其他字符（标点、空格等）
		count++
		i++
	}

	return count
}

// CountMessageTokens 计算消息列表的 token 总数
// 包含消息固定开销（角色、分隔符等）
func CountMessageTokens(messages []*schema.Message) int {
	total := 0
	for _, msg := range messages {
		// 每条消息有固定开销（角色、分隔符等）
		total += 4  // 消息固定开销
		total += CountTokens(msg.Content)
	}
	total += 2  // 对话结尾标记
	return total
}

// TrimMessages 根据 token 限制裁剪消息列表
// 保留系统消息和最近的消息，确保不超出 token 限制
func TrimMessages(messages []*schema.Message, maxTokens int) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	// 分离系统消息和对话消息
	var systemMessages []*schema.Message
	var chatMessages []*schema.Message

	for _, msg := range messages {
		if msg.Role == schema.System {
			systemMessages = append(systemMessages, msg)
		} else {
			chatMessages = append(chatMessages, msg)
		}
	}

	// 计算系统消息的 token
	systemTokens := CountMessageTokens(systemMessages)

	// 如果系统消息已经超限，安全处理边界情况
	if systemTokens >= maxTokens {
		if len(systemMessages) > 0 {
			return []*schema.Message{systemMessages[0]}
		}
		return []*schema.Message{}
	}

	// 从最新消息开始保留，直到达到 token 限制
	remainingTokens := maxTokens - systemTokens
	var reversedChat []*schema.Message

	for i := len(chatMessages) - 1; i >= 0; i-- {
		msgTokens := CountMessageTokens([]*schema.Message{chatMessages[i]})
		if remainingTokens-msgTokens < 0 {
			break
		}
		remainingTokens -= msgTokens
		reversedChat = append(reversedChat, chatMessages[i])
	}

	// 翻转回正确的时间顺序
	trimmedChat := make([]*schema.Message, len(reversedChat))
	for i, msg := range reversedChat {
		trimmedChat[len(reversedChat)-1-i] = msg
	}

	// 合并系统消息和裁剪后的对话消息
	result := make([]*schema.Message, 0, len(systemMessages)+len(trimmedChat))
	result = append(result, systemMessages...)
	result = append(result, trimmedChat...)

	return result
}
