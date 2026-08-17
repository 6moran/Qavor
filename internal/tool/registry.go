package tool

import (
	"sync"
	"time"
)

// Registry 工具注册表
type Registry struct {
	mu        sync.RWMutex
	tools     map[string]BuiltinTool
	cached    []ToolMeta
	cacheTime time.Time
}

// CacheTTL 缓存过期时间
const CacheTTL = 5 * time.Minute

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]BuiltinTool),
	}
}

// Register 注册工具
func (r *Registry) Register(t BuiltinTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	meta := t.Meta()
	r.tools[meta.Name] = t
	r.cached = nil // 清除缓存
}

// Get 获取工具
func (r *Registry) Get(name string) (BuiltinTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// ListAll 列出所有工具
func (r *Registry) ListAll() []ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.cached != nil && time.Since(r.cacheTime) < CacheTTL {
		return r.cached
	}

	result := make([]ToolMeta, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t.Meta())
	}
	r.cached = result
	r.cacheTime = time.Now()

	return r.cached
}

// ListByCategory 按分类列出工具
func (r *Registry) ListByCategory(cat Category) []ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ToolMeta
	for _, t := range r.tools {
		meta := t.Meta()
		if meta.Category == cat {
			result = append(result, meta)
		}
	}
	return result
}

// GetNames 获取所有工具名
func (r *Registry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Has 检查工具是否存在
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}
