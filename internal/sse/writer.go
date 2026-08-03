package sse

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SSEWriter 线程安全的 SSE 写入器
type SSEWriter struct {
	c       *gin.Context
	eventCh chan SSEEvent
	done    chan struct{}
	once    sync.Once
	logger  *zap.Logger
}

// NewSSEWriter 创建 SSE 写入器
func NewSSEWriter(c *gin.Context, logger *zap.Logger) *SSEWriter {
	w := &SSEWriter{
		c:       c,
		eventCh: make(chan SSEEvent, 100), // 缓冲 channel
		done:    make(chan struct{}),
		logger:  logger,
	}
	go w.writeLoop()
	return w
}

// writeLoop 写入循环（单线程处理所有写入）
func (w *SSEWriter) writeLoop() {
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.eventCh:
			if !ok {
				return
			}
			w.writeEvent(event)
		}
	}
}

// writeEvent 写入单个事件
func (w *SSEWriter) writeEvent(event SSEEvent) {
	jsonData, err := json.Marshal(event)
	if err != nil {
		w.logger.Error("序列化事件失败", zap.Error(err))
		return
	}

	// 检查连接是否已关闭
	if w.c.Writer.Written() {
		return
	}

	_, err = fmt.Fprintf(w.c.Writer, "event: %s\ndata: %s\n\n", event.Type, string(jsonData))
	if err != nil {
		w.logger.Debug("写入事件失败（连接可能已关闭）", zap.Error(err))
		return
	}

	w.c.Writer.Flush()
}

// Send 发送事件（线程安全）
func (w *SSEWriter) Send(eventType EventType, data interface{}) {
	event := SSEEvent{
		Type: eventType,
		Data: data,
	}

	select {
	case w.eventCh <- event:
	case <-w.done:
		// 连接已关闭，丢弃事件
	default:
		// Channel 满了，丢弃事件（避免阻塞）
		w.logger.Warn("SSE 事件队列已满，丢弃事件",
			zap.String("event_type", string(eventType)),
		)
	}
}

// Close 关闭写入器
func (w *SSEWriter) Close() {
	w.once.Do(func() {
		close(w.done)
		close(w.eventCh)
	})
}

// SendHeartbeat 发送心跳事件
func (w *SSEWriter) SendHeartbeat(messageID string) {
	w.Send(EventHeartbeat, HeartbeatData{
		MessageID: messageID,
		Timestamp: time.Now().Unix(),
	})
}
