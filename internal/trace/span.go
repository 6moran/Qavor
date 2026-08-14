package trace

import (
	"context"
	"sync"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Span 单个 Span 的句柄，持有创建时的上下文用于幂等结束
// 类似 OTel 的 Span，但简化了 API（没有 SetAttributes/SetName 等，直接在 End 时传入 SpanEnd）
// 设计要点：
//  - 幂等 End：用 sync.Once 保证只落库一次（agent.run span 可能被 defer 和 tracedIterator 两个路径调用）
//  - noop 标记：未启用或无父 SpanContext 时返回 noopSpan()，所有操作为空操作
//  - writeCtx：使用 context.WithoutCancel 防止业务取消导致 span 丢失
type Span struct {
	tracer   *Tracer          // 所属 Tracer（用于读取配置和调用 Writer）
	record   entity.TraceSpan // Span 实体（写入 DB 的数据）
	writeCtx context.Context  // 脱离取消信号的 context（保证 span 写入成功）
	once     sync.Once        // 幂等 End 的关键（只执行一次）
	noop     bool             // no-op 标记：true 时所有操作为空操作
}

// Record 返回 Span 的当前实体快照
func (s *Span) Record() entity.TraceSpan {
	return s.record
}

// End 幂等结束 Span：仅第一次调用生效，后续调用为 no-op
// 使用 context.WithoutCancel 保证业务 Context 取消后仍能保存 cancelled 状态
func (s *Span) End(end SpanEnd) {
	if s.noop {
		return
	}
	s.once.Do(func() {
		if end.EndedAt.IsZero() {
			end.EndedAt = time.Now()
		}
		if end.Status == "" {
			end.Status = SpanStatusOK
		}
		end.OutputSummary = s.tracer.sanitizeText(end.OutputSummary)
		end.ErrorMessage = s.tracer.sanitizeText(end.ErrorMessage)
		err := s.tracer.writer.EndSpan(context.WithoutCancel(s.writeCtx), s.record.SpanID, end)
		if err != nil {
			logger.Warn("trace: 结束 span 失败",
				zap.String("span_id", s.record.SpanID),
				zap.String("trace_id", s.record.TraceID),
				zap.String("operation", s.record.Operation),
				zap.Error(err))
		}
	})
}

// newSpan 创建真实 Span（写入 SpanWriter）
func newSpan(tracer *Tracer, writeCtx context.Context, record entity.TraceSpan) *Span {
	return &Span{
		tracer:   tracer,
		record:   record,
		writeCtx: writeCtx,
	}
}

// NoopSpan 返回不写入的 Span（Tracer 未启用或未注入父 SpanContext 时使用）
func NoopSpan() *Span {
	return &Span{noop: true}
}

// noopSpan 不写入的 Span（内部使用，保留旧名兼容已有代码）
func noopSpan() *Span {
	return NoopSpan()
}

// buildSpanRecord 从 SpanSpec 和父 SpanContext 构建实体
func buildSpanRecord(spec SpanSpec, parent SpanContext, ok bool) entity.TraceSpan {
	now := time.Now()
	rec := entity.TraceSpan{
		SpanID:       uuid.New().String(),
		Kind:         spec.Kind,
		Operation:    spec.Operation,
		DisplayName:  spec.DisplayName,
		Status:       SpanStatusRunning,
		StartedAt:    now,
		InputSummary: spec.InputSummary,
		Attributes:   spec.Attributes,
		CreatedAt:    now,
	}
	// TraceID 优先级：spec > parent
	if spec.TraceID != "" {
		rec.TraceID = spec.TraceID
	} else if ok {
		rec.TraceID = parent.TraceID
	}
	// ParentSpanID 优先级：spec > parent
	if spec.ParentSpanID != "" {
		rec.ParentSpanID = spec.ParentSpanID
	} else if ok {
		rec.ParentSpanID = parent.SpanID
	}
	if spec.RunID != "" {
		rec.RunID = spec.RunID
	} else if ok {
		rec.RunID = parent.RunID
	}
	if spec.RequestID != "" {
		rec.RequestID = spec.RequestID
	} else if ok {
		rec.RequestID = parent.RequestID
	}
	return rec
}
