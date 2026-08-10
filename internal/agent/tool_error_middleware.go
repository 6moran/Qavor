package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// newToolErrorRecoveryMiddleware 将普通工具失败转换为工具结果，
// 以便模型可以修正其下一步操作。取消和 Eino 控制信号仍然是真正的错误，
// 仍然会终止运行。
func newToolErrorRecoveryMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Name: "tool_error_recovery",
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				out, err := next(ctx, input)
				_, interrupt := compose.IsInterruptRerunError(err)
				if err == nil || ctx.Err() != nil || interrupt {
					return out, err
				}
				code := "TOOL_EXECUTION_FAILED"
				message := "工具执行失败，请根据错误信息修正参数或选择其他工具。"
				if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "does not exist") || strings.Contains(strings.ToLower(err.Error()), "no such file") {
					code = "WORKSPACE_FILE_NOT_FOUND"
					message = "文件不在当前 Agent workspace 中；知识库文件请使用 query_kb 查询，不要把知识库文档名传给 read_file。"
				}
				result, marshalErr := json.Marshal(map[string]any{
					"ok": false,
					"error": map[string]any{
						"code": code, "message": message, "recoverable": true,
						"retryable":         false,
						"suggested_actions": []string{"调用 query_kb 查询知识库", "使用查询结果中的 file_id 获取知识库内容"},
					},
				})
				if marshalErr != nil {
					return nil, fmt.Errorf("marshal tool recovery result: %w", marshalErr)
				}
				return &compose.ToolOutput{Result: string(result)}, nil
			}
		},
	}
}
