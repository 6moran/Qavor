package sse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 连接管理器
type Manager struct {
	mu          sync.RWMutex
	connections map[string][]*UserConnection // username -> []connection
	heartbeat   *HeartbeatManager
	logger      *zap.Logger
	config      *ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxConnectionsPerUser int           // 每用户最大连接数
	CleanInterval         time.Duration // 清理过期连接的间隔
	ConnectionTimeout     time.Duration // 连接超时时间
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxConnectionsPerUser: 5,
		CleanInterval:         5 * time.Minute,
		ConnectionTimeout:     10 * time.Minute,
	}
}

// NewManager 创建连接管理器
func NewManager(heartbeat *HeartbeatManager, logger *zap.Logger, config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	return &Manager{
		connections: make(map[string][]*UserConnection),
		heartbeat:   heartbeat,
		logger:      logger,
		config:      config,
	}
}

// Connect 建立用户连接
func (m *Manager) Connect(ctx context.Context, username string, deviceID string, writer *SSEWriter) *UserConnection {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成连接ID
	connID := generateConnID()

	// 检查连接数限制
	connections := m.connections[username]
	if len(connections) >= m.config.MaxConnectionsPerUser {
		// 关闭最旧的连接
		oldest := connections[0]
		oldest.Close()
		connections = connections[1:]
		m.logger.Info("关闭最旧连接（超过限制）",
			zap.String("username", username),
			zap.String("conn_id", oldest.ConnID),
		)
	}

	// 创建新连接
	conn := NewUserConnection(connID, deviceID, writer)
	connections = append(connections, conn)
	m.connections[username] = connections

	// 启动心跳
	m.heartbeat.Start(ctx, conn)

	// 发送 connected 事件
	conn.Writer.Send(NewSSEEvent(EventConnected, ConnectedData{
		ConnID: connID,
	}))

	m.logger.Info("用户 SSE 连接建立",
		zap.String("username", username),
		zap.String("conn_id", connID),
		zap.String("device_id", deviceID),
	)

	return conn
}

// Disconnect 断开用户连接
func (m *Manager) Disconnect(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for username, connections := range m.connections {
		for i, conn := range connections {
			if conn.ConnID == connID {
				conn.Close()
				m.connections[username] = append(connections[:i], connections[i+1:]...)
				m.logger.Info("用户 SSE 连接断开",
					zap.String("username", username),
					zap.String("conn_id", connID),
				)
				return
			}
		}
	}
}

// DisconnectUser 断开用户所有连接
func (m *Manager) DisconnectUser(username string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	connections := m.connections[username]
	for _, conn := range connections {
		conn.Close()
	}
	delete(m.connections, username)
	m.logger.Info("用户所有 SSE 连接断开",
		zap.String("username", username),
	)
}

// SendToUser 向用户所有连接广播
func (m *Manager) SendToUser(username string, event SSEEvent) error {
	m.mu.RLock()
	connections := m.connections[username]
	m.mu.RUnlock()

	if len(connections) == 0 {
		return fmt.Errorf("用户 %s 没有活跃的 SSE 连接", username)
	}

	for _, conn := range connections {
		if conn.IsAlive() {
			conn.Writer.Send(event)
			conn.UpdateLastEventID(event.ID)
		}
	}
	return nil
}

// SendToDevice 向指定设备推送
func (m *Manager) SendToDevice(username string, deviceID string, event SSEEvent) error {
	m.mu.RLock()
	connections := m.connections[username]
	m.mu.RUnlock()

	for _, conn := range connections {
		if conn.DeviceID == deviceID && conn.IsAlive() {
			conn.Writer.Send(event)
			conn.UpdateLastEventID(event.ID)
			return nil
		}
	}
	return fmt.Errorf("用户 %s 没有设备 %s 的活跃连接", username, deviceID)
}

// IsConnected 检查用户是否在线
func (m *Manager) IsConnected(username string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connections := m.connections[username]
	return len(connections) > 0
}

// GetConnections 获取用户的所有连接
func (m *Manager) GetConnections(username string) []*UserConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[username]
}

// GetConnectionCount 获取用户连接数
func (m *Manager) GetConnectionCount(username string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections[username])
}

// CleanExpiredConnections 清理过期连接
func (m *Manager) CleanExpiredConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for username, connections := range m.connections {
		var alive []*UserConnection
		for _, conn := range connections {
			if now.Sub(conn.LastActive) > m.config.ConnectionTimeout {
				conn.Close()
				m.logger.Info("清理过期 SSE 连接",
					zap.String("username", username),
					zap.String("conn_id", conn.ConnID),
				)
			} else {
				alive = append(alive, conn)
			}
		}
		if len(alive) == 0 {
			delete(m.connections, username)
		} else {
			m.connections[username] = alive
		}
	}
}

// StartCleaner 启动清理协程
func (m *Manager) StartCleaner(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.config.CleanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.CleanExpiredConnections()
			}
		}
	}()
}

// generateConnID 生成连接ID
func generateConnID() string {
	return fmt.Sprintf("conn_%d_%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1) // 确保不同的时间戳
	}
	return string(b)
}
