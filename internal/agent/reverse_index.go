// internal/agent/reverse_index.go
package agent

import "sync"

// ReverseIndex 维护 子智能体 slug → 引用它的主智能体 slug 集合。
// 子智能体被某主智能体挂载时 AddParent；主智能体缓存失效/删除时 RemoveParent。
// 并发安全（AgentManager 可能在多个请求下并发失效）。
type ReverseIndex struct {
	mu   sync.RWMutex
	subs map[string]map[string]struct{}
}

// NewReverseIndex 创建反向索引。
func NewReverseIndex() *ReverseIndex {
	return &ReverseIndex{subs: make(map[string]map[string]struct{})}
}

// AddParent 记录 subSlug 被 parentSlug 引用。幂等。
func (r *ReverseIndex) AddParent(subSlug, parentSlug string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.subs[subSlug] == nil {
		r.subs[subSlug] = make(map[string]struct{})
	}
	r.subs[subSlug][parentSlug] = struct{}{}
}

// RemoveParent 移除 subSlug 对 parentSlug 的引用。
func (r *ReverseIndex) RemoveParent(subSlug, parentSlug string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if set, ok := r.subs[subSlug]; ok {
		delete(set, parentSlug)
		if len(set) == 0 {
			delete(r.subs, subSlug)
		}
	}
}

// ParentsOf 返回引用 subSlug 的所有主智能体 slug。顺序无关。
func (r *ReverseIndex) ParentsOf(subSlug string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set, ok := r.subs[subSlug]
	if !ok {
		return nil
	}
	parents := make([]string, 0, len(set))
	for p := range set {
		parents = append(parents, p)
	}
	return parents
}
