package service

import (
	"strings"

	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/store"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/response"
)

// mcpServerService MCP服务器服务实现
type mcpServerService struct {
	fileStore store.MCPServerFileStore
}

// NewMCPServerService 创建MCP服务器服务
func NewMCPServerService(fileStore store.MCPServerFileStore) MCPServerService {
	return &mcpServerService{
		fileStore: fileStore,
	}
}

// CreateMCPServer 创建MCP服务器
func (s *mcpServerService) CreateMCPServer(username string, req *request.CreateMCPServerRequest) (*dto.MCPServerResponse, error) {
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

	// 转换 tags 类型
	var tags []string
	if req.Tags != nil {
		for _, tag := range req.Tags {
			if str, ok := tag.(string); ok {
				tags = append(tags, str)
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
		Tags:           tags,
		Icon:           req.Icon,
		Enabled:        true,
		DisabledTools:  disabledTools,
		CreatedBy:      username,
		UpdatedBy:      username,
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
func (s *mcpServerService) UpdateMCPServer(name string, username string, req *request.UpdateMCPServerRequest) (*dto.MCPServerResponse, error) {
	existing, err := s.fileStore.GetByName(name)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "MCP 服务器不存在")
	}

	// 构建更新配置
	updates := &entity.MCPServerConfig{
		UpdatedBy: username,
	}

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
	if req.Tags != nil {
		var tags []string
		for _, tag := range req.Tags {
			if str, ok := tag.(string); ok {
				tags = append(tags, str)
			}
		}
		updates.Tags = tags
	}
	if req.Icon != "" {
		updates.Icon = req.Icon
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
	return s.fileStore.Delete(name)
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
	return s.fileStore.Update(name, updates)
}

// RefreshIfChanged 刷新配置
func (s *mcpServerService) RefreshIfChanged() error {
	return s.fileStore.RefreshIfChanged()
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
	var tags entity.JSONArray
	if config.Tags != nil {
		tags = make(entity.JSONArray, len(config.Tags))
		for i, tag := range config.Tags {
			tags[i] = tag
		}
	}

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
		Tags:           tags,
		Icon:           config.Icon,
		Enabled:        enabled,
		DisabledTools:  disabledTools,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
	}
}
