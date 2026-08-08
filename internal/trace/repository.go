package trace

import (
	"context"
	"time"

	"Qavor/internal/model/entity"
)

// TraceFilter 列表筛选条件
type TraceFilter struct {
	Keyword        string
	AgentSlug      string
	ConversationID uint
	Status         string
	Source         string
	From           time.Time
	To             time.Time
	Page           int
	PageSize       int
}

// TraceRepository trace 数据访问接口，由 repository 包实现（解耦 trace 包与 GORM）
type TraceRepository interface {
	CreateTrace(ctx context.Context, t *entity.AgentTrace) error
	CreateSpan(ctx context.Context, s *entity.AgentTraceSpan) error
	UpdateSpan(ctx context.Context, s *entity.AgentTraceSpan) error
	GetTrace(ctx context.Context, traceID string) (*entity.AgentTrace, error)
	ListTraces(ctx context.Context, filter TraceFilter) ([]*entity.AgentTrace, int64, error)
	ListSpans(ctx context.Context, traceID string) ([]*entity.AgentTraceSpan, error)
	// FinishTrace 收尾：聚合 Token/模型名/耗时并更新状态（幂等：非 running 不再覆盖）
	FinishTrace(ctx context.Context, traceID, status, errMsg string) error
	// MarkTimeoutTraces 将 running 且 started_at 早于 before 的记录标记为 timeout，返回影响行数
	MarkTimeoutTraces(ctx context.Context, before time.Time) (int64, error)
	// DeleteExpired 物理删除 created_at 早于 before 的记录（先 spans 后 traces），返回删除的 trace 数
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}
