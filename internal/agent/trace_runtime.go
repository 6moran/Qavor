package agent

import (
	"context"
	"errors"
	"fmt"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"github.com/cloudwego/eino/adk"
	"github.com/google/uuid"
)

// runMetaFromContextKey 用于在 ctx 中标记已注入 RunMeta（避免重复注入覆盖 Worker 信息）
type runMetaInjectedKey struct{}

// buildRunMeta 构造 RunMeta。
// 优先使用 Worker 通过 ctx 注入的 RunMeta（run_id/request_id/conversation_id/mode/attempt），
// 缺失字段用 Agent 配置和调用参数补齐；同步调用无 run_id 时生成 UUID。
// 不得覆盖 Worker 提供的 run_id。
func buildRunMeta(ctx context.Context, cfg *AgentConfig, query, mode string) trace.RunMeta {
	// 1. 读取 Worker 注入的 RunMeta（异步路径优先）
	if existing, ok := trace.RunMetaFromContext(ctx); ok {
		// Worker 已注入：补齐缺失的 agent_slug/query
		meta := existing
		if meta.AgentSlug == "" && cfg != nil {
			meta.AgentSlug = cfg.Slug
		}
		if meta.Query == "" {
			meta.Query = query
		}
		if meta.Mode == "" {
			meta.Mode = mode
		}
		return meta
	}

	// 2. 同步路径：从 ctx 中的 SpanContext 读取 request_id/run_id（若有），否则生成
	meta := trace.RunMeta{
		AgentSlug: "",
		Query:     query,
		Mode:      mode,
		Attempt:   1,
	}
	if cfg != nil {
		meta.AgentSlug = cfg.Slug
	}
	if sc, ok := trace.SpanContextFromContext(ctx); ok {
		meta.RequestID = sc.RequestID
		meta.RunID = sc.RunID
	}
	if meta.RunID == "" {
		meta.RunID = uuid.New().String()
	}
	return meta
}

// Run 包装同步 Agent 执行，统一管理 agent.run Span 生命周期。
// execute 在 Span context 下执行；返回的 error 原样返回给调用方。
// panic 先记录 error Span 再原样 panic，不吞掉 panic。
// Tracer 为 nil 时仅执行 execute，不创建任何 Span。
func (r *AgentRuntime) Run(ctx context.Context, meta trace.RunMeta, execute func(context.Context) error) (err error) {
	if r == nil || r.Tracer == nil {
		return execute(ctx)
	}

	runCtx, span := r.StartRun(ctx, meta)
	defer func() {
		if recovered := recover(); recovered != nil {
			span.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorType:    "panic",
				ErrorMessage: fmt.Sprint(recovered),
			})
			panic(recovered) // 不吞掉 panic
		}
		EndRunFromError(span, err, runCtx.Err())
	}()

	err = execute(runCtx)
	return err
}

// StartRun 创建 agent.run Span 并返回注入 SpanContext 和 RunMeta 的 context。
// 调用方负责在合适时机调用 span.End（通常通过 EndRunFromError 或 tracedIterator）。
// Tracer 为 nil 或未启用时返回原 ctx 和 no-op Span。
func (r *AgentRuntime) StartRun(ctx context.Context, meta trace.RunMeta) (context.Context, *trace.Span) {
	if r == nil || r.Tracer == nil {
		// 仍注入 RunMeta，供下游 Callback 读取
		return injectRunMeta(ctx, meta), trace.NoopSpan()
	}

	// 从 ctx 中读取父 SpanContext（HTTP/queue.consume）
	spec := trace.SpanSpec{
		Kind:         "agent",
		Operation:    "agent.run",
		DisplayName:  meta.AgentSlug,
		RunID:        meta.RunID,
		InputSummary: truncateQuery(meta.Query, r.Tracer.MaxContentLength()),
		Attributes: entity.JSON{
			"agent_slug": meta.AgentSlug,
			"mode":       meta.Mode,
		},
	}
	if meta.RequestID != "" {
		spec.RequestID = meta.RequestID
	}
	if meta.Attempt > 0 {
		if spec.Attributes == nil {
			spec.Attributes = entity.JSON{}
		}
		spec.Attributes["attempt"] = meta.Attempt
	}
	if meta.ConversationID != 0 {
		if spec.Attributes == nil {
			spec.Attributes = entity.JSON{}
		}
		spec.Attributes["conversation_id"] = meta.ConversationID
	}
	if meta.ResumeFromRunID != "" {
		spec.Attributes["resume_from_run_id"] = meta.ResumeFromRunID
	}
	if meta.ResumeFromSpanID != "" {
		spec.Attributes["resume_from_span_id"] = meta.ResumeFromSpanID
	}

	newCtx, span := r.Tracer.StartSpan(ctx, spec)
	// 注入 RunMeta 供 eino Callback 读取 run_id 等信息
	newCtx = injectRunMeta(newCtx, meta)
	return newCtx, span
}

// EndRunFromError 根据 execute 的错误和 ctx 取消状态结束 Span。
// 幂等：多次调用只第一次生效（Span 内部 sync.Once 保证）。
// 状态映射规则：
//   - err == nil 且 ctx 未取消 -> ok
//   - err == nil 且 ctx 已取消 -> cancelled
//   - errors.Is(err, context.Canceled) -> cancelled
//   - errors.Is(err, context.DeadlineExceeded) -> cancelled（按取消处理）
//   - 其他 err -> error
func EndRunFromError(span *trace.Span, err error, ctxErr error) {
	if span == nil {
		return
	}
	if err == nil && ctxErr == nil {
		span.End(trace.SpanEnd{Status: trace.SpanStatusOK})
		return
	}
	if err == nil && ctxErr != nil {
		// ctx 已取消但 execute 未返回 error（罕见）：按 cancelled
		span.End(trace.SpanEnd{
			Status:       trace.SpanStatusCancelled,
			ErrorMessage: ctxErr.Error(),
		})
		return
	}
	// err != nil
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		span.End(trace.SpanEnd{
			Status:       trace.SpanStatusCancelled,
			ErrorMessage: err.Error(),
		})
		return
	}
	span.End(trace.SpanEnd{
		Status:       trace.SpanStatusError,
		ErrorMessage: err.Error(),
	})
}

// injectRunMeta 注入 RunMeta，使用 marker key 防止重复注入覆盖
func injectRunMeta(ctx context.Context, meta trace.RunMeta) context.Context {
	if _, ok := ctx.Value(runMetaInjectedKey{}).(struct{}); ok {
		return ctx // 已注入，不覆盖
	}
	ctx = context.WithValue(ctx, runMetaInjectedKey{}, struct{}{})
	return trace.WithRunMeta(ctx, meta)
}

// truncateQuery 截断 query（rune 安全）
func truncateQuery(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// —— 流式迭代器包装 ——

// eventIterator 内部迭代器接口（兼容 *adk.AsyncIterator 和测试 fake）
type eventIterator interface {
	Next() (*adk.AgentEvent, bool)
}

// tracedIterator 包装底层事件迭代器，在真正耗尽/错误/取消时结束 agent.run Span。
// 一次 Agent 执行只有一个 agent.run Span，且只结束一次（依赖 Span 的 sync.Once）。
type tracedIterator struct {
	inner eventIterator
	span  *trace.Span
	ctx   context.Context
}

// newTracedIterator 构造带 Span 管理的迭代器包装。
// inner 可为 *adk.AsyncIterator[*adk.AgentEvent] 或任何实现 eventIterator 的类型。
// span 由 StartRun 创建；ctx 为 StartRun 返回的 context（包含 SpanContext）。
func newTracedIterator(ctx context.Context, span *trace.Span, inner eventIterator) *tracedIterator {
	return &tracedIterator{inner: inner, span: span, ctx: ctx}
}

// Next 读取下一个事件，并在终态时结束 Span。
// 终态判定规则（与计划 Task 5 Step 3 一致）：
//   - inner.Next() 返回 ok=false 且 ctx 未取消 -> ok
//   - inner.Next() 返回 event.Err -> error
//   - ctx 已取消 -> cancelled
//   - event.Action.Interrupted != nil -> interrupted
func (it *tracedIterator) Next() (*adk.AgentEvent, bool) {
	ev, ok := it.inner.Next()
	if !ok {
		// 迭代器耗尽：根据 ctx 状态决定终态
		if it.ctx.Err() != nil {
			it.span.End(trace.SpanEnd{
				Status:       trace.SpanStatusCancelled,
				ErrorMessage: it.ctx.Err().Error(),
			})
		} else {
			it.span.End(trace.SpanEnd{Status: trace.SpanStatusOK})
		}
		return nil, false
	}
	// 优先判断 interrupt（即使 ctx 取消，interrupt 也是更具体的终态）
	if ev != nil && ev.Action != nil && ev.Action.Interrupted != nil {
		it.span.End(trace.SpanEnd{
			Status:       trace.SpanStatusInterrupted,
			ErrorMessage: fmt.Sprintf("interrupted: %v", ev.Action.Interrupted.Data),
		})
		return ev, true
	}
	if ev != nil && ev.Err != nil {
		// event error：映射 cancelled / error
		if errors.Is(ev.Err, context.Canceled) || errors.Is(ev.Err, context.DeadlineExceeded) {
			it.span.End(trace.SpanEnd{
				Status:       trace.SpanStatusCancelled,
				ErrorMessage: ev.Err.Error(),
			})
		} else {
			it.span.End(trace.SpanEnd{
				Status:       trace.SpanStatusError,
				ErrorMessage: ev.Err.Error(),
			})
		}
		return ev, true
	}
	// 普通事件：ctx 已取消时也结束为 cancelled（避免流式取消后 Span 悬挂）
	if it.ctx.Err() != nil {
		it.span.End(trace.SpanEnd{
			Status:       trace.SpanStatusCancelled,
			ErrorMessage: it.ctx.Err().Error(),
		})
		return ev, false
	}
	return ev, true
}
