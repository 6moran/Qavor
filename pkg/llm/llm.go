// Package llm 提供与 Agent 对话无关的独立 LLM 调用能力。
//
// 项目中有多处"单独向模型发送请求"的场景（知识库描述润色、示例问题生成、
// 思维导图生成、RAG 评估等），它们与 Agent 对话走不同的链路，属于公共工具。
// 本包统一封装这类调用的超时控制与错误分类：
//   - Chat：执行一次单轮 User 对话调用并返回文本；
//   - ClassifyError：把底层错误归类为 超时 / 连接失败 / 余额不足 / 认证失败 /
//     模型不存在 / 限流，并提供可直接展示给用户的中文友好提示
//     （与模型连接测试的错误提示同源）。
package llm

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ChatModel 独立调用所需的模型最小接口（eino 的 ToolCallingChatModel 天然满足）。
type ChatModel interface {
	Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error)
}
