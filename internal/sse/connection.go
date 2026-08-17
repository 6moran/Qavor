package sse

import (
	"sync"
	"time"
)

// ConnStatus 连接状态
type ConnStatus string

const (
	ConnStatusConnected    ConnStatus = "connected"
	ConnStatusDisconnected ConnStatus = "disconnected"
)

// UserConnection 用户连接
type UserConnection struct {
	ConnID      string        // 连接唯一ID
	DeviceID    string        // 设备标识
	Writer      *SSEWriter    // 写入器
	Status      ConnStatus    // 连接状态
	CreatedAt   time.Time     // 创建时间
	LastActive  time.Time     // 最后活跃时间
	LastEventID string        // 最后事件ID
	Done        chan struct{} // 关闭信号
	mu          sync.Mutex
}

// NewUserConnection 创建用户连接
func NewUserConnection(connID string, deviceID string, writer *SSEWriter) *UserConnection {
	return &UserConnection{
		ConnID:     connID,
		DeviceID:   deviceID,
		Writer:     writer,
		Status:     ConnStatusConnected,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Done:       make(chan struct{}),
	}
}

// UpdateLastEventID 更新最后事件ID
func (c *UserConnection) UpdateLastEventID(eventID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastEventID = eventID
}

// GetLastEventID 获取最后事件ID
func (c *UserConnection) GetLastEventID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LastEventID
}

// Close 关闭连接
func (c *UserConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.Done:
		// 已关闭
	default:
		close(c.Done)
	}
	c.Status = ConnStatusDisconnected
	c.Writer.Close()
}

// IsAlive 检查连接是否存活
func (c *UserConnection) IsAlive() bool {
	return c.Writer.IsAlive()
}
