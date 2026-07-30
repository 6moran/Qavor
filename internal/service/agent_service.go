package service

import (
	"encoding/json"
	"fmt"

	"Qavor/internal/agent"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/response"
)

type agentService struct {
	agentRepo repository.AgentRepository
}

func NewAgentService(agentRepo repository.AgentRepository) AgentService {
	return &agentService{agentRepo: agentRepo}
}

func (s *agentService) CreateAgent(req *request.CreateAgentRequest) (*dto.AgentResponse, error) {
	existing, _ := s.agentRepo.GetBySlug(req.Slug)
	if existing != nil {
		return nil, bizerrors.New(bizerrors.CodeResourceAlreadyExists, "Agent 已存在")
	}

	cfg := agent.AgentConfig{
		Name:          req.Name,
		Description:   req.Description,
		Instruction:   req.Instruction,
		ProviderID:    req.ProviderID,
		ModelName:     req.ModelName,
		Tools:         req.Tools,
		DisabledTools: req.DisabledTools,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		Metadata:      req.Metadata,
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	var cfgMap entity.JSON
	if err := json.Unmarshal(cfgJSON, &cfgMap); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	a := &entity.Agent{
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		ConfigJSON:  cfgMap,
		IsDefault:   req.IsDefault,
	}

	if req.IsDefault {
		if err := s.agentRepo.ClearDefault(); err != nil {
			return nil, err
		}
	}

	if err := s.agentRepo.Create(a); err != nil {
		return nil, err
	}

	return s.toResponse(a), nil
}

func (s *agentService) GetAgent(slug string) (*dto.AgentResponse, error) {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}
	return s.toResponse(a), nil
}

func (s *agentService) UpdateAgent(slug string, req *request.UpdateAgentRequest) (*dto.AgentResponse, error) {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}

	cfg := s.parseConfig(a.ConfigJSON)

	if req.Name != nil {
		a.Name = *req.Name
		cfg.Name = *req.Name
	}
	if req.Description != nil {
		a.Description = *req.Description
		cfg.Description = *req.Description
	}
	if req.Icon != nil {
		a.Icon = *req.Icon
	}
	if req.Instruction != nil {
		cfg.Instruction = *req.Instruction
	}
	if req.ProviderID != nil {
		cfg.ProviderID = *req.ProviderID
	}
	if req.ModelName != nil {
		cfg.ModelName = *req.ModelName
	}
	if req.Tools != nil {
		cfg.Tools = req.Tools
	}
	if req.DisabledTools != nil {
		cfg.DisabledTools = req.DisabledTools
	}
	if req.MaxTokens != nil {
		cfg.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		cfg.Temperature = *req.Temperature
	}
	if req.Metadata != nil {
		cfg.Metadata = req.Metadata
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	var cfgMap entity.JSON
	if err := json.Unmarshal(cfgJSON, &cfgMap); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	a.ConfigJSON = cfgMap

	if err := s.agentRepo.Update(a); err != nil {
		return nil, err
	}

	return s.toResponse(a), nil
}

func (s *agentService) DeleteAgent(slug string) error {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return err
	}
	if a == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}
	return s.agentRepo.Delete(slug)
}

func (s *agentService) ListAgents(req *request.AgentListRequest) (*response.PageResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	offset := (page - 1) * size
	agents, total, err := s.agentRepo.List(offset, size, req.Keyword)
	if err != nil {
		return nil, err
	}

	items := make([]dto.AgentResponse, len(agents))
	for i, a := range agents {
		items[i] = *s.toResponse(a)
	}

	return response.NewPageResponse(items, total, page, size), nil
}

func (s *agentService) SetDefault(slug string) error {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return err
	}
	if a == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}

	if err := s.agentRepo.ClearDefault(); err != nil {
		return err
	}
	return s.agentRepo.SetDefault(slug)
}

func (s *agentService) GetDefaultAgent() (*dto.AgentResponse, error) {
	a, err := s.agentRepo.GetDefault()
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "未设置默认 Agent")
	}
	return s.toResponse(a), nil
}

func (s *agentService) GetAgentConfig(slug string) (*agent.AgentConfig, error) {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}
	cfg := s.parseConfig(a.ConfigJSON)
	return &cfg, nil
}

func (s *agentService) parseConfig(raw entity.JSON) agent.AgentConfig {
	var cfg agent.AgentConfig
	if raw == nil {
		return cfg
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func (s *agentService) toResponse(a *entity.Agent) *dto.AgentResponse {
	cfg := s.parseConfig(a.ConfigJSON)
	return &dto.AgentResponse{
		Slug:        a.Slug,
		Name:        a.Name,
		Description: a.Description,
		Icon:        a.Icon,
		IsDefault:   a.IsDefault,
		Config:      cfg,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
