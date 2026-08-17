package service

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"Qavor/internal/agent/localfs"

	"github.com/cloudwego/eino/adk/filesystem"
)

// workspaceService 包装 LocalBackend，提供工作区文件操作。
type workspaceService struct {
	backend *localfs.LocalBackend
}

// normalizePath 规范化前端路径：空串或 "/" 视为 root；返回相对 root 的 / 分隔路径。
func normalizePath(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}

// entryFromFileInfo 映射目录条目。
func entryFromFileInfo(info filesystem.FileInfo) WorkspaceEntry {
	return WorkspaceEntry{
		Path:       info.Path,
		Name:       filepath.Base(info.Path),
		IsDir:      info.IsDir,
		Size:       info.Size,
		ModifiedAt: info.ModifiedAt,
	}
}

func (s *workspaceService) ListTree(ctx context.Context, path string) ([]WorkspaceEntry, error) {
	infos, err := s.backend.LsInfo(ctx, &filesystem.LsInfoRequest{Path: normalizePath(path)})
	if err != nil {
		return nil, err
	}
	entries := make([]WorkspaceEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, entryFromFileInfo(info))
	}
	return entries, nil
}

func (s *workspaceService) ReadContent(ctx context.Context, path string) ([]byte, error) {
	fc, err := s.backend.Read(ctx, &filesystem.ReadRequest{FilePath: normalizePath(path)})
	if err != nil {
		return nil, err
	}
	return []byte(fc.Content), nil
}

func (s *workspaceService) Save(ctx context.Context, path string, content []byte) (*WorkspaceEntry, error) {
	if err := s.backend.Write(ctx, &filesystem.WriteRequest{FilePath: normalizePath(path), Content: string(content)}); err != nil {
		return nil, err
	}
	return s.statEntry(ctx, path)
}

func (s *workspaceService) statEntry(ctx context.Context, path string) (*WorkspaceEntry, error) {
	abs, err := s.backend.ResolvePath(normalizePath(path))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	entry := entryFromFileInfo(filesystem.FileInfo{
		Path:       normalizePath(path),
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
	})
	return &entry, nil
}

func (s *workspaceService) Delete(ctx context.Context, path string) error {
	return s.backend.Delete(ctx, normalizePath(path))
}

func (s *workspaceService) CreateDirectory(ctx context.Context, parentPath, name string) (*WorkspaceEntry, error) {
	parent := normalizePath(parentPath)
	full := parent
	if name != "" {
		full = filepath.ToSlash(filepath.Join(filepath.FromSlash(parent), name))
	}
	if err := s.backend.CreateDirectory(ctx, full); err != nil {
		return nil, err
	}
	return s.statEntry(ctx, full)
}

func (s *workspaceService) Upload(ctx context.Context, parentPath string, files []FileUpload) ([]WorkspaceEntry, error) {
	parent := normalizePath(parentPath)
	entries := make([]WorkspaceEntry, 0, len(files))
	for _, f := range files {
		if f.Name == "" {
			continue
		}
		full := filepath.ToSlash(filepath.Join(filepath.FromSlash(parent), f.Name))
		if err := s.backend.Write(ctx, &filesystem.WriteRequest{FilePath: full, Content: string(f.Content)}); err != nil {
			return nil, err
		}
		entry, err := s.statEntry(ctx, full)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	return entries, nil
}
