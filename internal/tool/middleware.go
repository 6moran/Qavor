package tool

import (
	"context"
	"errors"
)

// ApprovalRequiredError 审批请求错误
type ApprovalRequiredError struct {
	ToolName string
	Args     map[string]any
}

func (e *ApprovalRequiredError) Error() string {
	return "tool approval required: " + e.ToolName
}

// ToolApprovalMiddleware 工具审批中间件
type ToolApprovalMiddleware struct {
	approvalMode string // "always_trust" | "require_approval"
}

// NewToolApprovalMiddleware 创建工具审批中间件
func NewToolApprovalMiddleware(approvalMode string) *ToolApprovalMiddleware {
	return &ToolApprovalMiddleware{approvalMode: approvalMode}
}

// Handle 处理工具调用审批
func (m *ToolApprovalMiddleware) Handle(toolName string, args map[string]any) (bool, error) {
	if m.approvalMode == "always_trust" {
		return true, nil // 完全信任，不拦截
	}

	// 检查是否为敏感工具
	if SensitiveTools[toolName] {
		// 返回审批请求，等待用户确认
		return false, &ApprovalRequiredError{
			ToolName: toolName,
			Args:     args,
		}
	}

	return true, nil
}

// IsApprovalRequired 检查是否需要审批
func (m *ToolApprovalMiddleware) IsApprovalRequired(toolName string) bool {
	if m.approvalMode == "always_trust" {
		return false
	}
	return SensitiveTools[toolName]
}

// ToolExecutor 工具执行器接口
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, args map[string]any) (any, error)
}

// ApprovalHandler 审批处理器接口
type ApprovalHandler interface {
	// RequestApproval 请求用户审批
	RequestApproval(ctx context.Context, toolName string, args map[string]any) (bool, error)
}

// ExecWithApproval 带审批的工具执行
func ExecWithApproval(
	ctx context.Context,
	executor ToolExecutor,
	middleware *ToolApprovalMiddleware,
	handler ApprovalHandler,
	toolName string,
	args map[string]any,
) (any, error) {
	// 检查是否需要审批
	needsApproval, err := middleware.Handle(toolName, args)
	if err != nil {
		return nil, err
	}

	// 需要审批
	if !needsApproval {
		// 请求用户审批
		approved, err := handler.RequestApproval(ctx, toolName, args)
		if err != nil {
			return nil, err
		}
		if !approved {
			return nil, errors.New("user rejected tool execution")
		}
	}

	// 执行工具
	return executor.Execute(ctx, toolName, args)
}
