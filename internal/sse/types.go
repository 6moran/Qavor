package sse

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SSEConfig SSE 配置
type SSEConfig struct {
	MaxStreamTime      time.Duration // 单次流式最大时长
	HeartbeatInterval  time.Duration // 流式过程中心跳间隔
	MaxConcurrentTasks int           // 单用户最大并发任务数
}

// GenerateTaskID 生成任务ID
func GenerateTaskID() string {
	id := uuid.New().String()[:8]
	timestamp := time.Now().Unix() % 1000000
	return fmt.Sprintf("task_%s_%06d", id, timestamp)
}
