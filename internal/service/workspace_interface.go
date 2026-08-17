package service

import (
	"context"

	"Qavor/internal/agent/localfs"
)

// WorkspaceEntry 目录树条目（Path 为相对 root 的 / 分隔路径）。
type WorkspaceEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// FileUpload 上传文件（内存缓冲，供 multipart 转交）。
type FileUpload struct {
	Name    string
	Content []byte
}

// WorkspaceService workspace 文件操作服务。
type WorkspaceService interface {
	ListTree(ctx context.Context, path string) ([]WorkspaceEntry, error)
	ReadContent(ctx context.Context, path string) ([]byte, error)
	Save(ctx context.Context, path string, content []byte) (*WorkspaceEntry, error)
	Delete(ctx context.Context, path string) error
	CreateDirectory(ctx context.Context, parentPath, name string) (*WorkspaceEntry, error)
	Upload(ctx context.Context, parentPath string, files []FileUpload) ([]WorkspaceEntry, error)
}

// NewWorkspaceService 创建 workspace 服务。
func NewWorkspaceService(backend *localfs.LocalBackend) WorkspaceService {
	return &workspaceService{backend: backend}
}
