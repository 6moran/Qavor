package sse

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GenerateTaskID 生成任务ID
func GenerateTaskID() string {
	id := uuid.New().String()[:8]
	timestamp := time.Now().Unix() % 1000000
	return fmt.Sprintf("task_%s_%06d", id, timestamp)
}
