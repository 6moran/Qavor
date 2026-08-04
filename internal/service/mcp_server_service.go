package service

import (
	"context"
	"strings"

	"Qavor/internal/mcp"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/store"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/response"
)

// mcpServerService MCP服务器服务实现
type mcpServerService struct {
	fileStore  store.MCPServerFileStore
	mcpManager *mcp.MCPManager
}

// NewMCPServerService 创建MCP服务器服务
func NewMCPServerService(fileStore store.MCPServerFileStore, mcpManager *mcp.MCPManager) MCPServerService {
	return &mcpServerService{
		fileStore:  fileStore,
		mcpManager: mcpManager,
	}
}

// CreateMCPServer 创建MCP服务器
func (s *mcpServerService) CreateMCPServer(req *request.CreateMCPServerRequest) (*dto.MCPServerResponse, error) {
	// name 唯一性检查
	existing, _ := s.fileStore.GetByName(req.Name)
	if existing != nil {
		return nil, bizerrors.New(bizerrors.CodeResourceAlreadyExists, "MCP 已存在")
	}

	// 转换 args 类型
	var args []string
	if req.Args != nil {
		for _, arg := range req.Args {
			if str, ok := arg.(string); ok {
				args = append(args, str)
			}
		}
	}

	// 转换 env 类型
	var env map[string]string
	if req.Env != nil {
		env = make(map[string]string)
		for k, v := range req.Env {
			if str, ok := v.(string); ok {
				env[k] = str
			}
		}
	}

	// 转换 headers 类型
	var headers map[string]string
	if req.Headers != nil {
		headers = make(map[string]string)
		for k, v := range req.Headers {
			if str, ok := v.(string); ok {
				headers[k] = str
			}
		}
	}

	// 转换 disabledTools 类型
	var disabledTools []string
	if req.DisabledTools != nil {
		for _, tool := range req.DisabledTools {
			if str, ok := tool.(string); ok {
				disabledTools = append(disabledTools, str)
			}
		}
	}

	config := &entity.MCPServerConfig{
		Name:           req.Name,
		Description:    req.Description,
		Transport:      req.Transport,
		URL:            req.URL,
		Command:        req.Command,
		Args:           args,
		Env:            env,
		Headers:        headers,
		Timeout:        req.Timeout,
		SSEReadTimeout: req.SSEReadTimeout,
		Enabled:        true,
		DisabledTools:  disabledTools,
	}

	if err := s.fileStore.Create(req.Name, config); err != nil {
		return nil, err
	}

	return s.toResponse(req.Name, config), nil
}

// GetMCPServer 获取MCP服务器
func (s *mcpServerService) GetMCPServer(name string) (*dto.MCPServerResponse, error) {
	// 每次读取前检查文件变更
	_ = s.fileStore.RefreshIfChanged()

	config, err := s.fileStore.GetByName(name)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}

	return s.toResponse(name, config), nil
}

// UpdateMCPServer 更新MCP服务器
func (s *mcpServerService) UpdateMCPServer(name string, req *request.UpdateMCPServerRequest) (*dto.MCPServerResponse, error) {
	existing, err := s.fileStore.GetByName(name)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}

	// 构建更新配置
	updates := &entity.MCPServerConfig{}

	if req.Name != "" {
		updates.Name = req.Name
	}
	if req.Description != "" {
		updates.Description = req.Description
	}
	if req.URL != "" {
		updates.URL = req.URL
	}
	if req.Command != "" {
		updates.Command = req.Command
	}
	if req.Args != nil {
		var args []string
		for _, arg := range req.Args {
			if str, ok := arg.(string); ok {
				args = append(args, str)
			}
		}
		updates.Args = args
	}
	if req.Env != nil {
		env := make(map[string]string)
		for k, v := range req.Env {
			if str, ok := v.(string); ok {
				env[k] = str
			}
		}
		updates.Env = env
	}
	if req.Headers != nil {
		headers := make(map[string]string)
		for k, v := range req.Headers {
			if str, ok := v.(string); ok {
				headers[k] = str
			}
		}
		updates.Headers = headers
	}
	if req.Timeout != nil {
		updates.Timeout = req.Timeout
	}
	if req.SSEReadTimeout != nil {
		updates.SSEReadTimeout = req.SSEReadTimeout
	}
	if req.Enabled != nil {
		updates.Enabled = *req.Enabled == 1
	}
	if req.DisabledTools != nil {
		var disabledTools []string
		for _, tool := range req.DisabledTools {
			if str, ok := tool.(string); ok {
				disabledTools = append(disabledTools, str)
			}
		}
		updates.DisabledTools = disabledTools
	}

	if err := s.fileStore.Update(name, updates); err != nil {
		return nil, err
	}

	// 返回更新后的配置
	updated, _ := s.fileStore.GetByName(name)
	return s.toResponse(name, updated), nil
}

// DeleteMCPServer 删除MCP服务器
func (s *mcpServerService) DeleteMCPServer(name string) error {
	existing, err := s.fileStore.GetByName(name)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}
	if err := s.fileStore.Delete(name); err != nil {
		return err
	}
	// 删除后关闭连接并清理缓存
	if s.mcpManager != nil {
		s.mcpManager.Close(name)
	}
	return nil
}

// ListMCPServers 分页获取MCP服务器列表
func (s *mcpServerService) ListMCPServers(req *request.MCPServerListRequest) (*response.PageResponse, error) {
	_ = s.fileStore.RefreshIfChanged()

	all, err := s.fileStore.GetAll()
	if err != nil {
		return nil, err
	}

	// 过滤（keyword 搜索 name）
	var items []dto.MCPServerResponse
	for name, cfg := range all {
		if req.Keyword != "" && !strings.Contains(name, req.Keyword) && !strings.Contains(cfg.Name, req.Keyword) {
			continue
		}
		items = append(items, *s.toResponse(name, cfg))
	}

	// 分页
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	total := len(items)
	start := (page - 1) * size
	if start >= total {
		items = []dto.MCPServerResponse{}
	} else {
		end := start + size
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	return response.NewPageResponse(items, int64(total), page, size), nil
}

// EnableMCPServer 启用MCP服务器
func (s *mcpServerService) EnableMCPServer(name string) error {
	existing, err := s.fileStore.GetByName(name)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}

	updates := &entity.MCPServerConfig{
		Enabled: true,
	}
	return s.fileStore.Update(name, updates)
}

// DisableMCPServer 停用MCP服务器
func (s *mcpServerService) DisableMCPServer(name string) error {
	existing, err := s.fileStore.GetByName(name)
	if err != nil {
		return err
	}
	if existing == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}

	updates := &entity.MCPServerConfig{
		Enabled: false,
	}
	if err := s.fileStore.Update(name, updates); err != nil {
		return err
	}
	// 禁用后关闭连接，释放资源
	if s.mcpManager != nil {
		s.mcpManager.Close(name)
	}
	return nil
}

// RefreshIfChanged 刷新配置
func (s *mcpServerService) RefreshIfChanged() error {
	return s.fileStore.RefreshIfChanged()
}

// TestMCPServer 测试已保存的 MCP 服务器连接
func (s *mcpServerService) TestMCPServer(name string) error {
	config, err := s.fileStore.GetByName(name)
	if err != nil {
		return err
	}
	if config == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}
	if s.mcpManager == nil {
		return bizerrors.New(bizerrors.CodeInternalError, "MCP 管理器未初始化")
	}
	_, err = s.mcpManager.TestConnect(config)
	return err
}

// TestMCPServerConfig 测试表单中尚未保存的 MCP 配置是否可连通
func (s *mcpServerService) TestMCPServerConfig(req *request.CreateMCPServerRequest) (*dto.MCPTestResponse, error) {
	config := &entity.MCPServerConfig{
		Transport:      req.Transport,
		URL:            req.URL,
		Command:        req.Command,
		Args:           toArgs(req.Args),
		Env:            toEnv(req.Env),
		Headers:        toHeaders(req.Headers),
		Timeout:        req.Timeout,
		SSEReadTimeout: req.SSEReadTimeout,
	}
	if s.mcpManager == nil {
		return nil, bizerrors.New(bizerrors.CodeInternalError, "MCP 管理器未初始化")
	}
	info, err := s.mcpManager.TestConnect(config)
	if err != nil {
		return nil, err
	}
	resp := &dto.MCPTestResponse{}
	if info != nil {
		resp.ServerName = info.Name
		resp.ServerVersion = info.Version
	}
	return resp, nil
}

// GetMCPServerTools 获取MCP服务器的工具列表
func (s *mcpServerService) GetMCPServerTools(name string) ([]*dto.MCPToolResponse, error) {
	config, err := s.fileStore.GetByName(name)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}
	return s.listTools(config)
}

// RefreshMCPServerTools 刷新MCP服务器的工具列表
func (s *mcpServerService) RefreshMCPServerTools(name string) ([]*dto.MCPToolResponse, error) {
	config, err := s.fileStore.GetByName(name)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}
	return s.listTools(config)
}

// listTools 从 MCPManager 拉取指定服务器的工具并转换为响应 DTO
func (s *mcpServerService) listTools(config *entity.MCPServerConfig) ([]*dto.MCPToolResponse, error) {
	if s.mcpManager == nil {
		return []*dto.MCPToolResponse{}, nil
	}

	// 确保服务器已连接，才能获取工具
	s.mcpManager.EnsureConnected([]string{config.Name})

	disabledSet := make(map[string]bool, len(config.DisabledTools))
	for _, t := range config.DisabledTools {
		disabledSet[t] = true
	}

	tools := s.mcpManager.GetToolsByServers([]string{config.Name})
	result := make([]*dto.MCPToolResponse, 0, len(tools))
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		result = append(result, &dto.MCPToolResponse{
			Name:        info.Name,
			Description: info.Desc,
			Enabled:     !disabledSet[info.Name],
		})
	}
	return result, nil
}

// ToggleMCPServerTool 切换单个工具的启用状态
func (s *mcpServerService) ToggleMCPServerTool(serverName, toolName string) error {
	config, err := s.fileStore.GetByName(serverName)
	if err != nil {
		return err
	}
	if config == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}

	// 切换 DisabledTools 中的工具名
	disabled := config.DisabledTools
	idx := -1
	for i, t := range disabled {
		if t == toolName {
			idx = i
			break
		}
	}
	if idx >= 0 {
		// 当前已禁用 → 启用（移除）
		disabled = append(disabled[:idx], disabled[idx+1:]...)
	} else {
		// 当前已启用 → 禁用（加入）
		disabled = append(disabled, toolName)
	}

	updates := &entity.MCPServerConfig{DisabledTools: disabled}
	return s.fileStore.Update(serverName, updates)
}

// toResponse 转换为响应 DTO
func (s *mcpServerService) toResponse(name string, config *entity.MCPServerConfig) *dto.MCPServerResponse {
	if config == nil {
		return nil
	}

	// 转换 args 类型
	var args entity.JSONArray
	if config.Args != nil {
		args = make(entity.JSONArray, len(config.Args))
		for i, arg := range config.Args {
			args[i] = arg
		}
	}

	// 转换 env 类型
	var env entity.JSON
	if config.Env != nil {
		env = make(entity.JSON)
		for k, v := range config.Env {
			env[k] = v
		}
	}

	// 转换 headers 类型
	var headers entity.JSON
	if config.Headers != nil {
		headers = make(entity.JSON)
		for k, v := range config.Headers {
			headers[k] = v
		}
	}

	// 转换 tags 类型
	// 转换 disabledTools 类型
	var disabledTools entity.JSONArray
	if config.DisabledTools != nil {
		disabledTools = make(entity.JSONArray, len(config.DisabledTools))
		for i, tool := range config.DisabledTools {
			disabledTools[i] = tool
		}
	}

	// 转换 enabled 类型
	enabled := 0
	if config.Enabled {
		enabled = 1
	}

	// 获取连接状态（mcpManager 未初始化时兜底为 unknown）
	status := "unknown"
	if s.mcpManager != nil {
		status = s.mcpManager.GetStatus(name).String()
	}

	return &dto.MCPServerResponse{
		Name:           name,
		Description:    config.Description,
		Transport:      config.Transport,
		URL:            config.URL,
		Command:        config.Command,
		Args:           args,
		Env:            env,
		Headers:        headers,
		Timeout:        config.Timeout,
		SSEReadTimeout: config.SSEReadTimeout,
		Enabled:        enabled,
		DisabledTools:  disabledTools,
		Status:         status,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
	}
}

// toArgs 将请求中的 JSONArray 参数转换为 []string
func toArgs(args entity.JSONArray) []string {
	if args == nil {
		return nil
	}
	var result []string
	for _, arg := range args {
		if str, ok := arg.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// toEnv 将请求中的 JSON 环境变量转换为 map[string]string
func toEnv(env entity.JSON) map[string]string {
	if env == nil {
		return nil
	}
	result := make(map[string]string, len(env))
	for k, v := range env {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

// toHeaders 将请求中的 JSON 请求头转换为 map[string]string
func toHeaders(headers entity.JSON) map[string]string {
	if headers == nil {
		return nil
	}
	result := make(map[string]string, len(headers))
	for k, v := range headers {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}
