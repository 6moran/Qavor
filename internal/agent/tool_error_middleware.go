package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// WrapToolError 包装 InvokableToolEndpoint，将非 interrupt 的 error 转成 tool result 喂回 LLM。
// 分流规则：
//   - compose.IsInterruptRerunError(err) → 原样上抛（审批中断，eino 会触发 checkpoint）
//   - secErr != nil && errors.Is(err, secErr) → 返回脱敏中文 ToolOutput（无路径泄露）
//   - 其他 → 返回 "错误: <原文>" ToolOutput
func WrapToolError(
	endpoint compose.InvokableToolEndpoint,
	secErr error,
) compose.InvokableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		out, err := endpoint(ctx, input)
		if err == nil {
			return out, nil
		}
		// interrupt 信号（审批中断或 resume 重定向）：原样上抛，eino 触发 checkpoint
		if _, ok := compose.IsInterruptRerunError(err); ok {
			return nil, err
		}
		// 安全策略拒绝：脱敏中文，不含路径
		if secErr != nil && errors.Is(err, secErr) {
			return &compose.ToolOutput{Result: "路径不在允许的工作空间范围内"}, nil
		}
		// 业务错误：中文包装 + 原文
		return &compose.ToolOutput{Result: fmt.Sprintf("错误: %s", err.Error())}, nil
	}
}

// WrapStreamToolError 包装 StreamableToolEndpoint，逻辑与 WrapToolError 一致。
func WrapStreamToolError(
	endpoint compose.StreamableToolEndpoint,
	secErr error,
) compose.StreamableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
		out, err := endpoint(ctx, input)
		if err == nil {
			return out, nil
		}
		if _, ok := compose.IsInterruptRerunError(err); ok {
			return nil, err
		}
		if secErr != nil && errors.Is(err, secErr) {
			return &compose.StreamToolOutput{
				Result: streamFromText("路径不在允许的工作空间范围内"),
			}, nil
		}
		return &compose.StreamToolOutput{
			Result: streamFromText(fmt.Sprintf("错误: %s", err.Error())),
		}, nil
	}
}

// streamFromText 将单段文本包装为 StreamReader[string]。
func streamFromText(text string) *schema.StreamReader[string] {
	return schema.StreamReaderFromArray([]string{text})
}
