package skill

import "sync"

// ActivationState 线程安全的 Skill 激活状态管理
type ActivationState struct {
	mu        sync.RWMutex
	activated map[string]bool
}

// NewActivationState 创建 ActivationState
func NewActivationState() *ActivationState {
	return &ActivationState{
		activated: make(map[string]bool),
	}
}

// Activate 激活指定 Skill
func (s *ActivationState) Activate(slug string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activated[slug] = true
}

// IsActivated 查询指定 Skill 是否已激活
func (s *ActivationState) IsActivated(slug string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activated[slug]
}

// GetActivated 获取所有已激活的 slug 列表
func (s *ActivationState) GetActivated() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []string
	for slug, ok := range s.activated {
		if ok {
			result = append(result, slug)
		}
	}
	return result
}
