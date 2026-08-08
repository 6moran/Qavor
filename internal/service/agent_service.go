package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"Qavor/internal/agent"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/response"
)

type agentService struct {
	agentRepo     repository.AgentRepository
	workspaceRoot string // agent 工作区根目录，主智能体创建时自动建 <slug> 子目录
}

func NewAgentService(agentRepo repository.AgentRepository, workspaceRoot string) AgentService {
	return &agentService{agentRepo: agentRepo, workspaceRoot: workspaceRoot}
}

func (s *agentService) CreateAgent(req *request.CreateAgentRequest) (*dto.AgentResponse, error) {
	// 生成唯一slug：agent-<纳秒时间戳>，冲突则重新生成
	slug := generateSlug()
	for {
		existing, _ := s.agentRepo.GetBySlug(slug)
		if existing == nil {
			break
		}
		slug = generateSlug()
	}

	// 根据 BackendID 判断是否为子智能体
	isSubagent := req.BackendID == "SubAgentBackend"

	// 仅主智能体自动创建工作根目录（文件操作限定在此目录内）。
	// 子智能体无文件操作能力，不建目录。
	if !isSubagent && s.workspaceRoot != "" {
		workDir := filepath.Join(s.workspaceRoot, slug)
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return nil, fmt.Errorf("创建 agent 工作目录失败: %w", err)
		}
	}

	// 构建配置，所有默认值在此处设置并存入数据库
	defaultTemperature := 0.7
	cfg := agent.AgentConfig{
		Name:        req.Name,
		Description: req.Description,
		Instruction: req.Instruction,
		ModelID:     req.ModelID,
		// 模型参数默认值
		Temperature: &defaultTemperature,
		MaxTokens:   intPtr(4096),
		// 工具相关默认值 - 默认启用内置工具
		Tools:                  []string{"current_time", "calculator"},
		MCPServers:             []string{},
		ToolRetrievalEnabled:   false,
		ToolRetrievalThreshold: 60,
		ToolRetrievalTopK:      10,
		// 扩展相关默认值
		Skills:     []string{},
		Knowledges: []string{},
		// 智能体配置默认值
		MaxIteration: 20,
	}

	// 主智能体专属配置
	if !isSubagent {
		enabled := true
		cfg.EnableGeneralSubAgent = &enabled
		cfg.Subagents = []string{}
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	var cfgMap entity.JSON
	if err := json.Unmarshal(cfgJSON, &cfgMap); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	a := &entity.Agent{
		Slug:        slug,
		BackendID:   req.BackendID,
		Name:        req.Name,
		Description: req.Description,
		ConfigJSON:  cfgMap,
	}

	if err := s.agentRepo.Create(a); err != nil {
		return nil, err
	}

	// 第一个主智能体自动设为默认
	if !isSubagent {
		defaultAgent, _ := s.agentRepo.GetDefault()
		if defaultAgent == nil {
			_ = s.agentRepo.SetDefault(slug)
			a.IsDefault = true
		}
	}

	return s.toResponse(a), nil
}

func (s *agentService) GetAgent(slug string) (*dto.AgentResponse, error) {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}
	return s.toResponse(a), nil
}

func (s *agentService) UpdateAgent(slug string, req *request.UpdateAgentRequest) (*dto.AgentResponse, error) {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}

	cfg := s.parseConfig(a.ConfigJSON)

	// 更新基本信息
	if req.Name != nil {
		a.Name = *req.Name
		cfg.Name = *req.Name
	}
	if req.Description != nil {
		a.Description = *req.Description
		cfg.Description = *req.Description
	}
	if req.Instruction != nil {
		cfg.Instruction = *req.Instruction
	}
	if req.ModelID != nil {
		cfg.ModelID = *req.ModelID
	}

	// 更新配置 JSON（用于运行时配置更新）
	if req.ConfigJSON != nil {
		if ctx, ok := req.ConfigJSON["context"]; ok {
			if ctxMap, ok := ctx.(map[string]interface{}); ok {
				mergeContext(&cfg, ctxMap)
			}
		}
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	var cfgMap entity.JSON
	if err := json.Unmarshal(cfgJSON, &cfgMap); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	a.ConfigJSON = cfgMap

	if err := s.agentRepo.Update(a); err != nil {
		return nil, err
	}

	return s.toResponse(a), nil
}

func (s *agentService) DeleteAgent(slug string) error {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return err
	}
	if a == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}
	return s.agentRepo.Delete(slug)
}

func (s *agentService) ListAgents(req *request.AgentListRequest) (*response.PageResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	offset := (page - 1) * size
	agents, total, err := s.agentRepo.List(offset, size, req.Keyword)
	if err != nil {
		return nil, err
	}

	items := make([]dto.AgentResponse, len(agents))
	for i, a := range agents {
		items[i] = *s.toResponse(a)
	}

	return response.NewPageResponse(items, total, page, size), nil
}

func (s *agentService) SetDefault(slug string) error {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return err
	}
	if a == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}

	if err := s.agentRepo.ClearDefault(); err != nil {
		return err
	}
	return s.agentRepo.SetDefault(slug)
}

func (s *agentService) GetDefaultAgent() (*dto.AgentResponse, error) {
	a, err := s.agentRepo.GetDefault()
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "未设置默认 Agent")
	}
	return s.toResponse(a), nil
}

func (s *agentService) GetDefaultAgentConfig() (*agent.AgentConfig, string, error) {
	a, err := s.agentRepo.GetDefault()
	if err != nil {
		return nil, "", err
	}
	if a == nil {
		return nil, "", bizerrors.New(bizerrors.CodeResourceNotFound, "未设置默认 Agent")
	}
	cfg := s.parseConfig(a.ConfigJSON)
	cfg.IsSubagent = a.BackendID == "SubAgentBackend"
	cfg.Slug = a.Slug
	return &cfg, a.Slug, nil
}

func (s *agentService) GetAgentConfig(slug string) (*agent.AgentConfig, error) {
	a, err := s.agentRepo.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "Agent 不存在")
	}
	cfg := s.parseConfig(a.ConfigJSON)
	cfg.IsSubagent = a.BackendID == "SubAgentBackend"
	cfg.Slug = a.Slug
	return &cfg, nil
}

func (s *agentService) parseConfig(raw entity.JSON) agent.AgentConfig {
	var cfg agent.AgentConfig
	if raw == nil {
		return cfg
	}
	// 兼容两种存储格式：前端保存的 { context: {...} } 或历史平铺数据
	if ctx, ok := raw["context"]; ok {
		if ctxMap, ok := ctx.(map[string]interface{}); ok {
			data, err := json.Marshal(ctxMap)
			if err == nil {
				_ = json.Unmarshal(data, &cfg)
				return cfg
			}
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func (s *agentService) toResponse(a *entity.Agent) *dto.AgentResponse {
	cfg := s.parseConfig(a.ConfigJSON)

	// 子智能体不暴露通用子智能体开关，避免前端误显示
	isSubagent := a.BackendID == "SubAgentBackend"
	if isSubagent {
		cfg.EnableGeneralSubAgent = nil
	}

	// 前端 extractContext 读 config_json.context，故响应时包一层 context
	cfgJSON, _ := json.Marshal(cfg)
	var cfgMap entity.JSON
	_ = json.Unmarshal(cfgJSON, &cfgMap)
	configJSON := entity.JSON{"context": cfgMap}

	return &dto.AgentResponse{
		Slug:              a.Slug,
		BackendID:         a.BackendID,
		Name:              a.Name,
		Description:       a.Description,
		IsDefault:         a.IsDefault,
		IsSubagent:        isSubagent,
		Config:            cfg,
		ConfigJSON:        configJSON,
		ConfigurableItems: buildConfigurableItems(a.BackendID),
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
	}
}

// generateSlug 生成唯一slug：agent-<纳秒时间戳>。
func generateSlug() string {
	return fmt.Sprintf("agent-%d", time.Now().UnixNano())
}

// mergeContext 将前端保存的 context map 合并进 AgentConfig。
// 只取 AgentConfig 认识的字段（按 json tag），避免多余 key 混入。
func mergeContext(cfg *agent.AgentConfig, ctx map[string]interface{}) {
	getString := func(key string) (string, bool) {
		v, ok := ctx[key]
		if !ok {
			return "", false
		}
		s, ok := v.(string)
		return s, ok
	}
	getStrings := func(key string) ([]string, bool) {
		v, ok := ctx[key]
		if !ok {
			return nil, false
		}
		arr, ok := v.([]interface{})
		if !ok {
			return nil, false
		}
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	}
	getBool := func(key string) (bool, bool) {
		v, ok := ctx[key]
		if !ok {
			return false, false
		}
		b, ok := v.(bool)
		return b, ok
	}
	getInt := func(key string) (int, bool) {
		v, ok := ctx[key]
		if !ok {
			return 0, false
		}
		switch n := v.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return int(i), true
			}
		}
		return 0, false
	}
	getFloat64 := func(key string) (float64, bool) {
		v, ok := ctx[key]
		if !ok {
			return 0, false
		}
		switch n := v.(type) {
		case float64:
			return n, true
		case int:
			return float64(n), true
		case json.Number:
			if f, err := n.Float64(); err == nil {
				return f, true
			}
		}
		return 0, false
	}

	if v, ok := getString("instruction"); ok {
		cfg.Instruction = v
	}
	if v, ok := getString("model_id"); ok {
		cfg.ModelID = v
	}
	if v, ok := getStrings("tools"); ok {
		cfg.Tools = v
	}
	if v, ok := getStrings("mcp_servers"); ok {
		cfg.MCPServers = v
	}
	if v, ok := getStrings("skills"); ok {
		cfg.Skills = v
	}
	if v, ok := getStrings("knowledges"); ok {
		cfg.Knowledges = v
	}
	if v, ok := getStrings("subagents"); ok {
		cfg.Subagents = v
	}
	if v, ok := getBool("tool_retrieval_enabled"); ok {
		cfg.ToolRetrievalEnabled = v
	}
	if v, ok := getInt("tool_retrieval_threshold"); ok {
		cfg.ToolRetrievalThreshold = v
	}
	if v, ok := getInt("tool_retrieval_top_k"); ok {
		cfg.ToolRetrievalTopK = v
	}
	if v, ok := getInt("max_iteration"); ok {
		cfg.MaxIteration = v
	}
	if v, ok := getBool("enable_general_subagent"); ok {
		cfg.EnableGeneralSubAgent = &v
	}
	if v, ok := getFloat64("temperature"); ok {
		cfg.Temperature = &v
	}
	if v, ok := getInt("max_tokens"); ok {
		cfg.MaxTokens = &v
	}
}

// buildConfigurableItems 按 backend_id 返回前端可配置项 schema（不含动态 options）。
// 动态 options（工具/MCP/技能/知识库/子智能体）由 controller 层注入。
// 默认值已在创建时存入数据库，此处不设置 Default。
func buildConfigurableItems(backendID string) map[string]dto.ConfigurableItem {
	items := map[string]dto.ConfigurableItem{
		"instruction":              {Name: "系统提示词", Kind: "prompt", Description: "定义智能体的角色和行为规则"},
		"model_id":                 {Name: "模型", Kind: "llm", Description: "选择智能体使用的模型"},
		"temperature":              {Name: "温度", Kind: "number", Type: "float", Description: "控制生成随机性（0-2）"},
		"max_tokens":               {Name: "最大Token数", Kind: "number", Type: "int", Description: "最大输出 token 数量"},
		"tool_retrieval_enabled":   {Name: "工具检索", Kind: "bool", Type: "boolean", Description: "启用工具自动检索"},
		"tool_retrieval_threshold": {Name: "工具检索阈值", Kind: "number", Type: "number", Description: "工具数量超过此值时启用检索"},
		"tool_retrieval_top_k":     {Name: "工具检索 TopK", Kind: "number", Type: "int", Description: "检索返回的工具数量"},
		"tools":                    {Name: "工具", Kind: "tools", Type: "list", Description: "关联的工具"},
		"mcp_servers":              {Name: "MCP 服务器", Kind: "mcps", Type: "list", Description: "关联的 MCP 服务器"},
		"skills":                   {Name: "技能", Kind: "skills", Type: "list", Description: "关联的技能"},
		"knowledges":               {Name: "知识库", Kind: "knowledges", Type: "list", Description: "关联的知识库"},
	}

	// 主智能体专属配置项（子智能体为轻量 ChatModelAgent，不参与）
	if backendID != "SubAgentBackend" {
		items["max_iteration"] = dto.ConfigurableItem{Name: "最大迭代次数", Kind: "number", Type: "int", Description: "任务分解的最大推理轮次"}
		items["enable_general_subagent"] = dto.ConfigurableItem{Name: "通用子智能体", Kind: "bool", Type: "boolean", Description: "允许主智能体自动创建通用子智能体执行任务"}
	}

	// 子智能体不能嵌套子智能体
	if backendID != "SubAgentBackend" {
		items["subagents"] = dto.ConfigurableItem{Name: "子智能体", Kind: "subagents", Type: "list", Description: "可调用的子智能体"}
	}

	return items
}

// intPtr 返回 int 指针
func intPtr(v int) *int {
	return &v
}
