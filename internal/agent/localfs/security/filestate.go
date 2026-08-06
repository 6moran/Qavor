package security

import (
	"sync"
	"time"

	"Qavor/pkg/config"
)

const fileStateMaxEntries = 512

// FileState 陈旧警告状态跟踪。
// 记录"agent 上次读取某文件的 mtime"，写前检测文件是否被外部修改（只警告不阻断）。
type FileState struct {
	enabled bool
	mu      sync.RWMutex
	mtimes  map[string]time.Time
}

func newFileState(cfg config.StalenessConfig, base bool) *FileState {
	return &FileState{
		enabled: base && (cfg.Enabled == nil || *cfg.Enabled),
		mtimes:  make(map[string]time.Time),
	}
}

// NoteRead 记录读取某文件的 mtime。
func (f *FileState) NoteRead(path string, mtime time.Time) {
	if f == nil || !f.enabled {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(path, mtime)
}

// NoteWrite 记录写入某文件的 mtime。
func (f *FileState) NoteWrite(path string, mtime time.Time) {
	f.NoteRead(path, mtime)
}

// StalenessWarning 返回外部修改警告（仅警告不阻断）。无记录或未过期返回空串。
func (f *FileState) StalenessWarning(path string, current time.Time) string {
	if f == nil || !f.enabled {
		return ""
	}
	f.mu.RLock()
	prev, ok := f.mtimes[path]
	f.mu.RUnlock()
	if !ok || !current.After(prev) {
		return ""
	}
	return "文件已被外部修改（自上次读取后），请重新读取后再编辑"
}

func (f *FileState) record(path string, mtime time.Time) {
	if len(f.mtimes) >= fileStateMaxEntries {
		f.evict()
	}
	f.mtimes[path] = mtime
}

// evict 按记录时间淘汰最早的一条（简单 LRU 近似）。
func (f *FileState) evict() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range f.mtimes {
		if first || v.Before(oldestTime) {
			oldestKey = k
			oldestTime = v
			first = false
		}
	}
	if oldestKey != "" {
		delete(f.mtimes, oldestKey)
	}
}
