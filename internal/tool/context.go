package tool

import "context"

// kbIDsContextKey 是知识库范围在 context 中的私有键。
// 知识库 ID 只能由 Agent 运行上下文注入，LLM 无法通过工具参数指定。
type kbIDsContextKey struct{}

// WithKnowledgeBaseIDs 将知识库 ID 列表的防御性拷贝绑定到 context。
func WithKnowledgeBaseIDs(ctx context.Context, kbIDs []string) context.Context {
	return context.WithValue(ctx, kbIDsContextKey{}, append([]string(nil), kbIDs...))
}

// KnowledgeBaseIDsFromContext 读取知识库 ID 列表的防御性拷贝；未绑定时返回 nil。
func KnowledgeBaseIDsFromContext(ctx context.Context) []string {
	ids, _ := ctx.Value(kbIDsContextKey{}).([]string)
	return append([]string(nil), ids...)
}
