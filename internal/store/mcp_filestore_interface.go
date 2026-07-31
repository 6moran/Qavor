package store

import "Qavor/internal/model/entity"

// MCPServerFileStore MCP服务器文件存储接口
type MCPServerFileStore interface {
	// GetAll 读取所有 MCP Server 配置
	GetAll() (map[string]*entity.MCPServerConfig, error)

	// GetByName 按 name 获取单个配置
	GetByName(name string) (*entity.MCPServerConfig, error)

	// Create 创建配置（name 不能重复）
	Create(name string, config *entity.MCPServerConfig) error

	// Update 更新配置（部分更新）
	Update(name string, config *entity.MCPServerConfig) error

	// Delete 删除配置
	Delete(name string) error

	// RefreshIfChanged 检查文件是否有变更，有变更则重新加载
	RefreshIfChanged() error

	// Signature 获取当前文件签名（用于外部检测）
	Signature() string
}
