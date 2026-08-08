package service

import (
	"context"
	"fmt"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"
)

type traceServiceImpl struct {
	repo trace.TraceRepository
}

// NewTraceService 创建链路追踪服务
func NewTraceService(repo trace.TraceRepository) TraceService {
	return &traceServiceImpl{repo: repo}
}

func (s *traceServiceImpl) ListTraces(ctx context.Context, filter TraceListFilter) ([]TraceItem, int64, error) {
	items, total, err := s.repo.ListTraces(ctx, trace.TraceFilter{
		Keyword:        filter.Keyword,
		AgentSlug:      filter.AgentSlug,
		ConversationID: filter.ConversationID,
		Status:         filter.Status,
		Source:         filter.Source,
		From:           filter.From,
		To:             filter.To,
		Page:           filter.Page,
		PageSize:       filter.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]TraceItem, 0, len(items))
	for _, it := range items {
		out = append(out, toTraceItem(it))
	}
	return out, total, nil
}

func (s *traceServiceImpl) GetTraceDetail(ctx context.Context, traceID string) (*TraceDetail, error) {
	t, err := s.repo.GetTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("trace %s 不存在", traceID)
	}
	spans, err := s.repo.ListSpans(ctx, traceID)
	if err != nil {
		return nil, err
	}
	spanItems := make([]TraceSpanItem, 0, len(spans))
	for _, sp := range spans {
		spanItems = append(spanItems, TraceSpanItem{
			SpanID:          sp.SpanID,
			ParentSpanID:    sp.ParentSpanID,
			Kind:            sp.Kind,
			Name:            sp.Name,
			Status:          sp.Status,
			StartedAt:       sp.StartedAt,
			EndedAt:         sp.EndedAt,
			DurationMs:      sp.DurationMs,
			InputSummary:    sp.InputSummary,
			OutputSummary:   sp.OutputSummary,
			TokensIn:        sp.TokensIn,
			TokensOut:       sp.TokensOut,
			ReasoningTokens: sp.ReasoningTokens,
			ErrorMessage:    sp.ErrorMessage,
		})
	}
	return &TraceDetail{Trace: toTraceItem(t), Spans: spanItems}, nil
}

func toTraceItem(t *entity.AgentTrace) TraceItem {
	return TraceItem{
		TraceID:      t.TraceID,
		Source:       t.Source,
		AgentSlug:    t.AgentSlug,
		Query:        t.Query,
		Status:       t.Status,
		ErrorMessage: t.ErrorMessage,
		DurationMs:   t.DurationMs,
		ModelName:    t.ModelName,
		TotalTokens:  t.TotalTokens,
		StartedAt:    t.StartedAt,
		EndedAt:      t.EndedAt,
	}
}
