package repository

import (
	"context"
	"errors"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"gorm.io/gorm"
)

type traceRepository struct {
	db *gorm.DB
}

// NewTraceRepository 创建 trace 数据访问实现（实现 trace.TraceRepository）
func NewTraceRepository(db *gorm.DB) trace.TraceRepository {
	return &traceRepository{db: db}
}

func (r *traceRepository) CreateTrace(ctx context.Context, t *entity.AgentTrace) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *traceRepository) CreateSpan(ctx context.Context, s *entity.AgentTraceSpan) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *traceRepository) UpdateSpan(ctx context.Context, s *entity.AgentTraceSpan) error {
	return r.db.WithContext(ctx).Model(&entity.AgentTraceSpan{}).
		Where("trace_id = ? AND span_id = ?", s.TraceID, s.SpanID).
		Updates(map[string]any{
			"status":           s.Status,
			"ended_at":         s.EndedAt,
			"duration_ms":      s.DurationMs,
			"output_summary":   s.OutputSummary,
			"error_message":    s.ErrorMessage,
			"tokens_in":        s.TokensIn,
			"tokens_out":       s.TokensOut,
			"reasoning_tokens": s.ReasoningTokens,
		}).Error
}

func (r *traceRepository) GetTrace(ctx context.Context, traceID string) (*entity.AgentTrace, error) {
	var t entity.AgentTrace
	err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *traceRepository) ListTraces(ctx context.Context, filter trace.TraceFilter) ([]*entity.AgentTrace, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentTrace{})
	if filter.Keyword != "" {
		q = q.Where("query ILIKE ?", "%"+filter.Keyword+"%")
	}
	if filter.AgentSlug != "" {
		q = q.Where("agent_slug = ?", filter.AgentSlug)
	}
	if filter.ConversationID > 0 {
		q = q.Where("conversation_id = ?", filter.ConversationID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Source != "" {
		q = q.Where("source = ?", filter.Source)
	}
	if !filter.From.IsZero() {
		q = q.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		q = q.Where("created_at <= ?", filter.To)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := filter.Page, filter.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var items []*entity.AgentTrace
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *traceRepository) ListSpans(ctx context.Context, traceID string) ([]*entity.AgentTraceSpan, error) {
	var spans []*entity.AgentTraceSpan
	err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("started_at ASC, id ASC").
		Find(&spans).Error
	return spans, err
}

func (r *traceRepository) FinishTrace(ctx context.Context, traceID, status, errMsg string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t entity.AgentTrace
		if err := tx.Where("trace_id = ?", traceID).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 未采集到组件调用（无根记录），静默
			}
			return err
		}
		if t.Status != entity.TraceStatusRunning {
			return nil // 幂等：已收尾不再覆盖
		}
		// 聚合 Token 与最后完成的 LLM span
		var agg struct {
			In  int
			Out int
		}
		_ = tx.Model(&entity.AgentTraceSpan{}).
			Where("trace_id = ? AND kind = ?", traceID, entity.SpanKindLLM).
			Select("COALESCE(SUM(tokens_in),0) AS in, COALESCE(SUM(tokens_out),0) AS out").
			Scan(&agg).Error
		var lastLLM entity.AgentTraceSpan
		_ = tx.Where("trace_id = ? AND kind = ? AND status = ?", traceID, entity.SpanKindLLM, entity.SpanStatusSuccess).
			Order("ended_at DESC NULLS LAST, id DESC").First(&lastLLM).Error

		now := time.Now()
		updates := map[string]any{
			"status":        status,
			"error_message": errMsg,
			"ended_at":      now,
			"duration_ms":   now.Sub(t.StartedAt).Milliseconds(),
			"total_tokens":  agg.In + agg.Out,
		}
		if lastLLM.ID > 0 {
			updates["model_name"] = lastLLM.Name
		}
		return tx.Model(&entity.AgentTrace{}).Where("trace_id = ?", traceID).Updates(updates).Error
	})
}

func (r *traceRepository) MarkTimeoutTraces(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&entity.AgentTrace{}).
		Where("status = ? AND started_at < ?", entity.TraceStatusRunning, before).
		Updates(map[string]any{
			"status":        entity.TraceStatusTimeout,
			"ended_at":      time.Now(),
			"error_message": "执行超时（超过限制时长未完成）",
		})
	return res.RowsAffected, res.Error
}

func (r *traceRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Model(&entity.AgentTrace{}).
		Where("created_at < ?", before).Pluck("trace_id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	// 先删 spans 再删 traces（janitor 保证顺序）
	if err := r.db.WithContext(ctx).Where("trace_id IN ?", ids).Delete(&entity.AgentTraceSpan{}).Error; err != nil {
		return 0, err
	}
	res := r.db.WithContext(ctx).Where("trace_id IN ?", ids).Delete(&entity.AgentTrace{})
	return res.RowsAffected, res.Error
}
