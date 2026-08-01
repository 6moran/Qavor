package sse

import "time"

// DefaultConfig 返回默认 SSE 配置
func DefaultConfig() *SSEConfig {
	return &SSEConfig{
		MaxStreamTime:      5 * time.Minute,  // 单次流式最大5分钟
		HeartbeatInterval:  15 * time.Second, // 心跳间隔：15秒
		MaxConcurrentTasks: 5,                // 单用户最大并发任务数
	}
}
