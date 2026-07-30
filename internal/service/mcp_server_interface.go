package service

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/pkg/response"
)

// MCPServerService MCP服务器服务接口
type MCPServerService interface {
	CreateMCPServer(username string, req *request.CreateMCPServerRequest) (*dto.MCPServerResponse, error)
	GetMCPServer(name string) (*dto.MCPServerResponse, error)
	UpdateMCPServer(name string, username string, req *request.UpdateMCPServerRequest) (*dto.MCPServerResponse, error)
	DeleteMCPServer(name string) error
	ListMCPServers(req *request.MCPServerListRequest) (*response.PageResponse, error)
	EnableMCPServer(name string) error
	DisableMCPServer(name string) error
	RefreshIfChanged() error
}
