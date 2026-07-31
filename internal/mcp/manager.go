package mcp

import (
	"context"
	"sync"

	"Qavor/internal/model/entity"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// MCPManager MCP 管理器
type MCPManager struct {
	mu      sync.RWMutex
	clients map[string]*client.Client
	tools   []tool.BaseTool
	status  map[string]string // "pending" | "ready" | "failed"
}

// NewMCPManager 创建 MCP 管理器
func NewMCPManager() *MCPManager {
	return &MCPManager{
		clients: make(map[string]*client.Client),
		status:  make(map[string]string),
	}
}

// StartAll 启动所有启用的 MCP 服务器
func (m *MCPManager) StartAll(configs map[string]*entity.MCPServerConfig) {
	for name, config := range configs {
		if !config.Enabled {
			continue
		}
		m.status[name] = "pending"
		go m.startServer(name, config)
	}
}

// startServer 启动单个 MCP 服务器
func (m *MCPManager) startServer(name string, config *entity.MCPServerConfig) {
	logger.Info("启动 MCP 服务器", zap.String("name", name))

	ctx := context.Background()

	// 创建 MCP 客户端
	var c *client.Client
	var err error

	switch config.Transport {
	case "stdio":
		if config.Command == "" {
			logger.Error("stdio 模式需要 command 参数", zap.String("name", name))
			m.mu.Lock()
			m.status[name] = "failed"
			m.mu.Unlock()
			return
		}

		// 构建环境变量
		var env []string
		for k, v := range config.Env {
			env = append(env, k+"="+v)
		}

		c, err = client.NewStdioMCPClient(config.Command, env, config.Args...)
	case "sse":
		c, err = client.NewSSEMCPClient(config.URL)
	case "streamable-http":
		c, err = client.NewStreamableHttpClient(config.URL)
	default:
		logger.Error("不支持的传输类型", zap.String("name", name), zap.String("transport", config.Transport))
		m.mu.Lock()
		m.status[name] = "failed"
		m.mu.Unlock()
		return
	}

	if err != nil {
		logger.Error("MCP 客户端创建失败", zap.String("name", name), zap.Error(err))
		m.mu.Lock()
		m.status[name] = "failed"
		m.mu.Unlock()
		return
	}

	// 获取工具
	tools, err := einomcp.GetTools(ctx, &einomcp.Config{
		Cli: c,
	})
	if err != nil {
		logger.Error("获取 MCP 工具失败", zap.String("name", name), zap.Error(err))
		m.mu.Lock()
		m.status[name] = "failed"
		m.mu.Unlock()
		return
	}

	// 注册工具
	m.mu.Lock()
	m.clients[name] = c
	m.tools = append(m.tools, tools...)
	m.status[name] = "ready"
	m.mu.Unlock()

	logger.Info("MCP 服务器启动成功", zap.String("name", name), zap.Int("tools", len(tools)))
}

// StopServer 停止单个 MCP 服务器
func (m *MCPManager) StopServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.clients[name]
	if !ok {
		return nil
	}

	// 关闭连接
	delete(m.clients, name)
	delete(m.status, name)

	// 重新构建工具列表
	m.tools = nil
	for n, client := range m.clients {
		if m.status[n] != "ready" {
			continue
		}
		tools, err := einomcp.GetTools(context.Background(), &einomcp.Config{
			Cli: client,
		})
		if err != nil {
			continue
		}
		m.tools = append(m.tools, tools...)
	}

	return c.Close()
}

// GetTools 获取所有工具
func (m *MCPManager) GetTools() []tool.BaseTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]tool.BaseTool, len(m.tools))
	copy(result, m.tools)
	return result
}

// GetStatus 获取服务器状态
func (m *MCPManager) GetStatus(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.status[name]
}

// GetAllStatus 获取所有状态
func (m *MCPManager) GetAllStatus() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.status))
	for k, v := range m.status {
		result[k] = v
	}
	return result
}

// Refresh 刷新配置
func (m *MCPManager) Refresh(configs map[string]*entity.MCPServerConfig) {
	// 停止已删除或禁用的服务器
	m.mu.RLock()
	currentServers := make(map[string]bool)
	for name := range m.clients {
		currentServers[name] = true
	}
	m.mu.RUnlock()

	// 停止不再需要的服务器
	for name := range currentServers {
		config, ok := configs[name]
		if !ok || !config.Enabled {
			m.StopServer(name)
		}
	}

	// 启动新的服务器
	for name, config := range configs {
		if !config.Enabled {
			continue
		}

		m.mu.RLock()
		_, exists := m.clients[name]
		m.mu.RUnlock()

		if !exists {
			m.status[name] = "pending"
			go m.startServer(name, config)
		}
	}
}

// Close 关闭所有连接
func (m *MCPManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, c := range m.clients {
		c.Close()
		delete(m.clients, name)
	}

	m.tools = nil
	m.status = make(map[string]string)
}
