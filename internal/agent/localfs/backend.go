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
// 做根目录隔离：resolve() 强制路径落在 root 内，越界（含符号链接解析后越界）拒绝。
// 安全叠加 security.Policies 管控层（敏感文件守卫、语法预检等）。
type LocalBackend struct {
	root       string // 工作区根 data/workspaces/<slug>（相对路径基准）
	skillsRoot string // 系统技能源目录，符号链接白名单基准（空=白名单关闭）
	sec        *security.Policies
}

// NewLocalBackend 创建本地文件系统后端。
func NewLocalBackend(root string, sec *security.Policies) *LocalBackend {
	return &LocalBackend{root: root, sec: sec}
}

// SetSkillsRoot 设置技能源目录，启用 skills/ 符号链接白名单。
func (b *LocalBackend) SetSkillsRoot(dir string) { b.skillsRoot = dir }

// withinRoot 判断 path 是否等于 root 或在其子路径内。
// 统一转绝对路径比较，避免相对 root（如 data/workspaces）匹配不上绝对请求路径。
// 用 root+分隔符 前缀判断，避免 root 前缀误匹配兄弟目录（如 data/workspaces vs data/workspaces-evil）。
func (b *LocalBackend) withinRoot(path string) bool {
	root := filepath.Clean(b.root)
	if absRoot, err := filepath.Abs(root); err == nil {
		root = absRoot
	}
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// resolve 解析请求路径为磁盘绝对路径，并强制校验在 root 内。
// 相对路径以 root 为基准；绝对路径原样解析。
// 越界（含符号链接解析后越界）返回统一错误。技能符号链接白名单见 isWhitelistedSkillPath。
func (b *LocalBackend) resolve(p string) (string, error) {
	if p == "" {
		p = "."
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(b.root, filepath.FromSlash(p)))
	}
	if !b.withinRoot(abs) {
		return "", fmt.Errorf("path escapes workspace root: %s", p)
	}
	// 技能白名单：跳过符号链接目标必须落在 root 内的校验
	if b.isWhitelistedSkillPath(abs) {
		return abs, nil
	}
	// 符号链接解析：真实路径也必须在 root 内
	if real, err := filepath.EvalSymlinks(abs); err == nil && !b.withinRoot(real) {
		return "", fmt.Errorf("path escapes workspace root via symlink: %s", p)
	}
	return abs, nil
}

// resolveForWrite 解析写入路径：先走 resolve() 完整校验（含文件自身符号链接），
// 再对最深已存在的祖先目录做 EvalSymlinks 检查，防止父级符号链接指向 root 外。
// 技能符号链接白名单为只读：命中白名单的写请求拒绝。
func (b *LocalBackend) resolveForWrite(p string) (string, error) {
	abs, err := b.resolve(p)
	if err != nil {
		return "", err
	}
	if b.isWhitelistedSkillPath(abs) {
		return "", fmt.Errorf("path is read-only (skill symlink): %s", p)
	}
	if err := b.ensureParentWithinRoot(p, abs); err != nil {
		return "", err
	}
	return abs, nil
}

// ensureParentWithinRoot 从目标文件的父目录向上找到第一个存在的路径，解析真实路径；
// 若越界（不在 root 内、也不在技能白名单内）则拒绝。p 为原始请求路径，用于错误回显。
// 最深已存在祖先命中技能白名单（父级是指向 skillsRoot 的符号链接）时同样拒绝——
// 技能目录只读，且新文件本身不存在时 isWhitelistedSkillPath(abs) 无法命中，需在此兜底。
func (b *LocalBackend) ensureParentWithinRoot(p, abs string) error {
	parent := filepath.Dir(abs)
	// 找到最深已存在路径
	for {
		if _, err := os.Lstat(parent); err == nil {
			break
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}
	if b.isWhitelistedSkillPath(parent) {
		return fmt.Errorf("path is read-only (skill symlink): %s", p)
	}
	real, err := filepath.EvalSymlinks(parent)
	if err != nil {
		// 父目录不可解析（悬空符号链接/权限受限）时按 root 内处理：
		// 后续 MkdirAll/WriteFile 会因此失败，写入不会真正发生，无安全漏洞。
		return nil
	}
	if !b.withinRoot(real) {
		return fmt.Errorf("path escapes workspace root via parent symlink: %s", p)
	}
	return nil
}

// isWhitelistedSkillPath 判断路径是否属于技能符号链接白名单。
// 仅当路径位于 root/skills/ 下，且符号链接真实目标在 skillsRoot 内时放行，
// 防止任意符号链接逃逸出 root。
// 注意：该白名单仅用于读路径（resolve）；写路径在 resolveForWrite 中命中白名单即拒绝（只读）。
func (b *LocalBackend) isWhitelistedSkillPath(abs string) bool {
	if b.skillsRoot == "" {
		return false
	}
	skillsPrefix := filepath.Join(b.root, "skills") + string(filepath.Separator)
	if !strings.HasPrefix(abs, skillsPrefix) {
		return false
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return false
	}
	cleanedRoot := filepath.Clean(b.skillsRoot)
	if real == cleanedRoot {
		return true
	}
	return strings.HasPrefix(real, cleanedRoot+string(filepath.Separator))
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
	if !b.withinRoot(fullPattern) {
		return nil, fmt.Errorf("path escapes workspace root: %s", pattern)
	}

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
	path, err := b.resolveForWrite(req.FilePath)
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
	path, err := b.resolveForWrite(req.FilePath)
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
	path, err := b.resolveForWrite(req.FilePath)
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

// ResolvePath 公开路径校验：返回磁盘绝对路径（供下载/上传用原始字节访问）。
// 复用 resolve() 的根隔离校验。
func (b *LocalBackend) ResolvePath(p string) (string, error) {
	return b.resolve(p)
}

// Delete 删除文件或目录（递归），拒绝删除 root 本身。
// 越界路径经 resolve() 校验拒绝。
func (b *LocalBackend) Delete(ctx context.Context, p string) error {
	abs, err := b.resolve(p)
	if err != nil {
		return err
	}
	// root 拒绝：统一转绝对路径比较，与 withinRoot 的归一化模式对齐，
	// 避免相对 root 下用绝对形式访问 root 时（resolve 原样返回绝对路径）绕过检查。
	absRoot, _ := filepath.Abs(filepath.Clean(b.root))
	absPath, _ := filepath.Abs(abs)
	if absPath == absRoot {
		return fmt.Errorf("cannot delete workspace root")
	}
	return os.RemoveAll(abs)
}

// CreateDirectory 创建目录（过根隔离校验），自动创建父目录。
func (b *LocalBackend) CreateDirectory(ctx context.Context, p string) error {
	abs, err := b.resolveForWrite(p)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}
