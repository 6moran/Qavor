package localfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"Qavor/internal/agent/localfs/security"
	"Qavor/pkg/logger"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk/filesystem"
	"go.uber.org/zap"
)

// maxReadBytes 超过该大小的文件不读内容，仅返回元数据提示（避免把大文件灌进上下文）。
const maxReadBytes = 50 * 1024 * 1024

// LocalBackend 基于本地磁盘的 filesystem.Backend。
// 不做根目录隔离（与 CowAgent 语义一致），安全完全依赖 security.Policies 管控层。
type LocalBackend struct {
	root string // 工作区根 data/workspaces/<slug>（相对路径基准）
	sec  *security.Policies
}

// NewLocalBackend 创建本地文件系统后端。
func NewLocalBackend(root string, sec *security.Policies) *LocalBackend {
	return &LocalBackend{root: root, sec: sec}
}

// resolve 解析请求路径为磁盘绝对路径。
// 相对路径以 root 为基准；绝对路径原样解析；不做穿越校验（无根目录隔离语义）。
func (b *LocalBackend) resolve(p string) (string, error) {
	if p == "" {
		p = "."
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Join(b.root, filepath.FromSlash(p)), nil
}

// checkCredentials 检查路径是否命中敏感文件，命中返回统一拒绝错误。
// 同时检查 filepath.Clean 与 EvalSymlinks 后的真实路径，防符号链接绕过。
func (b *LocalBackend) checkCredentials(path string) error {
	cred := b.credentials()
	if cred == nil {
		return nil
	}
	if cred.IsSensitive(filepath.Clean(path)) {
		return cred.DenyMessage(path)
	}
	if real, err := filepath.EvalSymlinks(path); err == nil && real != path {
		if cred.IsSensitive(real) {
			return cred.DenyMessage(path)
		}
	}
	return nil
}

func (b *LocalBackend) credentials() *security.Credentials {
	if b.sec == nil {
		return nil
	}
	return b.sec.Credentials()
}

func (b *LocalBackend) noteRead(path string) {
	if b.sec == nil {
		return
	}
	if st, err := os.Stat(path); err == nil {
		b.sec.Staleness().NoteRead(path, st.ModTime().UTC())
	}
}

func (b *LocalBackend) noteWrite(path string) {
	if b.sec == nil {
		return
	}
	if st, err := os.Stat(path); err == nil {
		b.sec.Staleness().NoteWrite(path, st.ModTime().UTC())
	}
}

// stalenessWarning 返回写前的陈旧警告（只警告不阻断）。
func (b *LocalBackend) stalenessWarning(path string) string {
	if b.sec == nil {
		return ""
	}
	if st, err := os.Stat(path); err == nil {
		return b.sec.Staleness().StalenessWarning(path, st.ModTime().UTC())
	}
	return ""
}

func (b *LocalBackend) fileInfo(path string, info os.FileInfo) filesystem.FileInfo {
	p := path
	if rel, err := filepath.Rel(b.root, path); err == nil {
		p = filepath.ToSlash(rel)
	}
	return filesystem.FileInfo{
		Path:       p,
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
	}
}

// LsInfo 列出目录下的文件信息。
func (b *LocalBackend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	dir, err := b.resolve(req.Path)
	if err != nil {
		return nil, err
	}
	if err := b.checkCredentials(dir); err != nil {
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
			continue
		}
		result = append(result, b.fileInfo(filepath.Join(dir, e.Name()), info))
	}
	return result, nil
}

// Read 读取文件内容，支持行号偏移和行数限制。
// 超过 maxReadBytes 的文件返回元数据提示，不读内容。
func (b *LocalBackend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	path, err := b.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	if err := b.checkCredentials(path); err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%s is a directory", req.FilePath)
	}
	if st.Size() > maxReadBytes {
		return &filesystem.FileContent{
			Content: fmt.Sprintf("文件超过 50MB（%d 字节），请使用 shell 命令查看", st.Size()),
		}, nil
	}
	b.noteRead(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
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

// GrepRaw 递归搜索匹配内容，跳过敏感文件。
func (b *LocalBackend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	searchRoot := b.root
	if req.Path != "" {
		r, err := b.resolve(req.Path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(r)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			re, err := regexpCompile(req)
			if err != nil {
				return nil, err
			}
			return b.grepFile(ctx, r, re)
		}
		searchRoot = r
	}
	if err := b.checkCredentials(searchRoot); err != nil {
		return nil, err
	}

	re, err := regexpCompile(req)
	if err != nil {
		return nil, err
	}
	glob := req.Glob
	if req.FileType != "" {
		glob = "**/*." + req.FileType
	}

	var matches []filesystem.GrepMatch
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
		if b.credentials() != nil && b.credentials().IsSensitive(path) {
			return nil
		}
		if glob != "" {
			rel, _ := filepath.Rel(searchRoot, path)
			match, err := doublestar.PathMatch(glob, filepath.ToSlash(rel))
			if err != nil || !match {
				return nil
			}
		}
		fileMatches, err := b.grepFile(ctx, path, re)
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
func (b *LocalBackend) grepFile(ctx context.Context, path string, re *regexp.Regexp) ([]filesystem.GrepMatch, error) {
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

// GlobInfo 使用 glob 模式搜索文件，过滤敏感路径。
func (b *LocalBackend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	base := b.root
	if req.Path != "" {
		r, err := b.resolve(req.Path)
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
		if b.credentials() != nil && b.credentials().IsSensitive(m) {
			continue
		}
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		result = append(result, b.fileInfo(m, info))
	}
	return result, nil
}

// Write 写入或更新文件内容，自动创建父目录。
// 依次执行：凭据守卫 → 行号前缀检测 → 陈旧警告 → 语法预检。
func (b *LocalBackend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	path, err := b.resolve(req.FilePath)
	if err != nil {
		return err
	}
	if err := b.checkCredentials(path); err != nil {
		return err
	}
	if lp := b.sec.LinePrefix(); lp != nil && lp.LooksLikeLineNumberedBlock(req.Content) {
		return fmt.Errorf("content looks like a line-numbered block; strip line numbers before writing")
	}
	if warn := b.stalenessWarning(path); warn != "" {
		logger.Warn(warn, zap.String("path", path))
	}
	oldContent := ""
	if old, err := os.ReadFile(path); err == nil {
		oldContent = string(old)
	}
	if sr := b.sec.Syntax().Validate(path, oldContent, req.Content); sr.Block {
		return fmt.Errorf("%s", sr.Message)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return err
	}
	b.noteWrite(path)
	return nil
}

// Edit 替换文件中的字符串。
// 依次执行：凭据守卫 → 陈旧警告 → 语法预检。
func (b *LocalBackend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	if req.OldString == "" {
		return fmt.Errorf("old string must not be empty")
	}
	path, err := b.resolve(req.FilePath)
	if err != nil {
		return err
	}
	if err := b.checkCredentials(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	oldContent := string(data)
	content := oldContent
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
	if sr := b.sec.Syntax().Validate(path, oldContent, content); sr.Block {
		return fmt.Errorf("%s", sr.Message)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	b.noteWrite(path)
	return nil
}

// ctxCloseWriter 包装 *os.File，在 ctx 取消时自动关闭（满足 AppendOpener 契约），
// Close 幂等，Write after Close 返回错误。
type ctxCloseWriter struct {
	f    *os.File
	ctx  context.Context
	once sync.Once
}

func (w *ctxCloseWriter) Write(p []byte) (int, error) { return w.f.Write(p) }

func (w *ctxCloseWriter) WriteString(s string) (int, error) { return w.f.WriteString(s) }

func (w *ctxCloseWriter) Flush() error { return w.f.Sync() }

func (w *ctxCloseWriter) Close() error {
	var err error
	w.once.Do(func() { err = w.f.Close() })
	return err
}

// OpenAppend 打开追加流（后台任务输出落盘）。
func (b *LocalBackend) OpenAppend(ctx context.Context, req *filesystem.OpenAppendRequest) (io.WriteCloser, error) {
	path, err := b.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	if err := b.checkCredentials(path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := &ctxCloseWriter{f: f, ctx: ctx}
	go func() {
		<-ctx.Done()
		_ = w.Close()
	}()
	return w, nil
}
