package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type traceRepository struct {
	db *gorm.DB
}

// NewTraceRepository 创建 trace 数据访问实现（实现 trace.LegacyTraceRepository）
func NewTraceRepository(db *gorm.DB) trace.LegacyTraceRepository {
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

func (r *traceRepository) ListTraces(ctx context.Context, filter trace.LegacyTraceFilter) ([]*entity.AgentTrace, int64, error) {
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

func (r *traceRepository) MarkTimeoutTraces(ctx context.Context, before time.Time) (int64, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&entity.AgentTrace{}).
		Where("status = ? AND started_at < ?", entity.TraceStatusRunning, before).
		Updates(map[string]any{
			"status":        entity.TraceStatusTimeout,
			"ended_at":      now,
			"duration_ms":   gorm.Expr("EXTRACT(EPOCH FROM (? - started_at)) * 1000", now),
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

// —— 新表 trace_records / trace_spans 的 Repository 实现 ——

// traceSpanRepo 实现新表 trace.TraceRepository 接口。
// 与旧 traceRepository 分离，避免方法签名冲突（CreateTrace/GetTrace/ListTraces 等同名但不同类型）。
type traceSpanRepo struct {
	db *gorm.DB
}

func mergeTraceAttributes(start, end entity.JSON) entity.JSON {
	if len(start) == 0 && len(end) == 0 {
		return nil
	}
	merged := make(entity.JSON, len(start)+len(end))
	for key, value := range start {
		merged[key] = value
	}
	for key, value := range end {
		merged[key] = value
	}
	return merged
}

func traceInt64(value any) int64 {
	switch number := value.(type) {
	case int:
		return int64(number)
	case int32:
		return int64(number)
	case int64:
		return number
	case float64:
		return int64(number)
	case json.Number:
		parsed, _ := number.Int64()
		return parsed
	default:
		return 0
	}
}

func queueWaitMilliseconds(attributes entity.JSON) int64 {
	if attributes == nil {
		return 0
	}
	if value, ok := attributes["queue_wait_ms"]; ok {
		return traceInt64(value)
	}
	return traceInt64(attributes["queue.wait_ms"])
}

// NewTraceSpanRepository 创建新表 trace 数据访问实现（实现 trace.TraceRepository）
func NewTraceSpanRepository(db *gorm.DB) trace.TraceRepository {
	return &traceSpanRepo{db: db}
}

// CreateTrace 创建 TraceRecord（冲突忽略：同一 TraceID 不覆盖）
func (r *traceSpanRepo) CreateTrace(ctx context.Context, record *entity.TraceRecord) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(record).Error
}

// UpdateTraceMetadata 只补全调用方提供的非空字段，避免恢复请求清空原问题摘要。
func (r *traceSpanRepo) UpdateTraceMetadata(ctx context.Context, traceID string, meta trace.TraceMetadata) error {
	updates := map[string]any{}
	if meta.ConversationID > 0 {
		updates["conversation_id"] = meta.ConversationID
	}
	if meta.QuerySummary != "" {
		updates["query_summary"] = meta.QuerySummary
	}
	if meta.EntryType != "" {
		updates["entry_type"] = meta.EntryType
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.TraceRecord{}).
		Where("trace_id = ?", traceID).
		Updates(updates).Error
}

// StartSpan 创建 Span（冲突忽略：同一 SpanID 不覆盖）
func (r *traceSpanRepo) StartSpan(ctx context.Context, span *entity.TraceSpan) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(span).Error
}

// EndSpan 幂等结束 Span：仅 status=running 时更新，第一个终态胜出。
// duration_ms 由数据库中的 started_at 与 end.EndedAt 计算，不信任调用方传入负数耗时。
func (r *traceSpanRepo) EndSpan(ctx context.Context, spanID string, end trace.SpanEnd) error {
	// 先读取 started_at 用于计算 duration_ms
	var span entity.TraceSpan
	if err := r.db.WithContext(ctx).Select("started_at", "attributes").Where("span_id = ?", spanID).First(&span).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // Span 不存在（可能 StartSpan 被丢弃），静默
		}
		return err
	}
	endedAt := end.EndedAt
	if endedAt.IsZero() {
		endedAt = time.Now()
	}
	durationMs := endedAt.Sub(span.StartedAt).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	updates := map[string]any{
		"status":           end.Status,
		"ended_at":         endedAt,
		"duration_ms":      durationMs,
		"output_summary":   end.OutputSummary,
		"error_type":       end.ErrorType,
		"error_message":    end.ErrorMessage,
		"tokens_in":        end.TokensIn,
		"tokens_out":       end.TokensOut,
		"reasoning_tokens": end.ReasoningTokens,
	}
	if mergedAttributes := mergeTraceAttributes(span.Attributes, end.Attributes); mergedAttributes != nil {
		updates["attributes"] = mergedAttributes
	}
	// WHERE status = running 保证第一个终态胜出（幂等）
	result := r.db.WithContext(ctx).
		Model(&entity.TraceSpan{}).
		Where("span_id = ? AND status = ?", spanID, trace.SpanStatusRunning).
		Updates(updates)
	return result.Error
}

// GetTrace 查询 TraceRecord
func (r *traceSpanRepo) GetTrace(ctx context.Context, traceID string) (*entity.TraceRecord, error) {
	var rec entity.TraceRecord
	err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// ListTraces 列表查询：只返回含 agent.run Span 的 Trace，聚合 Token/调用次数/首个错误。
// BusinessRunStatus 留空，由 Service 层从 agent_runs 表补充。
func (r *traceSpanRepo) ListTraces(ctx context.Context, filter trace.TraceFilter) ([]trace.TraceSummary, int64, error) {
	latestAgentSpans := func() *gorm.DB {
		return r.db.WithContext(ctx).
			Table("trace_spans AS latest_source").
			Select("DISTINCT ON (latest_source.trace_id) latest_source.trace_id, latest_source.run_id, latest_source.status AS agent_status").
			Where("latest_source.operation = ?", "agent.run").
			Order("latest_source.trace_id, latest_source.started_at DESC")
	}
	// 基础查询：trace_records 中存在 agent.run span 的 trace
	agentRunSubquery := r.db.WithContext(ctx).
		Model(&entity.TraceSpan{}).
		Select("DISTINCT trace_id").
		Where("operation = ?", "agent.run")

	q := r.db.WithContext(ctx).
		Model(&entity.TraceRecord{}).
		Where("trace_id IN (?)", agentRunSubquery)

	// 筛选条件
	if filter.Keyword != "" {
		q = q.Where("query_summary ILIKE ?", "%"+filter.Keyword+"%")
	}
	if filter.AgentSlug != "" {
		// agent_slug 在 trace_spans 的 attributes->>'agent_slug'，需要子查询
		agentSlugSubquery := r.db.WithContext(ctx).
			Model(&entity.TraceSpan{}).
			Select("DISTINCT trace_id").
			Where("operation = ? AND attributes->>'agent_slug' = ?", "agent.run", filter.AgentSlug)
		q = q.Where("trace_id IN (?)", agentSlugSubquery)
	}
	if filter.ConversationID > 0 {
		q = q.Where("conversation_id = ?", filter.ConversationID)
	}
	if filter.Model != "" {
		// 存在 display_name = filter.Model 的 LLM span
		modelSubquery := r.db.WithContext(ctx).
			Model(&entity.TraceSpan{}).
			Select("DISTINCT trace_id").
			Where("kind = ? AND display_name = ?", entity.SpanKindLLM, filter.Model)
		q = q.Where("trace_id IN (?)", modelSubquery)
	}
	if filter.Tool != "" {
		// 存在 display_name = filter.Tool 的 Tool span
		toolSubquery := r.db.WithContext(ctx).
			Model(&entity.TraceSpan{}).
			Select("DISTINCT trace_id").
			Where("kind = ? AND display_name = ?", entity.SpanKindTool, filter.Tool)
		q = q.Where("trace_id IN (?)", toolSubquery)
	}
	if filter.Status != "" {
		statusSubquery := r.db.WithContext(ctx).
			Table("(?) AS latest_agent", latestAgentSpans()).
			Select("latest_agent.trace_id").
			Where("latest_agent.agent_status = ?", filter.Status)
		q = q.Where("trace_id IN (?)", statusSubquery)
	}
	if filter.ErrorOnly {
		errorSubquery := r.db.WithContext(ctx).
			Model(&entity.TraceSpan{}).
			Select("DISTINCT trace_id").
			Where("status = ? AND error_message <> ''", trace.SpanStatusError)
		q = q.Where("trace_id IN (?)", errorSubquery)
	}
	if filter.MismatchOnly {
		mismatchSubquery := r.db.WithContext(ctx).
			Table("(?) AS latest_agent", latestAgentSpans()).
			Select("latest_agent.trace_id").
			Joins("JOIN agent_runs AS business_run ON business_run.id = latest_agent.run_id").
			Where("latest_agent.agent_status <> '' AND business_run.status <> ''").
			Where("latest_agent.agent_status IN ?", []string{
				trace.SpanStatusOK,
				trace.SpanStatusError,
				trace.SpanStatusCancelled,
				trace.SpanStatusInterrupted,
				trace.SpanStatusRunning,
				trace.SpanStatusTimeout,
			}).
			Where(`NOT (
				(latest_agent.agent_status = ? AND business_run.status = ?) OR
				(latest_agent.agent_status = ? AND business_run.status = ?) OR
				(latest_agent.agent_status = ? AND business_run.status = ?) OR
				(latest_agent.agent_status = ? AND business_run.status = ?) OR
				(latest_agent.agent_status = ? AND business_run.status = ?) OR
				(latest_agent.agent_status = ? AND business_run.status = ?) OR
				(latest_agent.agent_status = ? AND business_run.status IN (?, ?)) OR
				(latest_agent.agent_status = ? AND business_run.status = ?)
			)`,
				trace.SpanStatusOK, entity.StatusCompleted,
				trace.SpanStatusError, entity.StatusFailed,
				// cancelled / interrupted 互认（用户手动中断：span 常落成 cancelled，业务 run 是 interrupted）
				trace.SpanStatusCancelled, entity.StatusCancelled,
				trace.SpanStatusCancelled, entity.StatusInterrupted,
				trace.SpanStatusInterrupted, entity.StatusInterrupted,
				trace.SpanStatusInterrupted, entity.StatusCancelled,
				trace.SpanStatusRunning, entity.StatusPending, entity.StatusRunning,
				// timeout 仅与 failed 互认；span 超时但 run 被取消属于异常，应判为不一致
				trace.SpanStatusTimeout, entity.StatusFailed,
			)
		q = q.Where("trace_id IN (?)", mismatchSubquery)
	}
	if !filter.From.IsZero() {
		q = q.Where("created_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		q = q.Where("created_at <= ?", filter.To)
	}

	// 计算总数
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	page, pageSize := filter.Page, filter.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	// 查询 trace_id 列表（按 created_at DESC）
	var traceIDs []string
	if err := q.Order("created_at DESC").
		Offset((page-1)*pageSize).
		Limit(pageSize).
		Pluck("trace_id", &traceIDs).Error; err != nil {
		return nil, 0, err
	}
	if len(traceIDs) == 0 {
		return []trace.TraceSummary{}, total, nil
	}

	// 批量取出本页 trace 的 records 与 spans，避免逐条 trace 查询的 N+1
	var records []entity.TraceRecord
	if err := r.db.WithContext(ctx).Where("trace_id IN ?", traceIDs).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	recByID := make(map[string]*entity.TraceRecord, len(records))
	for i := range records {
		recByID[records[i].TraceID] = &records[i]
	}

	var allSpans []entity.TraceSpan
	if err := r.db.WithContext(ctx).Where("trace_id IN ?", traceIDs).Order("started_at ASC").Find(&allSpans).Error; err != nil {
		return nil, 0, err
	}
	spansByID := make(map[string][]*entity.TraceSpan, len(traceIDs))
	for i := range allSpans {
		tid := allSpans[i].TraceID
		spansByID[tid] = append(spansByID[tid], &allSpans[i])
	}

	// 内存聚合（与原 buildSummary 逻辑一致），保持 traceIDs 的顺序（created_at DESC）
	summaries := make([]trace.TraceSummary, 0, len(traceIDs))
	for _, tid := range traceIDs {
		rec, ok := recByID[tid]
		if !ok {
			continue
		}
		summaries = append(summaries, r.buildSummaryFromRecord(rec, spansByID[tid]))
	}
	return summaries, total, nil
}

// buildSummaryFromRecord 由已查询的 TraceRecord 与 spans 在内存中聚合出列表摘要。
// 抽离为纯函数，供 ListTraces 批量取数后复用，消除逐条 trace 查询导致的 N+1。
func (r *traceSpanRepo) buildSummaryFromRecord(rec *entity.TraceRecord, spans []*entity.TraceSpan) trace.TraceSummary {
	summary := trace.TraceSummary{
		TraceID:      rec.TraceID,
		RequestID:    rec.RequestID,
		QuerySummary: rec.QuerySummary,
		StartedAt:    rec.CreatedAt,
	}

	var minStarted, maxEnded *time.Time
	for _, s := range spans {
		// agent.run span 信息
		if s.Operation == "agent.run" {
			summary.RunID = s.RunID
			summary.AgentStatus = s.Status
			if s.DisplayName != "" {
				summary.AgentSlug = s.DisplayName
			}
		}
		// queue.consume span 的 queue.wait_ms
		if s.Operation == "queue.consume" && s.Attributes != nil {
			summary.QueueWaitMs = queueWaitMilliseconds(s.Attributes)
		}
		// 计数
		switch s.Kind {
		case "llm":
			summary.LLMCount++
			summary.TotalTokens += s.TokensIn + s.TokensOut
		case "tool":
			summary.ToolCount++
		}
		// 首个错误
		if s.Status == trace.SpanStatusError && s.ErrorMessage != "" && summary.FirstError == "" {
			summary.FirstError = s.ErrorMessage
		}
		// 计算总耗时
		if minStarted == nil || s.StartedAt.Before(*minStarted) {
			t := s.StartedAt
			minStarted = &t
		}
		if s.EndedAt != nil {
			if maxEnded == nil || s.EndedAt.After(*maxEnded) {
				t := *s.EndedAt
				maxEnded = &t
			}
		}
	}
	if minStarted != nil && maxEnded != nil {
		summary.DurationMs = maxEnded.Sub(*minStarted).Milliseconds()
	}
	return summary
}

// ListSpans 查询某 Trace 的所有 Span（按 started_at 排序）
func (r *traceSpanRepo) ListSpans(ctx context.Context, traceID string) ([]*entity.TraceSpan, error) {
	var spans []*entity.TraceSpan
	err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("started_at ASC").
		Find(&spans).Error
	return spans, err
}

// GetSpan 查询单条 Span 完整记录（含 attributes），供详情页按需懒加载。
func (r *traceSpanRepo) GetSpan(ctx context.Context, spanID string) (*entity.TraceSpan, error) {
	var span entity.TraceSpan
	err := r.db.WithContext(ctx).Where("span_id = ?", spanID).First(&span).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &span, nil
}

// GetTraceIDByRunID 通过 run_id 反查 trace_id（从 agent.run span 查询）
func (r *traceSpanRepo) GetTraceIDByRunID(ctx context.Context, runID string) (string, error) {
	if runID == "" {
		return "", nil
	}
	var span entity.TraceSpan
	err := r.db.WithContext(ctx).
		Select("trace_id").
		Where("run_id = ? AND operation = ?", runID, "agent.run").
		Order("started_at DESC"). // 多次重试取最近一次
		First(&span).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return span.TraceID, nil
}

// GetAgentRunSpan 通过 run_id 查询最近一次 agent.run Span，供恢复/重试关联原 Trace。
func (r *traceSpanRepo) GetAgentRunSpan(ctx context.Context, runID string) (*trace.RunSpanRef, error) {
	if runID == "" {
		return nil, nil
	}
	var span entity.TraceSpan
	err := r.db.WithContext(ctx).
		Where("run_id = ? AND operation = ?", runID, "agent.run").
		Order("started_at DESC").
		First(&span).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	attempt := 1
	if span.Attributes != nil {
		switch value := span.Attributes["attempt"].(type) {
		case int:
			attempt = value
		case int64:
			attempt = int(value)
		case float64:
			attempt = int(value)
		case json.Number:
			if parsed, parseErr := value.Int64(); parseErr == nil {
				attempt = int(parsed)
			}
		}
	}
	if attempt < 1 {
		attempt = 1
	}
	return &trace.RunSpanRef{TraceID: span.TraceID, SpanID: span.SpanID, Attempt: attempt}, nil
}

// MarkTimeoutSpans 将超时 running Span 标记为 timeout（新表）
func (r *traceSpanRepo) MarkTimeoutSpans(ctx context.Context, before time.Time) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&entity.TraceSpan{}).
		Where("status = ? AND started_at < ?", trace.SpanStatusRunning, before).
		Updates(map[string]any{
			"status":        trace.SpanStatusTimeout,
			"ended_at":      now,
			"duration_ms":   gorm.Expr("EXTRACT(EPOCH FROM (? - started_at)) * 1000", now),
			"error_type":    "timeout",
			"error_message": "执行超时（超过限制时长未完成）",
		})
	return result.RowsAffected, result.Error
}

// DeleteExpired 删除过期 Trace 数据（先删 Span 再删 Record）
func (r *traceSpanRepo) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	var traceIDs []string
	if err := r.db.WithContext(ctx).
		Model(&entity.TraceRecord{}).
		Where("expires_at < ?", before).
		Pluck("trace_id", &traceIDs).Error; err != nil {
		return 0, err
	}
	if len(traceIDs) == 0 {
		return 0, nil
	}
	// 先删 spans 再删 records
	if err := r.db.WithContext(ctx).Where("trace_id IN ?", traceIDs).Delete(&entity.TraceSpan{}).Error; err != nil {
		return 0, err
	}
	result := r.db.WithContext(ctx).Where("trace_id IN ?", traceIDs).Delete(&entity.TraceRecord{})
	return result.RowsAffected, result.Error
}
