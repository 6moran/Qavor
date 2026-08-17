package tool

// NewDefaultRegistry 创建默认注册表
// 需要通过 Register 方法手动注册工具
func NewDefaultRegistry() *Registry {
	return NewRegistry()
}

// RegisterFromProvider 从 ToolProvider 注册工具
func (r *Registry) RegisterFromProvider(provider ToolProvider) {
	for _, t := range provider.GetTools() {
		r.Register(t)
	}
}
