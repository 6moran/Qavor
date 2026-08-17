package trace

import (
	"context"

	"Qavor/internal/model/entity"
)

// SpanHandle 手动 span 句柄，内部委托新架构的 Span。
// nil 句柄表示 trace 未启用或上下文缺失，调用方安全 nil-check 后 defer End 即可。
type SpanHandle struct {
	span *Span
}

// StartSpan 手动创建一个 span（适配 Tracer 实例架构）。
//
// 用法：
//
//	tracer := ... // app 装配的 *trace.Tracer，可为 nil
//	ctx, span := trace.StartSpan(tracer, ctx, entity.SpanKindContext, "FetchContext", "conv=123")
//	defer span.End(ctx, entity.SpanStatusSuccess, "12 msgs", "", 1024, 0)
//
// 当 tracer 为 nil、trace 未启用或 ctx 无父 SpanContext（未采样）时，返回原 ctx 和 nil。
// 调用方对返回的 *SpanHandle 做 nil-check 后再使用，nil 时 End 为 no-op。
//
// 父子关系：自动读取 ctx 中的 SpanContext（由 eino callback 或上一次 StartSpan 注入）作为 parent。
// name 同时作为新表的 operation 与 display_name。
func StartSpan(tracer *Tracer, ctx context.Context, kind, name, inputSummary string) (context.Context, *SpanHandle) {
	if tracer == nil {
		return ctx, nil
	}
	newCtx, span := tracer.StartSpan(ctx, SpanSpec{
		Kind:         kind,
		Operation:    name,
		DisplayName:  name,
		InputSummary: inputSummary,
	})
	if span.noop {
		return newCtx, nil
	}
	return newCtx, &SpanHandle{span: span}
}

// End 补全 span：写入 status / 输出摘要 / 错误 / token / 耗时。
// h 为 nil 时直接返回（no-op），便于 defer span.End(...) 无条件调用。
// ctx 参数保留仅为兼容旧调用形态，新架构 Span 内部持有写入上下文，无需传入。
// 旧表状态常量（entity.SpanStatusSuccess 等）自动映射到新表状态。
func (h *SpanHandle) End(_ context.Context, status, outputSummary, errMsg string, tokensIn, tokensOut int) {
	if h == nil || h.span == nil {
		return
	}
	h.span.End(SpanEnd{
		Status:        normalizeStatus(status),
		OutputSummary: outputSummary,
		ErrorMessage:  errMsg,
		TokensIn:      tokensIn,
		TokensOut:     tokensOut,
	})
}

// normalizeStatus 将旧表状态常量映射到新表状态：
// entity.SpanStatusSuccess("success") → SpanStatusOK("ok")，其余透传。
func normalizeStatus(status string) string {
	if status == entity.SpanStatusSuccess {
		return SpanStatusOK
	}
	return status
}
