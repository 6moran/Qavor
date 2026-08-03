package rag

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// SystemPrompt 约束模型只根据检索到的知识片段回答。
const SystemPrompt = `你是一个严谨的问答助手，请严格依据提供的"知识片段"回答用户问题。
当知识片段不足以回答问题时，请明确说明"根据当前知识库无法回答该问题"，不要编造答案。
回答中请使用 [1]、[2]… 这样的数字索引标注所依据的片段，索引顺序必须与"知识片段"的编号一致。
禁止输出任何文件ID、chunk_id、数据库主键等内部标识。`

// EmptyAnswer 无命中时的固定回答，直接返回，不调用 LLM。
const EmptyAnswer = "当前知识库中没有找到相关内容。"

// ChatTemplateUserContent 是 User 消息的 FString 模板，变量仅保留 query 和 context。
const ChatTemplateUserContent = `用户问题：{query}

知识片段：
{context}

请按系统要求作答，并在答案中用 [n] 标注引用。`

// NewRAGChatTemplate 构造 Eino prompt.ChatTemplate，变量只保留 query 和 context。
// 系统消息固定约束，用户消息通过 FString 渲染查询和检索上下文。
func NewRAGChatTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		&schema.Message{Role: schema.System, Content: SystemPrompt},
		&schema.Message{Role: schema.User, Content: ChatTemplateUserContent},
	)
}

// BuildContextFromDocs 将检索到的 schema.Document 列表拼接为模板的 context 变量。
// 序号由程序生成，模型不允许修改引用索引。
func BuildContextFromDocs(docs []*schema.Document) string {
	if len(docs) == 0 {
		return "（无可用知识片段）"
	}
	var b strings.Builder
	for i, d := range docs {
		filename := metaDataString(d, MetaKeyFilename)
		if filename == "" {
			filename = "未知文件"
		}
		idx := i + 1
		b.WriteString(fmt.Sprintf("[%d] 文件《%s》:\n%s\n\n", idx, filename, d.Content))
	}
	return strings.TrimRight(b.String(), "\n")
}
