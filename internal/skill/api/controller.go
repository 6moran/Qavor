package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"Qavor/internal/model/entity"
	"Qavor/internal/skill"
	"Qavor/internal/skill/remote"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller Skill API 控制器
type Controller struct {
	svc     skill.SkillService
	install *skill.InstallService
	loader  skill.SkillLoader
}

// NewController 创建控制器
func NewController(svc skill.SkillService, loader skill.SkillLoader, install *skill.InstallService) *Controller {
	return &Controller{svc: svc, loader: loader, install: install}
}

// ListSkills 获取 Skill 列表
func (ctrl *Controller) ListSkills(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	skills, total, err := ctrl.svc.List((page-1)*pageSize, pageSize, keyword)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"list":  skills,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// GetSkill 获取 Skill 详情
func (ctrl *Controller) GetSkill(c *gin.Context) {
	slug := c.Param("slug")
	skillEntity, err := ctrl.svc.GetBySlug(slug)
	if err != nil {
		response.BizError(c, err)
		return
	}
	if skillEntity == nil {
		response.NotFound(c, "Skill 不存在")
		return
	}

	response.Success(c, skillEntity)
}

// CreateSkill 创建 Skill
func (ctrl *Controller) CreateSkill(c *gin.Context) {
	var req entity.Skill
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.svc.Create(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, req)
}

// UpdateSkill 更新 Skill
func (ctrl *Controller) UpdateSkill(c *gin.Context) {
	slug := c.Param("slug")
	var req entity.Skill
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.svc.Update(slug, &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, req)
}

// DeleteSkill 删除 Skill
func (ctrl *Controller) DeleteSkill(c *gin.Context) {
	slug := c.Param("slug")
	if err := ctrl.svc.Delete(slug); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// BatchCreateSkills 批量创建 Skill
func (ctrl *Controller) BatchCreateSkills(c *gin.Context) {
	var req []*entity.Skill
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	for _, skill := range req {
		if err := ctrl.svc.Create(skill); err != nil {
			response.BizError(c, err)
			return
		}
	}

	response.Success(c, req)
}

// GetSkillOptions 获取 Skill 选项列表
func (ctrl *Controller) GetSkillOptions(c *gin.Context) {
	options, err := ctrl.svc.GetOptions()
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"skills": options})
}

// GetSkillTree 获取 Skill 目录结构
func (ctrl *Controller) GetSkillTree(c *gin.Context) {
	slug := c.Param("slug")
	dir := ctrl.loader.GetSkillDir(slug)

	tree, err := buildFileTree(dir, dir)
	if err != nil {
		response.BizError(c, fmt.Errorf("读取目录失败: %w", err))
		return
	}

	response.Success(c, tree)
}

// DeleteSkillsBatch 批量删除 Skills（POST 方式）
func (ctrl *Controller) DeleteSkillsBatch(c *gin.Context) {
	var req struct {
		Slugs []string `json:"slugs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	for _, slug := range req.Slugs {
		if err := ctrl.svc.Delete(slug); err != nil {
			response.BizError(c, err)
			return
		}
	}

	response.Success(c, nil)
}

// PrepareSkillUpload 准备 Skill 上传
func (ctrl *Controller) PrepareSkillUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	f, err := file.Open()
	if err != nil {
		response.BizError(c, fmt.Errorf("打开文件失败: %w", err))
		return
	}
	defer f.Close()

	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		response.BizError(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}

	results, err := ctrl.install.InstallFromZip(data, file.Filename)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, results)
}

// GetSkillDependencyOptions 获取 Skill 依赖选项
func (ctrl *Controller) GetSkillDependencyOptions(c *gin.Context) {
	slug := c.Query("slug")

	options, err := ctrl.svc.GetDependencyOptions(slug)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, options)
}

// ListBuiltinSkills 列出内置 Skills
func (ctrl *Controller) ListBuiltinSkills(c *gin.Context) {
	skills, err := ctrl.svc.ListBuiltinSkills()
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{"skills": skills})
}

// SyncBuiltinSkills 同步内置 Skills
func (ctrl *Controller) SyncBuiltinSkills(c *gin.Context) {
	if err := ctrl.svc.SyncBuiltinSkills(); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// DeleteSkillFile 删除 Skill 文件
func (ctrl *Controller) DeleteSkillFile(c *gin.Context) {
	slug := c.Param("slug")
	relPath := c.Query("path")
	if relPath == "" {
		response.BadRequest(c, "path 参数不能为空")
		return
	}

	dir := ctrl.loader.GetSkillDir(slug)
	absPath := filepath.Join(dir, relPath)

	if !isSubPath(dir, absPath) {
		response.BadRequest(c, "非法路径")
		return
	}

	if err := os.Remove(absPath); err != nil {
		response.BizError(c, fmt.Errorf("删除文件失败: %w", err))
		return
	}

	response.Success(c, nil)
}

// UpdateSkillDependencies 更新 Skill 依赖
func (ctrl *Controller) UpdateSkillDependencies(c *gin.Context) {
	slug := c.Param("slug")

	var req struct {
		ToolDependencies []string `json:"tool_dependencies"`
		MCPDependencies  []string `json:"mcp_dependencies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	skillEntity, err := ctrl.svc.GetBySlug(slug)
	if err != nil {
		response.BizError(c, err)
		return
	}
	if skillEntity == nil {
		response.NotFound(c, "Skill 不存在")
		return
	}

	skillEntity.ToolDependencies = toStringArray(req.ToolDependencies)
	skillEntity.MCPDependencies = toStringArray(req.MCPDependencies)

	if err := ctrl.svc.Update(slug, skillEntity); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// UpdateSkillEnabled 更新 Skill 启用状态
func (ctrl *Controller) UpdateSkillEnabled(c *gin.Context) {
	slug := c.Param("slug")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	skillEntity, err := ctrl.svc.GetBySlug(slug)
	if err != nil {
		response.BizError(c, err)
		return
	}
	if skillEntity == nil {
		response.NotFound(c, "Skill 不存在")
		return
	}

	skillEntity.Enabled = req.Enabled

	if err := ctrl.svc.Update(slug, skillEntity); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, skillEntity)
}

// ListRemoteSkills 列出远程 Skills
func (ctrl *Controller) ListRemoteSkills(c *gin.Context) {
	var req struct {
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	provider, err := remote.Resolve(req.Source)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	skills, err := provider.List(req.Source)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, skills)
}

// PrepareRemoteSkills 准备远程 Skills
func (ctrl *Controller) PrepareRemoteSkills(c *gin.Context) {
	var req struct {
		Source string   `json:"source"`
		Skills []string `json:"skills"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	results, err := ctrl.install.InstallFromRemote(req.Source, req.Skills)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, results)
}

// GetSkillFile 读取 Skill 文件内容
func (ctrl *Controller) GetSkillFile(c *gin.Context) {
	slug := c.Param("slug")
	relPath := c.Query("path")
	if relPath == "" {
		response.BadRequest(c, "path 参数不能为空")
		return
	}

	dir := ctrl.loader.GetSkillDir(slug)
	absPath := filepath.Join(dir, relPath)

	// 防止路径穿越
	if !isSubPath(dir, absPath) {
		response.BadRequest(c, "非法路径")
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		response.BizError(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}

	response.Success(c, gin.H{
		"path":    relPath,
		"content": string(data),
	})
}

// UpdateSkillFile 更新 Skill 文件内容
func (ctrl *Controller) UpdateSkillFile(c *gin.Context) {
	slug := c.Param("slug")

	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Path == "" {
		response.BadRequest(c, "path 不能为空")
		return
	}

	dir := ctrl.loader.GetSkillDir(slug)
	absPath := filepath.Join(dir, req.Path)

	if !isSubPath(dir, absPath) {
		response.BadRequest(c, "非法路径")
		return
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		response.BizError(c, fmt.Errorf("创建目录失败: %w", err))
		return
	}

	if err := os.WriteFile(absPath, []byte(req.Content), 0644); err != nil {
		response.BizError(c, fmt.Errorf("写入文件失败: %w", err))
		return
	}

	response.Success(c, nil)
}

// fileTree 文件树节点
type fileTree struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Children []*fileTree `json:"children,omitempty"`
}

func buildFileTree(root, baseDir string) (*fileTree, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	node := &fileTree{
		Name:  info.Name(),
		Path:  filepath.ToSlash(filepath.Base(root)),
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		return node, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		childPath := filepath.Join(root, entry.Name())
		child, err := buildFileTree(childPath, baseDir)
		if err != nil {
			continue
		}
		node.Children = append(node.Children, child)
	}

	return node, nil
}

func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && len(rel) > 0 && rel[0] != '.'
}

// toStringArray 将 []string 转换为 entity.JSONArray
func toStringArray(items []string) entity.JSONArray {
	result := make(entity.JSONArray, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

// ImportSkill 导入 Skill
func (ctrl *Controller) ImportSkill(c *gin.Context) {
	slug := c.PostForm("slug")
	if slug == "" {
		response.BadRequest(c, "slug 不能为空")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	f, err := file.Open()
	if err != nil {
		response.BizError(c, fmt.Errorf("打开文件失败: %w", err))
		return
	}
	defer f.Close()

	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		response.BizError(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}

	if err := ctrl.svc.Import(slug, data); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ExportSkill 导出 Skill
func (ctrl *Controller) ExportSkill(c *gin.Context) {
	slug := c.Param("slug")

	data, err := ctrl.svc.Export(slug)
	if err != nil {
		response.BizError(c, err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", slug))
	c.Data(200, "application/zip", data)
}
