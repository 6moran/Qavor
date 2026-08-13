package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"Qavor/internal/model/entity"
)

// MCPServerFileStoreImpl MCP服务器文件存储实现
type MCPServerFileStoreImpl struct {
	mu        sync.RWMutex
	filePath  string
	servers   map[string]*entity.MCPServerConfig
	signature string
}

// mcpFileData mcp.json 文件结构
type mcpFileData struct {
	MCPServers map[string]*entity.MCPServerConfig `json:"mcpServers"`
}

// NewMCPServerFileStore 创建文件存储实例
func NewMCPServerFileStore(workspace string) (*MCPServerFileStoreImpl, error) {
	s := &MCPServerFileStoreImpl{
		servers: make(map[string]*entity.MCPServerConfig),
	}

	// 确定文件路径
	mcpPath := filepath.Join(workspace, "mcp.json")

	if _, err := os.Stat(mcpPath); err == nil {
		s.filePath = mcpPath
	} else {
		// 文件不存在，创建默认的 mcp.json
		s.filePath = mcpPath
		if err := s.writeDefault(); err != nil {
			return nil, err
		}
	}

	// 初始加载
	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// NewEmptyMCPServerFileStore 创建空的文件存储（用于降级）
func NewEmptyMCPServerFileStore() *MCPServerFileStoreImpl {
	return &MCPServerFileStoreImpl{
		servers: make(map[string]*entity.MCPServerConfig),
	}
}

// load 从文件读取配置到内存
func (s *MCPServerFileStoreImpl) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("读取 mcp.json 失败: %w", err)
	}

	var fileData mcpFileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("解析 mcp.json 失败: %w", err)
	}

	s.servers = fileData.MCPServers
	if s.servers == nil {
		s.servers = make(map[string]*entity.MCPServerConfig)
	}

	s.updateSignature()
	return nil
}

// GetAll 返回所有配置（深拷贝）
func (s *MCPServerFileStoreImpl) GetAll() (map[string]*entity.MCPServerConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*entity.MCPServerConfig, len(s.servers))
	for k, v := range s.servers {
		cp := *v
		result[k] = &cp
	}
	return result, nil
}

// GetByName 按 name 获取
func (s *MCPServerFileStoreImpl) GetByName(name string) (*entity.MCPServerConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, ok := s.servers[name]
	if !ok {
		return nil, nil
	}
	cp := *cfg
	return &cp, nil
}

// Create 创建新配置
func (s *MCPServerFileStoreImpl) Create(name string, config *entity.MCPServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.servers[name]; exists {
		return fmt.Errorf("name '%s' 已存在", name)
	}

	now := time.Now().UTC()
	config.CreatedAt = now
	config.UpdatedAt = now
	s.servers[name] = config

	return s.save()
}

// Update 更新配置
func (s *MCPServerFileStoreImpl) Update(name string, updates *entity.MCPServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.servers[name]
	if !ok {
		return fmt.Errorf("name '%s' 不存在", name)
	}

	// 部分字段更新（Name 不允许更新，因为它就是 key）
	if updates.Description != "" {
		existing.Description = updates.Description
	}
	if updates.Transport != "" {
		existing.Transport = updates.Transport
	}
	if updates.URL != "" {
		existing.URL = updates.URL
	}
	if updates.Command != "" {
		existing.Command = updates.Command
	}
	if updates.Args != nil {
		existing.Args = updates.Args
	}
	if updates.Env != nil {
		existing.Env = updates.Env
	}
	if updates.Headers != nil {
		existing.Headers = updates.Headers
	}
	if updates.Timeout != nil {
		existing.Timeout = updates.Timeout
	}
	if updates.SSEReadTimeout != nil {
		existing.SSEReadTimeout = updates.SSEReadTimeout
	}
	// Enabled 字段需要特殊处理，允许设置为 false
	if updates.Enabled != existing.Enabled {
		existing.Enabled = updates.Enabled
	}
	if updates.DisabledTools != nil {
		existing.DisabledTools = updates.DisabledTools
	}
	existing.UpdatedAt = time.Now().UTC()
	return s.save()
}

// Delete 删除配置
func (s *MCPServerFileStoreImpl) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.servers[name]; !exists {
		return fmt.Errorf("name '%s' 不存在", name)
	}

	delete(s.servers, name)
	return s.save()
}

// RefreshIfChanged 检测文件变更，有变化则重新加载
func (s *MCPServerFileStoreImpl) RefreshIfChanged() error {
	newSig := s.computeSignature()
	if newSig == s.signature {
		return nil
	}
	return s.load()
}

// Signature 获取当前文件签名
func (s *MCPServerFileStoreImpl) Signature() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.signature
}

// save 将内存中的配置写回文件
func (s *MCPServerFileStoreImpl) save() error {
	fileData := mcpFileData{MCPServers: s.servers}
	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 mcp.json 失败: %w", err)
	}

	// 原子写入：先写临时文件，再重命名
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("重命名临时文件失败: %w", err)
	}

	s.updateSignature()
	return nil
}

// writeDefault 创建默认的 mcp.json 文件
func (s *MCPServerFileStoreImpl) writeDefault() error {
	// 确保目录存在
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	fileData := mcpFileData{
		MCPServers: make(map[string]*entity.MCPServerConfig),
	}
	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// updateSignature 更新签名
func (s *MCPServerFileStoreImpl) updateSignature() {
	s.signature = s.computeSignature()
}

// computeSignature 计算文件签名（mtime + sha256）
func (s *MCPServerFileStoreImpl) computeSignature() string {
	stat, err := os.Stat(s.filePath)
	if err != nil {
		return ""
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%d-%x", stat.ModTime().UnixNano(), hash)
}
