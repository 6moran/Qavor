package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"
	pkgerrors "Qavor/pkg/errors"
)

type traceServiceImpl struct {
	repo    trace.TraceRepository
	runRepo RunStatusReader
}

func stringSliceAttribute(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

// NewTraceService 创建链路追踪服务。
// repo: 新表 trace_records/trace_spans 的查询接口。
// runRepo: agent_runs 表读取接口，用于补充业务 Run 状态（可为 nil，则 BusinessRunStatus 留空）。
func NewTraceService(repo trace.TraceRepository, runRepo RunStatusReader) TraceService {
	return &traceServiceImpl{repo: repo, runRepo: runRepo}
}

func (s *traceServiceImpl) ListTraces(ctx context.Context, filter TraceListFilter) ([]TraceItem, int64, error) {
	summaries, total, err := s.repo.ListTraces(ctx, trace.TraceFilter{
		Keyword:        filter.Keyword,
		AgentSlug:      filter.AgentSlug,
		ConversationID: filter.ConversationID,
		Status:         filter.Status,
		Model:          filter.Model,
		Tool:           filter.Tool,
		ErrorOnly:      filter.ErrorOnly,
		MismatchOnly:   filter.MismatchOnly,
		From:           filter.From,
		To:             filter.To,
		Page:           filter.Page,
		PageSize:       filter.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	out := make([]TraceItem, 0, len(summaries))
	for _, sm := range summaries {
		// 补充 BusinessRunStatus：从 agent_runs 表读取
		businessStatus := sm.BusinessRunStatus
		if s.runRepo != nil && sm.RunID != "" {
			if run, rerr := s.runRepo.GetByID(sm.RunID); rerr == nil && run != nil {
				businessStatus = run.Status
			}
		}
		mismatch := isStatusMismatch(sm.AgentStatus, businessStatus)

		out = append(out, TraceItem{
			TraceID:           sm.TraceID,
			RunID:             sm.RunID,
			RequestID:         sm.RequestID,
			AgentSlug:         sm.AgentSlug,
			QuerySummary:      sm.QuerySummary,
			AgentStatus:       sm.AgentStatus,
			BusinessRunStatus: businessStatus,
			StatusMismatch:    mismatch,
			DurationMs:        sm.DurationMs,
			QueueWaitMs:       sm.QueueWaitMs,
			LLMCount:          sm.LLMCount,
			ToolCount:         sm.ToolCount,
			TotalTokens:       sm.TotalTokens,
			FirstError:        sm.FirstError,
			StartedAt:         sm.StartedAt,
		})
	}
	return out, total, nil
}

func (s *traceServiceImpl) GetTraceDetail(ctx context.Context, traceID string) (*TraceDetail, error) {
	rec, err := s.repo.GetTrace(ctx, traceID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, fmt.Sprintf("trace %s 不存在", traceID))
	}

	spans, err := s.repo.ListSpans(ctx, traceID)
	if err != nil {
		return nil, err
	}

	// 按 started_at 升序排序（保证稳定展示）
	sort.SliceStable(spans, func(i, j int) bool {
		return spans[i].StartedAt.Before(spans[j].StartedAt)
	})

	// 建立 tool_call_id → llm span_id 映射，为匹配的 Tool Span 增加 triggered_by_span_id
	toolCallToLLM := map[string]string{}
	for _, sp := range spans {
		if sp.Kind != entity.SpanKindLLM || sp.Attributes == nil {
			continue
		}
		for _, id := range stringSliceAttribute(sp.Attributes["tool_call_ids"]) {
			toolCallToLLM[id] = sp.SpanID
		}
	}

	spanItems := make([]TraceSpanItem, 0, len(spans))
	for _, sp := range spans {
		item := spanToItem(sp, "")
		// 为 Tool Span 关联触发它的 LLM Span（数据库 parent_span_id 保持不变）
		if sp.Kind == entity.SpanKindTool && sp.Attributes != nil {
			if tcID, ok := sp.Attributes["tool_call_id"].(string); ok && tcID != "" {
				if llmSpanID, ok := toolCallToLLM[tcID]; ok {
					item.TriggeredBySpanID = llmSpanID
				}
			}
		}
		// 列表不返回 Attributes 大字段，前端按需通过 GetSpanDetail 拉取完整详情
		item.Attributes = nil
		spanItems = append(spanItems, item)
	}

	// 组装 Run 摘要（从 agent.run span 提取 run_id 后查 agent_runs）
	var runSummary *TraceRunSummary
	if s.runRepo != nil {
		runID := extractRunID(spans)
		if runID != "" {
			if run, rerr := s.runRepo.GetByID(runID); rerr == nil && run != nil {
				runSummary = &TraceRunSummary{
					RunID:      run.ID,
					Status:     run.Status,
					ErrorType:  run.ErrorType,
					StartedAt:  run.StartedAt,
					FinishedAt: run.FinishedAt,
				}
			}
		}
	}

	diagnostics := buildDiagnostics(spans, runSummary)

	return &TraceDetail{
		Trace:       *rec,
		Run:         runSummary,
		Spans:       spanItems,
		SpanTotal:   int64(len(spans)),
		Diagnostics: diagnostics,
	}, nil
}

// GetSpanDetail 获取单条 Span 完整详情（含 attributes），供详情页按需懒加载。
func (s *traceServiceImpl) GetSpanDetail(ctx context.Context, spanID string) (*TraceSpanItem, error) {
	if spanID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeBadRequest, "span_id 不能为空")
	}
	sp, err := s.repo.GetSpan(ctx, spanID)
	if err != nil {
		return nil, err
	}
	if sp == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, fmt.Sprintf("span %s 不存在", spanID))
	}
	item := spanToItem(sp, "")
	return &item, nil
}

// spanToItem 将持久化 Span 转换为前端展示用的平铺项（含 attributes）。
func spanToItem(sp *entity.TraceSpan, triggeredBy string) TraceSpanItem {
	return TraceSpanItem{
		SpanID:            sp.SpanID,
		ParentSpanID:      sp.ParentSpanID,
		Kind:              sp.Kind,
		Operation:         sp.Operation,
		DisplayName:       sp.DisplayName,
		RunID:             sp.RunID,
		RequestID:         sp.RequestID,
		Status:            sp.Status,
		StartedAt:         sp.StartedAt,
		EndedAt:           sp.EndedAt,
		DurationMs:        sp.DurationMs,
		InputSummary:      sp.InputSummary,
		OutputSummary:     sp.OutputSummary,
		TokensIn:          sp.TokensIn,
		TokensOut:         sp.TokensOut,
		ReasoningTokens:   sp.ReasoningTokens,
		ErrorType:         sp.ErrorType,
		ErrorMessage:      sp.ErrorMessage,
		Attributes:        sp.Attributes,
		TriggeredBySpanID: triggeredBy,
	}
}

func (s *traceServiceImpl) GetTraceByRunID(ctx context.Context, runID string) (string, error) {
	traceID, err := s.repo.GetTraceIDByRunID(ctx, runID)
	if err != nil {
		return "", err
	}
	if traceID == "" {
		return "", pkgerrors.New(pkgerrors.CodeNotFound, fmt.Sprintf("run %s 无关联 trace", runID))
	}
	return traceID, nil
}

// extractRunID 从 spans 中提取 agent.run span 的 run_id
func extractRunID(spans []*entity.TraceSpan) string {
	for _, sp := range spans {
		if sp.Operation == "agent.run" && sp.RunID != "" {
			return sp.RunID
		}
	}
	return ""
}

// isStatusMismatch 判断 Agent Span 状态与业务 Run 状态是否一致
// 映射关系：
//
//	ok ↔ completed
//	error ↔ failed
//	cancelled ↔ cancelled
//	interrupted ↔ interrupted
//	running ↔ pending/running
func isStatusMismatch(agentStatus, businessStatus string) bool {
	if agentStatus == "" || businessStatus == "" {
		return false
	}
	expected := map[string][]string{
		trace.SpanStatusOK:          {entity.StatusCompleted},
		trace.SpanStatusError:       {entity.StatusFailed},
		trace.SpanStatusCancelled:   {entity.StatusCancelled},
		trace.SpanStatusInterrupted: {entity.StatusInterrupted},
		trace.SpanStatusRunning:     {entity.StatusPending, entity.StatusRunning},
		trace.SpanStatusTimeout:     {entity.StatusFailed, entity.StatusCancelled},
	}
	allowed, ok := expected[agentStatus]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == businessStatus {
			return false
		}
	}
	return true
}

// buildDiagnostics 构建诊断提示（running span、孤儿 span、状态不一致）
func buildDiagnostics(spans []*entity.TraceSpan, run *TraceRunSummary) []TraceDiagnostic {
	var diags []TraceDiagnostic
	spanIDs := map[string]bool{}
	for _, sp := range spans {
		spanIDs[sp.SpanID] = true
	}
	for _, sp := range spans {
		// running span 诊断（非 http.server 的 running span）
		if sp.Status == trace.SpanStatusRunning && sp.Operation != "http.server" {
			diags = append(diags, TraceDiagnostic{
				Code:    "running_span",
				Message: fmt.Sprintf("span %s (%s) 仍处于 running 状态", sp.SpanID, sp.Operation),
				SpanID:  sp.SpanID,
			})
		}
		// 孤儿 span 诊断（parent_span_id 非空但不存在）
		if sp.ParentSpanID != "" && !spanIDs[sp.ParentSpanID] {
			diags = append(diags, TraceDiagnostic{
				Code:    "orphan_span",
				Message: fmt.Sprintf("span %s 的父 span %s 不存在", sp.SpanID, sp.ParentSpanID),
				SpanID:  sp.SpanID,
			})
		}
		// 慢队列诊断（queue.consume 排队超过阈值）
		if sp.Operation == "queue.consume" && sp.Attributes != nil {
			if ms := attrInt64(sp.Attributes["queue_wait_ms"]); ms > slowQueueThresholdMs {
				diags = append(diags, TraceDiagnostic{
					Code:    "slow_queue",
					Message: fmt.Sprintf("排队等待 %dms 超过阈值 %dms", ms, slowQueueThresholdMs),
					SpanID:  sp.SpanID,
				})
			}
		}
	}
	// 状态不一致诊断
	if run != nil {
		agentRunSpan := findAgentRunSpan(spans)
		if agentRunSpan != nil {
			if isStatusMismatch(agentRunSpan.Status, run.Status) {
				diags = append(diags, TraceDiagnostic{
					Code:    "status_mismatch",
					Message: fmt.Sprintf("agent.run span 状态为 %s，但 agent_runs.status 为 %s", agentRunSpan.Status, run.Status),
					SpanID:  agentRunSpan.SpanID,
				})
			}
		}
	}
	return diags
}

// slowQueueThresholdMs 慢队列诊断阈值（与前端 collectDiagnostics 默认值保持一致）
const slowQueueThresholdMs = 10000

// attrInt64 从 attributes 中读取整型值（兼容 int/int64/float64/json.Number）。
func attrInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		if p, err := n.Int64(); err == nil {
			return p
		}
	}
	return 0
}

func findAgentRunSpan(spans []*entity.TraceSpan) *entity.TraceSpan {
	for _, sp := range spans {
		if sp.Operation == "agent.run" {
			return sp
		}
	}
	return nil
}
