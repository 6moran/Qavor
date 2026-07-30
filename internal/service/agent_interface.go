package service

import (
	"Qavor/internal/agent"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/pkg/response"
)

// AgentService 智能体服务接口
type AgentService interface {
	CreateAgent(req *request.CreateAgentRequest) (*dto.AgentResponse, error)
	GetAgent(slug string) (*dto.AgentResponse, error)
	UpdateAgent(slug string, req *request.UpdateAgentRequest) (*dto.AgentResponse, error)
	DeleteAgent(slug string) error
	ListAgents(req *request.AgentListRequest) (*response.PageResponse, error)
	SetDefault(slug string) error
	GetDefaultAgent() (*dto.AgentResponse, error)
	// GetAgentConfig 获取运行时配置（供 AgentManager 使用）
	GetAgentConfig(slug string) (*agent.AgentConfig, error)
}
