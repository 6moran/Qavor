package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

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

// isFileNotFoundError 判断错误是否为“文件不存在”类错误。
// 知识库文档常被模型误当作 workspace 本地文件去 read_file，触发此类错误，
// 此时应引导模型改用 query_kb 检索，而不是反复尝试读取本地文件。
func isFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "cannot find the file")
}

// buildRecoveryResult 构造“可恢复”的工具结果 JSON，引导模型下一步修正。
func buildRecoveryResult(code, message string) string {
	result, _ := json.Marshal(map[string]any{
		"ok": false,
		"error": map[string]any{
			"code":        code,
			"message":     message,
			"recoverable": true,
			"retryable":   false,
			"suggested_actions": []string{
				"调用 query_kb 查询知识库",
				"使用查询结果中的 file_id 获取知识库内容",
			},
		},
	})
	return string(result)
}

// newToolErrorRecoveryMiddleware 将“文件不在 workspace”的工具失败转换为可恢复的工具结果，
// 引导模型改用 query_kb 查询知识库，而不是把知识库文档名传给 read_file。
//
// 注意：read_file 等文件系统工具在 eino 中走流式（Streamable）路径，因此这里同时实现
// Invokable 与 Streamable 两个分支，否则流式调用失败仍会抛出 NodeRunError。
func newToolErrorRecoveryMiddleware() compose.ToolMiddleware {
	const code = "WORKSPACE_FILE_NOT_FOUND"
	const message = "文件不在当前 Agent workspace 中；知识库文件请使用 query_kb 查询，不要把知识库文档名传给 read_file。"
	return compose.ToolMiddleware{
		Name: "tool_error_recovery",
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				out, err := next(ctx, input)
				_, interrupt := compose.IsInterruptRerunError(err)
				if err == nil || ctx.Err() != nil || interrupt {
					return out, err
				}
				if isFileNotFoundError(err) {
					return &compose.ToolOutput{Result: buildRecoveryResult(code, message)}, nil
				}
				return nil, err
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
				out, err := next(ctx, input)
				_, interrupt := compose.IsInterruptRerunError(err)
				if err == nil || ctx.Err() != nil || interrupt {
					return out, err
				}
				if isFileNotFoundError(err) {
					return &compose.StreamToolOutput{
						Result: streamFromText(buildRecoveryResult(code, message)),
					}, nil
				}
				return nil, err
			}
		},
	}
}
