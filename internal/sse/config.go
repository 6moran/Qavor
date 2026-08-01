package sse

import (
	"time"

	"Qavor/pkg/config"
)

// SSEConfig SSE 配置（内部使用）
type SSEConfig struct {
	MaxStreamTime      time.Duration // 单次流式最大时长
	HeartbeatInterval  time.Duration // 流式过程中心跳间隔
	MaxConcurrentTasks int           // 单用户最大并发任务数
}

// NewSSEConfig 从应用配置创建 SSE 配置
func NewSSEConfig(cfg *config.SSEConfig) *SSEConfig {
	return &SSEConfig{
		MaxStreamTime:      time.Duration(cfg.MaxStreamTime) * time.Second,
		HeartbeatInterval:  time.Duration(cfg.HeartbeatInterval) * time.Second,
		MaxConcurrentTasks: cfg.MaxConcurrentTasks,
	}
}

// DefaultConfig 返回默认 SSE 配置（用于测试或无配置文件时）
func DefaultConfig() *SSEConfig {
	return &SSEConfig{
		MaxStreamTime:      5 * time.Minute,  // 单次流式最大5分钟
		HeartbeatInterval:  15 * time.Second, // 心跳间隔：15秒
		MaxConcurrentTasks: 5,                // 单用户最大并发任务数
	}
}
