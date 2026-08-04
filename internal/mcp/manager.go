package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"Qavor/internal/model/entity"
	"Qavor/internal/store"

	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// ConnectStatus MCP 服务器连接状态
type ConnectStatus int

const (
	StatusUnknown    ConnectStatus = iota // 从未尝试连接
	StatusConnecting                      // 正在连接
	StatusConnected                       // 已连接
	StatusFailed                          // 连接失败
)

func (s ConnectStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// MCPManager MCP 管理器
// 所有 Agent 共用同一个实例，连接池全局共享
type MCPManager struct {
	mu          sync.RWMutex
	clients     map[string]*client.Client
	serverTools map[string][]tool.BaseTool // name → 该服务器的工具（nil = 未获取）
	status      map[string]ConnectStatus
	fileStore   store.MCPServerFileStore
}

// NewMCPManager 创建 MCP 管理器（不建立任何连接）
func NewMCPManager(fileStore store.MCPServerFileStore) *MCPManager {
	return &MCPManager{
		clients:     make(map[string]*client.Client),
		serverTools: make(map[string][]tool.BaseTool),
		status:      make(map[string]ConnectStatus),
		fileStore:   fileStore,
	}
}

// Preheat 后台异步预热白名单内的 MCP 服务器
// 只建连接，不获取工具列表
func (m *MCPManager) Preheat(whitelist []string) {
	if len(whitelist) == 0 {
		return
	}

	configs, err := m.fileStore.GetAll()
	if err != nil {
		logger.Warn("MCP 配置加载失败，跳过预热", zap.Error(err))
		return
	}

	allowSet := make(map[string]bool, len(whitelist))
	for _, name := range whitelist {
		allowSet[name] = true
	}

	count := 0
	for name, config := range configs {
		if !config.Enabled || !allowSet[name] {
			continue
		}
		m.mu.Lock()
		m.status[name] = StatusConnecting
		m.mu.Unlock()

		go m.connect(name, config)
		count++
	}

	if count > 0 {
		logger.Info("MCP 预热已启动", zap.Int("servers", count), zap.Strings("whitelist", whitelist))
	}
}

// EnsureConnected 确保指定的 MCP 服务器已连接
// 已连接 → 跳过；未连接 → 懒加载；预热失败 → 重试
func (m *MCPManager) EnsureConnected(names []string) {
	for _, name := range names {
		m.mu.RLock()
		s := m.status[name]
		m.mu.RUnlock()

		if s == StatusConnected {
			continue
		}

		config, err := m.fileStore.GetByName(name)
		if err != nil || config == nil || !config.Enabled {
			continue
		}

		if err := m.connect(name, config); err != nil {
			logger.Warn("MCP 服务器连接失败（懒加载）",
				zap.String("name", name), zap.Error(err))
		}
	}
}

// newClient 按传输类型创建 MCP client，返回后尚未 Start/Initialize
func (m *MCPManager) newClient(config *entity.MCPServerConfig) (*client.Client, error) {
	transport := strings.ReplaceAll(strings.ToLower(config.Transport), "_", "-")

	var c *client.Client
	var err error

	switch transport {
	case "stdio":
		if config.Command == "" {
			return nil, fmt.Errorf("stdio 模式需要 command 参数")
		}
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
		return nil, fmt.Errorf("不支持的传输类型: %s", config.Transport)
	}
	return c, err
}

// connect 连接单个 MCP 服务器（只建连接，不获取工具）
func (m *MCPManager) connect(name string, config *entity.MCPServerConfig) error {
	m.mu.RLock()
	if m.status[name] == StatusConnected {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	logger.Info("连接 MCP 服务器", zap.String("name", name))

	c, err := m.newClient(config)
	if err != nil {
		m.mu.Lock()
		m.status[name] = StatusFailed
		m.mu.Unlock()
		return err
	}

	// 启动 transport 并完成 MCP 握手：Start -> Initialize 后才能请求工具等接口
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		m.mu.Lock()
		m.status[name] = StatusFailed
		m.mu.Unlock()
		logger.Warn("MCP 服务器启动失败", zap.String("name", name), zap.Error(err))
		return fmt.Errorf("MCP 服务器启动失败: %w", err)
	}
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "qavor",
				Version: "0.7.1",
			},
		},
	}); err != nil {
		m.mu.Lock()
		m.status[name] = StatusFailed
		m.mu.Unlock()
		logger.Warn("MCP 服务器初始化失败", zap.String("name", name), zap.Error(err))
		return fmt.Errorf("MCP 服务器初始化失败: %w", err)
	}

	m.mu.Lock()
	m.clients[name] = c
	m.status[name] = StatusConnected
	m.mu.Unlock()

	logger.Info("MCP 服务器连接成功", zap.String("name", name))
	return nil
}

// TestConnect 测试 MCP 服务器连通性：建立连接并完成握手后立即关闭
// 不写状态机、不入连接池，仅用于表单"测试连接"按钮的预验证
// 返回握手响应中的 ServerInfo（服务器名称/版本），供前端展示
func (m *MCPManager) TestConnect(config *entity.MCPServerConfig) (*mcp.Implementation, error) {
	c, err := m.newClient(config)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("MCP 服务器启动失败: %w", err)
	}
	resp, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "qavor",
				Version: "0.7.1",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("MCP 服务器初始化失败: %w", err)
	}
	return &resp.ServerInfo, nil
}

// GetToolsByServers 获取指定 MCP 服务器的工具
// 只获取指定服务器的工具，不涉及其他服务器
func (m *MCPManager) GetToolsByServers(names []string) []tool.BaseTool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var all []tool.BaseTool
	for _, name := range names {
		// 已有缓存，直接用
		if cached, ok := m.serverTools[name]; ok && cached != nil {
			all = append(all, cached...)
			continue
		}

		// 未缓存，从客户端获取
		c, ok := m.clients[name]
		if !ok || m.status[name] != StatusConnected {
			continue
		}

		fetched, err := einomcp.GetTools(context.Background(), &einomcp.Config{Cli: c})
		if err != nil {
			logger.Warn("获取 MCP 工具失败", zap.String("name", name), zap.Error(err))
			continue
		}
		m.serverTools[name] = fetched
		all = append(all, fetched...)
		logger.Info("MCP 工具已获取", zap.String("name", name), zap.Int("count", len(fetched)))
	}

	return all
}

// GetTools 获取所有已连接服务器的工具（兼容旧接口）
func (m *MCPManager) GetTools() []tool.BaseTool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, c := range m.clients {
		if m.status[name] == StatusConnected && m.serverTools[name] == nil {
			fetched, err := einomcp.GetTools(context.Background(), &einomcp.Config{Cli: c})
			if err != nil {
				logger.Warn("获取 MCP 工具失败", zap.String("name", name), zap.Error(err))
				continue
			}
			m.serverTools[name] = fetched
		}
	}

	var all []tool.BaseTool
	for _, tools := range m.serverTools {
		all = append(all, tools...)
	}
	return all
}

// GetStatus 获取指定服务器的连接状态
func (m *MCPManager) GetStatus(name string) ConnectStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status[name]
}

// GetAllStatus 获取所有服务器的连接状态
func (m *MCPManager) GetAllStatus() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.status))
	for k, v := range m.status {
		result[k] = v.String()
	}
	return result
}

// Has 检查 MCP 服务器是否存在
func (m *MCPManager) Has(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.status[name]
	return exists
}

// Close 关闭指定服务器的连接并清理缓存（删除或禁用 MCP 时调用）
func (m *MCPManager) Close(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error
	if c, ok := m.clients[name]; ok && c != nil {
		err = c.Close()
	}
	delete(m.clients, name)
	delete(m.serverTools, name)
	delete(m.status, name)
	return err
}

// CloseAll 关闭所有 MCP 服务器连接并清理缓存（服务优雅关闭时调用）
func (m *MCPManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, c := range m.clients {
		if c != nil {
			if err := c.Close(); err != nil {
				logger.Warn("关闭 MCP 服务器连接失败", zap.String("name", name), zap.Error(err))
			}
		}
	}
	m.clients = make(map[string]*client.Client)
	m.serverTools = make(map[string][]tool.BaseTool)
	m.status = make(map[string]ConnectStatus)
}
