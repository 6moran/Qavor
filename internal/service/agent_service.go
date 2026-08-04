package service

import (
	"encoding/json"
	"fmt"
	"strings"
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
	agentRepo repository.AgentRepository
}

func NewAgentService(agentRepo repository.AgentRepository) AgentService {
	return &agentService{agentRepo: agentRepo}
}

func (s *agentService) CreateAgent(req *request.CreateAgentRequest) (*dto.AgentResponse, error) {
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		// 前端不传 slug 时自动生成，冲突则追加随机后缀
		slug = generateSlug(req.Name)
		for {
			existing, _ := s.agentRepo.GetBySlug(slug)
			if existing == nil {
				break
			}
			slug = fmt.Sprintf("%s-%d", slug, time.Now().UnixNano()%1000000)
		}
	} else {
		existing, _ := s.agentRepo.GetBySlug(slug)
		if existing != nil {
			return nil, bizerrors.New(bizerrors.CodeResourceAlreadyExists, "Agent 已存在")
		}
	}

	cfg := agent.AgentConfig{
		Name:          req.Name,
		Description:   req.Description,
		Instruction:   req.Instruction,
		ProviderID:    req.ProviderID,
		ModelName:     req.ModelName,
		Tools:         req.Tools,
		DisabledTools: req.DisabledTools,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		Metadata:      req.Metadata,
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
		Icon:        req.Icon,
		ConfigJSON:  cfgMap,
		IsDefault:   req.IsDefault,
		IsSubagent:  req.IsSubagent,
	}

	if req.IsDefault {
		if err := s.agentRepo.ClearDefault(); err != nil {
			return nil, err
		}
	}

	if err := s.agentRepo.Create(a); err != nil {
		return nil, err
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

	if req.Name != nil {
		a.Name = *req.Name
		cfg.Name = *req.Name
	}
	if req.Description != nil {
		a.Description = *req.Description
		cfg.Description = *req.Description
	}
	if req.Icon != nil {
		a.Icon = *req.Icon
	}
	if req.Instruction != nil {
		cfg.Instruction = *req.Instruction
	}
	if req.ProviderID != nil {
		cfg.ProviderID = *req.ProviderID
	}
	if req.ModelName != nil {
		cfg.ModelName = *req.ModelName
	}
	if req.Tools != nil {
		cfg.Tools = req.Tools
	}
	if req.DisabledTools != nil {
		cfg.DisabledTools = req.DisabledTools
	}
	if req.MaxTokens != nil {
		cfg.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		cfg.Temperature = *req.Temperature
	}
	if req.Metadata != nil {
		cfg.Metadata = req.Metadata
	}

	// 前端保存运行时配置：body { config_json: { context: {...} } }，拆出 context 合并进 AgentConfig
	if ctx, ok := req.ConfigJSON["context"]; ok {
		if ctxMap, ok := ctx.(map[string]interface{}); ok {
			mergeContext(&cfg, ctxMap)
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
		Icon:              a.Icon,
		IsDefault:         a.IsDefault,
		IsSubagent:        a.IsSubagent,
		Config:            cfg,
		ConfigJSON:        configJSON,
		ConfigurableItems: buildConfigurableItems(a.BackendID),
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
	}
}

// generateSlug 由名称生成 slug：转小写、非字母数字字符压缩为连字符。
// 若结果为空（如全中文名），回退为 agent-<纳秒时间戳>。
func generateSlug(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = fmt.Sprintf("agent-%d", time.Now().UnixNano())
	}
	return slug
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
	getFloat := func(key string) (float64, bool) {
		v, ok := ctx[key]
		if !ok {
			return 0, false
		}
		f, ok := v.(float64)
		return f, ok
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

	if v, ok := getString("instruction"); ok {
		cfg.Instruction = v
	}
	if v, ok := getString("model_name"); ok {
		cfg.ModelName = v
	}
	if v, ok := getString("provider_id"); ok {
		cfg.ProviderID = v
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
	if v, ok := getStrings("disabled_tools"); ok {
		cfg.DisabledTools = v
	}
	if v, ok := getFloat("temperature"); ok {
		cfg.Temperature = v
	}
	if v, ok := getInt("max_tokens"); ok {
		cfg.MaxTokens = v
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
	if v, ok := ctx["metadata"]; ok {
		if metaMap, ok := v.(map[string]interface{}); ok {
			meta := make(map[string]string, len(metaMap))
			for k, val := range metaMap {
				if s, ok := val.(string); ok {
					meta[k] = s
				}
			}
			if len(meta) > 0 {
				cfg.Metadata = meta
			}
		}
	}
}

// buildConfigurableItems 按 backend_id 返回前端可配置项 schema（不含动态 options）。
// 动态 options（工具/MCP/技能/知识库/子智能体）由 controller 层注入。
func buildConfigurableItems(backendID string) map[string]dto.ConfigurableItem {
	items := map[string]dto.ConfigurableItem{
		"model_name":               {Name: "模型", Kind: "llm", Description: "选择智能体使用的模型"},
		"instruction":              {Name: "系统提示词", Kind: "prompt", Description: "智能体的系统提示词"},
		"temperature":              {Name: "温度", Kind: "number", Type: "number", Description: "控制生成随机性，0.0-1.0", Default: 0.7},
		"max_tokens":               {Name: "最大 Token", Kind: "number", Type: "int", Description: "单次生成的最大 token 数", Default: 4096},
		"tool_retrieval_enabled":   {Name: "工具检索", Kind: "bool", Type: "boolean", Description: "启用工具自动检索"},
		"tool_retrieval_threshold": {Name: "工具检索阈值", Kind: "number", Type: "number", Description: "工具检索相似度阈值", Default: 0.6},
		"tool_retrieval_top_k":     {Name: "工具检索 TopK", Kind: "number", Type: "int", Description: "检索返回的工具数量", Default: 10},
		"tools":                    {Name: "工具", Kind: "tools", Type: "list", Description: "启用的内置工具"},
		"mcp_servers":              {Name: "MCP 服务器", Kind: "mcps", Type: "list", Description: "连接的 MCP 服务器"},
		"skills":                   {Name: "技能", Kind: "skills", Type: "list", Description: "启用的技能"},
		"knowledges":               {Name: "知识库", Kind: "knowledges", Type: "list", Description: "关联的知识库"},
	}

	// 子智能体不能嵌套子智能体
	if backendID != "SubAgentBackend" {
		items["subagents"] = dto.ConfigurableItem{Name: "子智能体", Kind: "subagents", Type: "list", Description: "可调用的子智能体"}
	}

	return items
}
