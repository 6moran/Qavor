package skill

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"Qavor/internal/model/entity"
)

// InstallResult 安装结果
type InstallResult struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

// InstallService Skill 安装服务
type InstallService struct {
	repo   SkillRepository
	loader SkillLoader
}

// NewInstallService 创建安装服务
func NewInstallService(repo SkillRepository, loader SkillLoader) *InstallService {
	return &InstallService{repo: repo, loader: loader}
}

// ---------- zip 解析 ----------

// zipLayout 解析 zip：返回 slug -> 目录内文件（相对 slug）。
func zipLayout(data []byte) (map[string]map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("解析 zip 失败: %w", err)
	}
	var names []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		names = append(names, filepath.ToSlash(f.Name))
	}
	top := commonTopDir(names)

	out := map[string]map[string][]byte{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rel := filepath.ToSlash(f.Name)
		if top != "" {
			rel = strings.TrimPrefix(rel, top)
		}
		parts := strings.Split(rel, "/")

		var slug, file string
		if len(parts) < 2 {
			slug = "root"
			file = rel
		} else {
			slug, file = parts[0], strings.Join(parts[1:], "/")
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if out[slug] == nil {
			out[slug] = map[string][]byte{}
		}
		out[slug][file] = content
	}
	return out, nil
}

// commonTopDir 计算文件路径的公共顶层目录
func commonTopDir(names []string) string {
	if len(names) == 0 {
		return ""
	}
	first := strings.SplitN(names[0], "/", 2)
	if len(first) < 2 {
		return ""
	}
	cand := first[0]
	for _, n := range names[1:] {
		if !strings.HasPrefix(n, cand+"/") {
			return ""
		}
	}
	return cand + "/"
}

// frontmatterNameDesc 从 SKILL.md frontmatter 提取 name/description
func frontmatterNameDesc(content string) (string, string) {
	var name, desc string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), "\"")
		} else if strings.HasPrefix(line, "description:") {
			desc = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"")
		}
	}
	return name, desc
}

// ---------- 上传 zip ----------

// InstallFromZip 上传 zip 并直接安装
func (s *InstallService) InstallFromZip(fileData []byte, filename string) ([]*InstallResult, error) {
	layout, err := zipLayout(fileData)
	if err != nil {
		return nil, err
	}

	var items []*InstallResult
	for slug, entries := range layout {
		// 检查是否有 SKILL.md
		md, ok := entries["SKILL.md"]
		if !ok {
			items = append(items, &InstallResult{
				Name:    slug,
				Slug:    slug,
				Success: false,
				Error:   "缺少 SKILL.md，已跳过",
			})
			continue
		}
		name, desc := frontmatterNameDesc(string(md))
		result := s.installSkill(slug, entries, "upload")
		items = append(items, &InstallResult{
			Name:        name,
			Slug:        slug,
			Description: desc,
			Success:     result.Success,
			Error:       result.Error,
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("zip 中未找到任何含 SKILL.md 的 Skill")
	}
	return items, nil
}

// ---------- 远程拉取（使用 npx skills CLI） ----------

// InstallFromRemote 从 GitHub 拉取并直接安装
func (s *InstallService) InstallFromRemote(source string, slugs []string) ([]*InstallResult, error) {
	// 创建隔离的工作目录
	tempHome, env, workdir := createIsolatedWorkdir()
	defer os.RemoveAll(tempHome)

	var items []*InstallResult
	for _, slug := range slugs {
		result := s.installFromRemoteCLI(source, slug, env, workdir, tempHome)
		items = append(items, result)
	}

	return items, nil
}

// installFromRemoteCLI 使用 npx skills CLI 安装单个 skill
func (s *InstallService) installFromRemoteCLI(source, slug string, env []string, workdir string, tempHome string) *InstallResult {
	// 执行: npx -y skills add <source> --skill <slug> -g -y --copy
	args := []string{
		"-y", "skills", "add", source,
		"--skill", slug,
		"-g", "-y", "--copy",
	}

	_, err := runCLI("npx", args, env, workdir)
	if err != nil {
		return &InstallResult{
			Name:    slug,
			Slug:    slug,
			Success: false,
			Error:   fmt.Sprintf("CLI 执行失败: %v", err),
		}
	}

	// 从临时目录提取下载的 skill
	skillsDir := filepath.Join(tempHome, ".agents", "skills")
	skillDir := findSkillDir(skillsDir, slug)
	if skillDir == "" {
		return &InstallResult{
			Name:    slug,
			Slug:    slug,
			Success: false,
			Error:   "CLI 执行成功但未找到下载的 skill",
		}
	}

	// 读取 skill 文件
	entries, err := readSkillDir(skillDir)
	if err != nil {
		return &InstallResult{
			Name:    slug,
			Slug:    slug,
			Success: false,
			Error:   fmt.Sprintf("读取 skill 文件失败: %v", err),
		}
	}

	// 提取元数据
	md, ok := entries["SKILL.md"]
	if !ok {
		return &InstallResult{
			Name:    slug,
			Slug:    slug,
			Success: false,
			Error:   "缺少 SKILL.md",
		}
	}
	name, desc := frontmatterNameDesc(string(md))

	// 安装到本地
	result := s.installSkill(slug, entries, "remote")
	result.Name = name
	result.Description = desc
	return result
}

// createIsolatedWorkdir 创建隔离的临时 HOME 目录
func createIsolatedWorkdir() (string, []string, string) {
	tempHome, _ := os.MkdirTemp("", ".remote-skills-*")
	workdir := filepath.Join(tempHome, "workspace")
	os.MkdirAll(workdir, 0755)

	// 复制当前环境变量
	env := os.Environ()
	// 设置 HOME 为临时目录（Windows 用 USERPROFILE）
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+tempHome)
	} else {
		env = append(env, "HOME="+tempHome)
	}

	return tempHome, env, workdir
}

// runCLI 执行 CLI 命令
func runCLI(name string, args []string, env []string, workdir string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	// 清洗 ANSI 转义序列
	cleaned := cleanCLIOutput(string(output))

	if err != nil {
		return cleaned, fmt.Errorf("命令执行失败: %w, 输出: %s", err, cleaned)
	}
	return cleaned, nil
}

// cleanCLIOutput 清洗 CLI 输出（去除 ANSI 转义序列）
func cleanCLIOutput(output string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(output, "")
}

// findSkillDir 在 skills 目录下查找指定 skill
func findSkillDir(skillsDir, name string) string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == name {
			return filepath.Join(skillsDir, name)
		}
	}
	return ""
}

// readSkillDir 读取 skill 目录下的所有文件
func readSkillDir(dir string) (map[string][]byte, error) {
	entries := map[string][]byte{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// 使用正斜杠
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[rel] = data
		return nil
	})
	return entries, err
}

// ---------- 核心安装逻辑 ----------

// installSkill 安装单个 skill 到本地目录并写入数据库
func (s *InstallService) installSkill(slug string, entries map[string][]byte, sourceType string) *InstallResult {
	destDir := s.loader.GetSkillDir(slug)

	// 已存在则跳过
	if _, statErr := os.Stat(destDir); statErr == nil {
		return &InstallResult{
			Slug:    slug,
			Success: false,
			Error:   "Skill 已存在",
		}
	}

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return &InstallResult{
			Slug:    slug,
			Success: false,
			Error:   err.Error(),
		}
	}

	// 写入文件
	for rel, content := range entries {
		abs := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			return &InstallResult{
				Slug:    slug,
				Success: false,
				Error:   err.Error(),
			}
		}
		if err := os.WriteFile(abs, content, 0644); err != nil {
			return &InstallResult{
				Slug:    slug,
				Success: false,
				Error:   err.Error(),
			}
		}
	}

	// 从 SKILL.md 提取元数据
	md, _ := entries["SKILL.md"]
	name, desc := frontmatterNameDesc(string(md))

	// 写入数据库
	skillEntity := &entity.Skill{
		Slug:        slug,
		Name:        name,
		Description: desc,
		SourceType:  sourceType,
		DirPath:     slug,
		Enabled:     true,
	}
	if err := s.repo.Create(skillEntity); err != nil {
		return &InstallResult{
			Slug:    slug,
			Success: false,
			Error:   err.Error(),
		}
	}

	return &InstallResult{
		Slug:    slug,
		Success: true,
	}
}
