package agent

import (
	"context"
	"errors"

	"Qavor/internal/tool"

	"github.com/cloudwego/eino/adk"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ErrToolRejected 用户拒绝工具执行。
var ErrToolRejected = errors.New("tool execution rejected by user")

// ApprovalModeAlwaysTrust 完全信任模式：敏感工具直接放行，不审批。
const ApprovalModeAlwaysTrust = "always_trust"

// ApprovalModeDefault 默认模式：敏感工具执行前需用户审批。
const ApprovalModeDefault = "default"

// approvalModeKey context key，用于在运行时传递审批模式（请求级）。
type approvalModeKey struct{}

// WithApprovalMode 把审批模式写入 ctx。mode 为 ApprovalModeAlwaysTrust / ApprovalModeDefault。
func WithApprovalMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, approvalModeKey{}, mode)
}

// ApprovalModeFrom 从 ctx 读取审批模式，缺省返回 default。
func ApprovalModeFrom(ctx context.Context) string {
	if v, ok := ctx.Value(approvalModeKey{}).(string); ok && v != "" {
		return v
	}
	return ApprovalModeDefault
}

// ApprovalRequest 审批请求信息（作为 tool.StatefulInterrupt 的 info 传给外层）。
type ApprovalRequest struct {
	ToolName string `json:"tool_name"`
	Args     string `json:"args"`
}

// approvalState 审批状态（StatefulInterrupt 的 state，恢复时读回）。
type approvalState struct {
	ToolName string
	Args     string
}

// checkpoint 使用 gob 编码，ApprovalRequest/approvalState 作为 interface 值存入
// InterruptSignal，必须注册具体类型，否则 gob 反序列化会报
// "type not registered for interface"。
func init() {
	schema.RegisterName[*ApprovalRequest]("agent.ApprovalRequest")
	schema.RegisterName[approvalState]("agent.approvalState")
}

// ApprovalMiddleware 工具审批中间件。
// 拦截敏感工具（SensitiveTools，如 execute）的执行：首次调用触发 tool.StatefulInterrupt，
// 暂停 agent 等待用户审批；恢复时按用户决定放行（approve）或拒绝（reject）。
// 同时实现 WrapInvokableToolCall（内置工具等 InvokableTool）与 WrapStreamableToolCall（execute 等 StreamableTool）。
// 实现 ChatModelAgentMiddlewareName 提供稳定名，避免中断地址与底层工具相撞。
// 审批模式从 ctx 读取（WithApprovalMode），支持请求级 mode（Agent 实例可缓存复用）。
type ApprovalMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.Message]
}

// NewApprovalMiddleware 创建审批中间件。
func NewApprovalMiddleware() *ApprovalMiddleware {
	return &ApprovalMiddleware{
		TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.Message]{},
	}
}

// ChatModelAgentMiddlewareName 返回稳定中间件名（中断地址用，不能与其他中间件冲突）。
func (m *ApprovalMiddleware) ChatModelAgentMiddlewareName() string {
	return "qavor_approval"
}

// sensitive 判断工具是否需审批（敏感工具且非 always_trust）。
func (m *ApprovalMiddleware) sensitive(ctx context.Context, tCtx *adk.ToolContext) bool {
	if ApprovalModeFrom(ctx) == ApprovalModeAlwaysTrust {
		return false
	}
	if tCtx == nil {
		return false
	}
	return tool.SensitiveTools[tCtx.Name]
}

// WrapInvokableToolCall 包装同步工具调用（内置工具等 InvokableTool）。
// sensitive 判断在运行时基于请求 ctx 的审批模式进行（Agent 实例可缓存复用）。
func (m *ApprovalMiddleware) WrapInvokableToolCall(
	_ context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
		// 非敏感工具（或 always_trust 模式）直接放行
		if !m.sensitive(ctx, tCtx) {
			return endpoint(ctx, argumentsInJSON, opts...)
		}
		// 被点名恢复：读用户决定
		isTarget, hasData, data := einotool.GetResumeContext[string](ctx)
		if isTarget {
			if hasData && data == "approve" {
				// 用户批准：真正执行
				return endpoint(ctx, argumentsInJSON, opts...)
			}
			// 用户拒绝
			return "", ErrToolRejected
		}
		// 首次触发：中断等待审批
		return "", einotool.StatefulInterrupt(ctx,
			&ApprovalRequest{ToolName: tCtx.Name, Args: argumentsInJSON},
			approvalState{ToolName: tCtx.Name, Args: argumentsInJSON})
	}, nil
}

// WrapStreamableToolCall 包装流式工具调用（execute 等 StreamableTool）。
// sensitive 判断在运行时基于请求 ctx 的审批模式进行（Agent 实例可缓存复用）。
func (m *ApprovalMiddleware) WrapStreamableToolCall(
	_ context.Context,
	endpoint adk.StreamableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.StreamableToolCallEndpoint, error) {
	return func(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (*schema.StreamReader[string], error) {
		// 非敏感工具（或 always_trust 模式）直接放行
		if !m.sensitive(ctx, tCtx) {
			return endpoint(ctx, argumentsInJSON, opts...)
		}
		isTarget, hasData, data := einotool.GetResumeContext[string](ctx)
		if isTarget {
			if hasData && data == "approve" {
				return endpoint(ctx, argumentsInJSON, opts...)
			}
			return nil, ErrToolRejected
		}
		// 首次触发：中断等待审批
		return nil, einotool.StatefulInterrupt(ctx,
			&ApprovalRequest{ToolName: tCtx.Name, Args: argumentsInJSON},
			approvalState{ToolName: tCtx.Name, Args: argumentsInJSON})
	}, nil
}
