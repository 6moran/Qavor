package skill

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"Qavor/internal/model/entity"
	"Qavor/internal/skill/remote"
)

// InstallResult 安装结果
type InstallResult struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

// InstallService Skill 安装服务：zip 上传 + GitHub 远程拉取
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

// skillItemsFromLayout 从解压结果生成草稿条目，并校验 SKILL.md
func skillItemsFromLayout(layout map[string]map[string][]byte, sourceType string) ([]*InstallResult, map[string]map[string][]byte) {
	var items []*InstallResult
	valid := map[string]map[string][]byte{}
	slugs := make([]string, 0, len(layout))
	for slug := range layout {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		entries := layout[slug]
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
		items = append(items, &InstallResult{
			Name:        name,
			Slug:        slug,
			Description: desc,
			Success:     true,
		})
		valid[slug] = entries
	}
	return items, valid
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
	items, valid := skillItemsFromLayout(layout, "upload")
	if len(items) == 0 {
		return nil, fmt.Errorf("zip 中未找到任何含 SKILL.md 的 Skill")
	}

	for slug, entries := range valid {
		result := s.installSkill(slug, entries, "upload")
		// 更新对应 item 的结果
		for _, item := range items {
			if item.Slug == slug {
				item.Success = result.Success
				item.Error = result.Error
				break
			}
		}
	}

	return items, nil
}

// ---------- 远程拉取 ----------

// InstallFromRemote 从 GitHub 拉取并直接安装
func (s *InstallService) InstallFromRemote(source string, slugs []string) ([]*InstallResult, error) {
	provider, err := remote.Resolve(source)
	if err != nil {
		return nil, err
	}

	var items []*InstallResult
	for _, slug := range slugs {
		zipData, err := provider.Fetch(source, slug)
		if err != nil {
			items = append(items, &InstallResult{
				Name:    slug,
				Slug:    slug,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}
		subLayout, err := zipLayout(zipData)
		if err != nil {
			items = append(items, &InstallResult{
				Name:    slug,
				Slug:    slug,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}
		// 合并所有文件到一个 slug 下
		entries := map[string][]byte{}
		for _, e := range subLayout {
			for k, v := range e {
				entries[k] = v
			}
		}
		md, ok := entries["SKILL.md"]
		if !ok {
			items = append(items, &InstallResult{
				Name:    slug,
				Slug:    slug,
				Success: false,
				Error:   "拉取内容中缺少 SKILL.md",
			})
			continue
		}
		name, desc := frontmatterNameDesc(string(md))

		result := s.installSkill(slug, entries, "remote")
		items = append(items, &InstallResult{
			Name:        name,
			Slug:        slug,
			Description: desc,
			Success:     result.Success,
			Error:       result.Error,
		})
	}

	return items, nil
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
