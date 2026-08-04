package remote

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// GitHubProvider 从 GitHub 仓库拉取 skills（tarball 方案，一次请求，零额外依赖）。
type GitHubProvider struct {
	client *http.Client
	token  string // 可选：GitHub token，私有/高限流仓库用
}

// NewGitHubProvider 创建 GitHub 拉取源
func NewGitHubProvider(token string) *GitHubProvider {
	return &GitHubProvider{
		client: &http.Client{Timeout: 60 * time.Second},
		token:  token,
	}
}

var repoRe = regexp.MustCompile(
	`^(?:https?://(?:www\.)?github\.com/)?([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/?$`)

func (p *GitHubProvider) Name() string { return "github" }

// Recognize 识别 owner/repo 或 github.com/owner/repo
func (p *GitHubProvider) Recognize(source string) bool {
	return repoRe.MatchString(strings.TrimSpace(source))
}

func (p *GitHubProvider) parse(source string) (owner, repo string, err error) {
	m := repoRe.FindStringSubmatch(strings.TrimSpace(source))
	if len(m) != 3 {
		return "", "", fmt.Errorf("无法识别 GitHub 仓库: %s", source)
	}
	return m[1], m[2], nil
}

// defaultBranch 查询仓库默认分支
func (p *GitHubProvider) defaultBranch(owner, repo string) (string, error) {
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo), nil)
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("查询仓库 %s/%s 失败: %s", owner, repo, resp.Status)
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.DefaultBranch, nil
}

// fetchTarball 下载仓库 tarball 字节
func (p *GitHubProvider) fetchTarball(owner, repo, ref string) ([]byte, error) {
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", owner, repo, ref), nil)
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("拉取仓库 %s/%s 失败: %s", owner, repo, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// scanTarball 解析 tarball：剥掉顶层 {repo}-{ref} 目录，返回 slug -> 文件映射
// 支持三种结构：
// 1. 根目录下有 SKILL.md（slug = "root"）
// 2. 单层子目录下有 SKILL.md（slug = 目录名）
// 3. 多层子目录下有 SKILL.md（slug = SKILL.md 所在目录名）
func scanTarball(data []byte) (map[string]map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	// 第一遍：收集所有文件，并找到 SKILL.md 所在的目录
	type fileEntry struct {
		relPath string
		content []byte
	}
	var allFiles []fileEntry
	skillDirs := map[string]bool{} // 记录包含 SKILL.md 的目录

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		rest := strings.SplitN(hdr.Name, "/", 2)
		if len(rest) < 2 {
			continue
		}
		relPath := rest[1]

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}

		allFiles = append(allFiles, fileEntry{relPath: relPath, content: content})

		// 检查是否是 SKILL.md
		if relPath == "SKILL.md" {
			skillDirs[""] = true // 根目录
		} else if strings.HasSuffix(relPath, "/SKILL.md") {
			dir := strings.TrimSuffix(relPath, "/SKILL.md")
			skillDirs[dir] = true
		}
	}

	// 如果没有找到 SKILL.md，返回空
	if len(skillDirs) == 0 {
		return map[string]map[string][]byte{}, nil
	}

	// 第二遍：根据 SKILL.md 的位置分组文件
	out := map[string]map[string][]byte{}
	for _, f := range allFiles {
		slug := resolveSlug(f.relPath, skillDirs)
		if slug == "" {
			continue
		}
		if out[slug] == nil {
			out[slug] = map[string][]byte{}
		}
		out[slug][f.relPath] = f.content
	}
	return out, nil
}

// resolveSlug 根据文件路径和 SKILL.md 目录列表，确定文件所属的 slug
func resolveSlug(relPath string, skillDirs map[string]bool) string {
	// 检查根目录
	if skillDirs[""] {
		return "root"
	}

	// 检查子目录
	parts := strings.Split(relPath, "/")
	for i := 1; i <= len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if skillDirs[dir] {
			return parts[i-1] // 返回目录名作为 slug
		}
	}
	return ""
}

// parseFrontmatterMeta 从 SKILL.md frontmatter 提取 name/description（复用 loader 能力，此处轻量解析）
func parseFrontmatterMeta(content string) (name, desc string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, "\"'")
		} else if strings.HasPrefix(line, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			desc = strings.Trim(desc, "\"'")
		}
	}
	return name, desc
}

// List 列出仓库内所有含 SKILL.md 的 skill 目录
func (p *GitHubProvider) List(source string) ([]RemoteSkillMeta, error) {
	owner, repo, err := p.parse(source)
	if err != nil {
		return nil, err
	}
	ref, err := p.defaultBranch(owner, repo)
	if err != nil {
		return nil, err
	}
	data, err := p.fetchTarball(owner, repo, ref)
	if err != nil {
		return nil, err
	}
	files, err := scanTarball(data)
	if err != nil {
		return nil, err
	}

	var metas []RemoteSkillMeta
	for slug, entries := range files {
		// 查找 SKILL.md：可能是 slug/SKILL.md 或 SKILL.md（根目录）
		var md []byte
		var ok bool
		if slug == "root" {
			md, ok = entries["SKILL.md"]
		} else {
			md, ok = entries[slug+"/SKILL.md"]
			if !ok {
				// 也可能是相对路径中的 SKILL.md
				for path, content := range entries {
					if strings.HasSuffix(path, "/SKILL.md") {
						md = content
						ok = true
						break
					}
				}
			}
		}
		if !ok {
			continue
		}
		name, desc := parseFrontmatterMeta(string(md))
		metas = append(metas, RemoteSkillMeta{
			Slug:        slug,
			Name:        name,
			Description: desc,
			Source:      source,
		})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Slug < metas[j].Slug })
	return metas, nil
}

// Fetch 把单个 skill 目录打包成 zip 返回
func (p *GitHubProvider) Fetch(source, slug string) ([]byte, error) {
	owner, repo, err := p.parse(source)
	if err != nil {
		return nil, err
	}
	ref, err := p.defaultBranch(owner, repo)
	if err != nil {
		return nil, err
	}
	data, err := p.fetchTarball(owner, repo, ref)
	if err != nil {
		return nil, err
	}
	files, err := scanTarball(data)
	if err != nil {
		return nil, err
	}
	entries, ok := files[slug]
	if !ok {
		return nil, fmt.Errorf("仓库中不存在 skill: %s", slug)
	}
	return ZipSkillDir(slug, entries, files["root"] != nil)
}

// ZipSkillDir 把 skill 目录文件打包成 zip（供 remote 拉取与本地导出复用）
func ZipSkillDir(slug string, entries map[string][]byte, isRoot ...bool) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// 稳定排序保证输出可重现
	paths := make([]string, 0, len(entries))
	for rel := range entries {
		if len(isRoot) > 0 && isRoot[0] {
			// 根目录模式：包含所有文件
			paths = append(paths, rel)
		} else {
			// 子目录模式：包含 SKILL.md 文件
			if strings.HasSuffix(rel, "/SKILL.md") || rel == "SKILL.md" {
				paths = append(paths, rel)
			}
		}
	}
	sort.Strings(paths)
	for _, rel := range paths {
		var zipPath string
		if len(isRoot) > 0 && isRoot[0] {
			// 根目录模式：保持原路径
			zipPath = rel
		} else {
			// 子目录模式：提取 SKILL.md 作为文件名
			if strings.HasSuffix(rel, "/SKILL.md") {
				zipPath = "SKILL.md"
			} else {
				zipPath = rel
			}
		}
		w, err := zw.Create(zipPath)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(entries[rel]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
