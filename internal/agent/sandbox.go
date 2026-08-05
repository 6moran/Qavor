package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk/filesystem"
)

// DiskSandbox 基于磁盘的文件系统沙箱，限制 agent 只能访问沙箱根目录内的文件。
// 所有路径操作都经过安全校验，防止路径穿越（如 ../../etc/passwd）。
type DiskSandbox struct {
	root string // 沙箱根目录（绝对路径）
}

// NewDiskSandbox 创建磁盘沙箱。
// 如果 root 不存在则自动创建。
func NewDiskSandbox(root string) (*DiskSandbox, error) {
	if root == "" {
		return nil, fmt.Errorf("sandbox root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve sandbox root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox root %q: %w", abs, err)
	}
	return &DiskSandbox{root: abs}, nil
}

// resolve 将请求中的路径解析为沙箱内的绝对路径，并校验不得越出沙箱。
func (s *DiskSandbox) resolve(p string) (string, error) {
	if p == "" {
		p = "."
	}
	// filepath.Join 会清理路径（去除 ../ 等），然后用 Rel 校验仍位于根目录内
	full := filepath.Join(s.root, filepath.FromSlash(p))
	rel, err := filepath.Rel(s.root, full)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", p, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes sandbox root", p)
	}
	return full, nil
}

// fileInfo 将 os.FileInfo 转换为 filesystem.FileInfo。
func (s *DiskSandbox) fileInfo(path string, info os.FileInfo) filesystem.FileInfo {
	rel, _ := filepath.Rel(s.root, path)
	return filesystem.FileInfo{
		Path:       filepath.ToSlash(rel),
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
	}
}

// LsInfo 列出目录下的文件信息。
func (s *DiskSandbox) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	dir, err := s.resolve(req.Path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make([]filesystem.FileInfo, 0, len(entries))
	for _, e := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, s.fileInfo(filepath.Join(dir, e.Name()), info))
	}
	return result, nil
}

// Read 读取文件内容，支持行号偏移和行数限制。
func (s *DiskSandbox) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	path, err := s.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	// 按行裁剪
	if req.Offset > 1 || req.Limit > 0 {
		lines := strings.Split(content, "\n")
		start := req.Offset - 1
		if start < 0 {
			start = 0
		}
		if start >= len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if req.Limit > 0 && start+req.Limit < end {
			end = start + req.Limit
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return &filesystem.FileContent{Content: content}, nil
}

// GrepRaw 在沙箱内递归搜索匹配内容。
func (s *DiskSandbox) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	// 搜索根目录
	searchRoot := s.root
	if req.Path != "" {
		r, err := s.resolve(req.Path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(r)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			searchRoot = r
		} else {
			// 指定的是单个文件，只搜索该文件
			re, err := regexpCompile(req)
			if err != nil {
				return nil, err
			}
			return s.grepFile(ctx, r, re)
		}
	}

	re, err := regexpCompile(req)
	if err != nil {
		return nil, err
	}

	var matches []filesystem.GrepMatch
	glob := req.Glob
	fileType := req.FileType
	if fileType != "" {
		glob = "**/*." + fileType
	}

	err = filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		// 应用 glob 过滤
		if glob != "" {
			rel, _ := filepath.Rel(searchRoot, path)
			match, err := doublestar.PathMatch(glob, filepath.ToSlash(rel))
			if err != nil || !match {
				return nil
			}
		}
		fileMatches, err := s.grepFile(ctx, path, re)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// regexpCompile 编译正则表达式，处理大小写敏感选项。
func regexpCompile(req *filesystem.GrepRequest) (*regexp.Regexp, error) {
	pattern := req.Pattern
	if req.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid grep pattern %q: %w", req.Pattern, err)
	}
	return re, nil
}

// grepFile 搜索单个文件内容。
func (s *DiskSandbox) grepFile(ctx context.Context, path string, re *regexp.Regexp) ([]filesystem.GrepMatch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // 跳过无法读取的文件
	}
	lines := strings.Split(string(data), "\n")
	var result []filesystem.GrepMatch
	for i, line := range lines {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if re.MatchString(line) {
			result = append(result, filesystem.GrepMatch{
				Content: line,
				Path:    filepath.ToSlash(path),
				Line:    i + 1,
			})
		}
	}
	return result, nil
}

// GlobInfo 使用 glob 模式搜索文件。
func (s *DiskSandbox) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	base := s.root
	if req.Path != "" {
		r, err := s.resolve(req.Path)
		if err != nil {
			return nil, err
		}
		base = r
	}

	pattern := req.Pattern
	if !strings.HasPrefix(pattern, "**") && !strings.HasPrefix(pattern, "/") {
		pattern = "**/" + pattern
	}
	fullPattern := filepath.Join(base, filepath.FromSlash(pattern))

	matches, err := doublestar.FilepathGlob(fullPattern, doublestar.WithNoFollow())
	if err != nil {
		return nil, err
	}

	result := make([]filesystem.FileInfo, 0, len(matches))
	for _, m := range matches {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// 校验匹配结果仍在沙箱内
		rel, err := filepath.Rel(s.root, m)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		result = append(result, s.fileInfo(m, info))
	}
	return result, nil
}

// Write 写入或更新文件内容，自动创建父目录。
func (s *DiskSandbox) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	path, err := s.resolve(req.FilePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return err
	}
	return nil
}

// Edit 替换文件中的字符串。
func (s *DiskSandbox) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	if req.OldString == "" {
		return fmt.Errorf("old string must not be empty")
	}
	path, err := s.resolve(req.FilePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	if req.ReplaceAll {
		if !strings.Contains(content, req.OldString) {
			return fmt.Errorf("old string not found in %s", req.FilePath)
		}
		content = strings.ReplaceAll(content, req.OldString, req.NewString)
	} else {
		count := strings.Count(content, req.OldString)
		if count == 0 {
			return fmt.Errorf("old string not found in %s", req.FilePath)
		}
		if count > 1 {
			return fmt.Errorf("old string appears %d times in %s, set ReplaceAll to true", count, req.FilePath)
		}
		content = strings.Replace(content, req.OldString, req.NewString, 1)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
