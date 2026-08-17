package service

import "context"

// TimeseriesPoint 时间序列数据点
type TimeseriesPoint struct {
	Date string         `json:"date"`
	Data map[string]int `json:"data"`
}

// TimeseriesResult 时间序列查询结果
type TimeseriesResult struct {
	Data        []TimeseriesPoint `json:"data"`
	Categories  []string          `json:"categories"`
	AgentNames  map[string]string `json:"agent_names,omitempty"`
	ModelNames  map[string]string `json:"model_names,omitempty"`
}

// DashboardService 仪表盘统计服务接口
type DashboardService interface {
	// GetCallTimeseries 获取调用次数/Token消耗时间序列
	// dataType: models/agents/tokens
	// timeRange: today/7days/thisMonth
	GetCallTimeseries(ctx context.Context, dataType, timeRange string) (*TimeseriesResult, error)
}