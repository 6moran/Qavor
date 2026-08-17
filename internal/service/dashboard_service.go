package service

import (
	"context"
	"fmt"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// dashboardService 仪表盘统计服务实现
type dashboardService struct {
	db *gorm.DB
}

// NewDashboardService 创建仪表盘统计服务
func NewDashboardService(db *gorm.DB) DashboardService {
	return &dashboardService{db: db}
}

// timeRangeSQL 根据 timeRange 返回时间过滤 WHERE 和 group by 表达式
// tablePrefix 可为空字符串（默认）或 "agent_runs." 等表名前缀
func timeRangeSQL(timeRange string, tablePrefix ...string) (where string, groupFmt string) {
	prefix := ""
	if len(tablePrefix) > 0 {
		prefix = tablePrefix[0]
	}
	now := time.Now()
	col := prefix + "created_at"
	switch timeRange {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return fmt.Sprintf("%s >= '%s' AND %s < '%s'",
				col, start.Format("2006-01-02 15:04:05"),
				col, start.AddDate(0, 0, 1).Format("2006-01-02 15:04:05")),
			fmt.Sprintf("to_char(%s, 'YYYY-MM-DD HH24:00')", col)
	case "7days":
		start := now.AddDate(0, 0, -6)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		return fmt.Sprintf("%s >= '%s'", col, start.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", col)
	case "thisMonth":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return fmt.Sprintf("%s >= '%s'", col, start.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", col)
	default:
		start := now.AddDate(0, 0, -6)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		return fmt.Sprintf("%s >= '%s'", col, start.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", col)
	}
}

// generateDateBuckets 生成指定时间范围内的所有日期桶，确保图表始终显示完整时间范围
func generateDateBuckets(timeRange string) []string {
	now := time.Now()
	var buckets []string

	switch timeRange {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		// 今天按小时分桶：00, 01, ..., 23
		for i := 0; i < 24; i++ {
			bucket := time.Date(start.Year(), start.Month(), start.Day(), i, 0, 0, 0, now.Location())
			buckets = append(buckets, bucket.Format("2006-01-02 15:04"))
		}
	case "7days":
		start := now.AddDate(0, 0, -6)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		for i := 0; i < 7; i++ {
			bucket := start.AddDate(0, 0, i)
			buckets = append(buckets, bucket.Format("2006-01-02"))
		}
	case "thisMonth":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		daysInMonth := now.Day()
		for i := 0; i < daysInMonth; i++ {
			bucket := start.AddDate(0, 0, i)
			buckets = append(buckets, bucket.Format("2006-01-02"))
		}
	default:
		start := now.AddDate(0, 0, -6)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		for i := 0; i < 7; i++ {
			bucket := start.AddDate(0, 0, i)
			buckets = append(buckets, bucket.Format("2006-01-02"))
		}
	}

	return buckets
}

// GetCallTimeseries 获取调用统计时间序列
func (s *dashboardService) GetCallTimeseries(ctx context.Context, dataType, timeRange string) (*TimeseriesResult, error) {
	switch dataType {
	case "models":
		return s.getModelStats(ctx, timeRange)
	case "agents":
		return s.getAgentStats(ctx, timeRange)
	case "tokens":
		return s.getTokenStats(ctx, timeRange)
	default:
		return nil, fmt.Errorf("不支持的数据类型: %s", dataType)
	}
}

// getModelStats 按模型统计Token消耗（查新表 trace_spans）
func (s *dashboardService) getModelStats(ctx context.Context, timeRange string) (*TimeseriesResult, error) {
	timeWhere, groupFmt := timeRangeSQL(timeRange)

	type modelRow struct {
		TimeBucket string
		ModelName  string
		Tokens     int
	}
	var rows []modelRow
	err := s.db.WithContext(ctx).Model(&entity.TraceSpan{}).
		Select(fmt.Sprintf("%s AS time_bucket, COALESCE(NULLIF(display_name, ''), 'unknown') AS model_name, SUM(tokens_in + tokens_out + COALESCE(reasoning_tokens, 0)) AS tokens", groupFmt)).
		Where("kind = ? AND status = ?", "llm", "ok").
		Where(timeWhere).
		Group("time_bucket, model_name").
		Order("time_bucket ASC").
		Scan(&rows).Error
	if err != nil {
		logger.Error("查询模型Token统计失败", zap.Error(err))
		return nil, err
	}

	categories := make(map[string]bool)
	bucketMap := make(map[string]map[string]int)

	for _, row := range rows {
		if _, ok := bucketMap[row.TimeBucket]; !ok {
			bucketMap[row.TimeBucket] = make(map[string]int)
		}
		bucketMap[row.TimeBucket][row.ModelName] += row.Tokens
		categories[row.ModelName] = true
	}

	catList := make([]string, 0, len(categories))
	for cat := range categories {
		catList = append(catList, cat)
	}

	// 填充完整时间范围，确保图表始终显示所有日期
	allBuckets := generateDateBuckets(timeRange)
	data := make([]TimeseriesPoint, 0, len(allBuckets))
	for _, bucket := range allBuckets {
		if _, ok := bucketMap[bucket]; !ok {
			bucketMap[bucket] = make(map[string]int)
		}
		data = append(data, TimeseriesPoint{
			Date: bucket,
			Data: bucketMap[bucket],
		})
	}

	return &TimeseriesResult{
		Data:       data,
		Categories: catList,
	}, nil
}

// getAgentStats 按智能体统计Token消耗（agent_runs JOIN trace_spans）
func (s *dashboardService) getAgentStats(ctx context.Context, timeRange string) (*TimeseriesResult, error) {
	// 使用 agent_runs. 前缀避免与 trace_spans.created_at 歧义
	timeWhere, groupFmt := timeRangeSQL(timeRange, "agent_runs.")

	type agentRow struct {
		TimeBucket  string
		AgentSlug   string
		TotalTokens int
	}
	var rows []agentRow

	// 使用 agent_runs.created_at 作为时间基准，LEFT JOIN trace_spans 获取 token 数
	err := s.db.WithContext(ctx).Table("agent_runs").
		Select(fmt.Sprintf("%s AS time_bucket, agent_runs.agent_slug, COALESCE(SUM(COALESCE(ts.tokens_in, 0) + COALESCE(ts.tokens_out, 0) + COALESCE(ts.reasoning_tokens, 0)), 0) AS total_tokens", groupFmt)).
		Joins("LEFT JOIN trace_spans ts ON ts.run_id = agent_runs.id AND ts.kind = 'llm' AND ts.status = 'ok'").
		Where(timeWhere).
		Group("time_bucket, agent_runs.agent_slug").
		Order("time_bucket ASC").
		Scan(&rows).Error
	if err != nil {
		logger.Error("查询智能体Token统计失败", zap.Error(err))
		return nil, err
	}

	categories := make(map[string]bool)
	bucketMap := make(map[string]map[string]int)

	for _, row := range rows {
		if _, ok := bucketMap[row.TimeBucket]; !ok {
			bucketMap[row.TimeBucket] = make(map[string]int)
		}
		bucketMap[row.TimeBucket][row.AgentSlug] += row.TotalTokens
		categories[row.AgentSlug] = true
	}

	catList := make([]string, 0, len(categories))
	for cat := range categories {
		catList = append(catList, cat)
	}

	// 填充完整时间范围，确保图表始终显示所有日期
	allBuckets := generateDateBuckets(timeRange)
	data := make([]TimeseriesPoint, 0, len(allBuckets))
	for _, bucket := range allBuckets {
		if _, ok := bucketMap[bucket]; !ok {
			bucketMap[bucket] = make(map[string]int)
		}
		data = append(data, TimeseriesPoint{
			Date: bucket,
			Data: bucketMap[bucket],
		})
	}

	// 获取 agent slug → name 映射
	var agents []entity.Agent
	agentNames := make(map[string]string)
	if err := s.db.WithContext(ctx).Model(&entity.Agent{}).Find(&agents).Error; err == nil {
		for _, a := range agents {
			agentNames[a.Slug] = a.Name
		}
	}

	// 使用 name 替换 slug 作为 categories
	namedCats := make([]string, 0, len(catList))
	seen := make(map[string]bool)
	for _, slug := range catList {
		name := agentNames[slug]
		if name == "" {
			name = slug
		}
		if !seen[name] {
			seen[name] = true
			namedCats = append(namedCats, name)
		}
	}

	return &TimeseriesResult{
		Data:       data,
		Categories: namedCats,
		AgentNames: agentNames,
	}, nil
}

// getTokenStats 获取总Token消耗时间序列（查新表 trace_spans，全部 llm span 汇总）
func (s *dashboardService) getTokenStats(ctx context.Context, timeRange string) (*TimeseriesResult, error) {
	timeWhere, groupFmt := timeRangeSQL(timeRange)

	type tokenRow struct {
		TimeBucket string
		Tokens     int
	}
	var rows []tokenRow
	err := s.db.WithContext(ctx).Model(&entity.TraceSpan{}).
		Select(fmt.Sprintf("%s AS time_bucket, SUM(tokens_in + tokens_out + COALESCE(reasoning_tokens, 0)) AS tokens", groupFmt)).
		Where("kind = ? AND status = ?", "llm", "ok").
		Where(timeWhere).
		Group("time_bucket").
		Order("time_bucket ASC").
		Scan(&rows).Error
	if err != nil {
		logger.Error("查询总Token统计失败", zap.Error(err))
		return nil, err
	}

	// 构建已有数据的 map
	tokenMap := make(map[string]int)
	for _, row := range rows {
		tokenMap[row.TimeBucket] = row.Tokens
	}

	// 填充完整时间范围，确保图表始终显示所有日期
	allBuckets := generateDateBuckets(timeRange)
	data := make([]TimeseriesPoint, 0, len(allBuckets))
	for _, bucket := range allBuckets {
		tokens := tokenMap[bucket]
		data = append(data, TimeseriesPoint{
			Date: bucket,
			Data: map[string]int{"total_tokens": tokens},
		})
	}

	return &TimeseriesResult{
		Data:       data,
		Categories: []string{"total_tokens"},
	}, nil
}
