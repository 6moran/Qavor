package agent

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"Qavor/internal/mcp"
	"Qavor/internal/skill"
	"Qavor/internal/tool"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// ConfigFetcher 获取 Agent 配置的接口（避免循环依赖）
type ConfigFetcher interface {
	GetAgentConfig(slug string) (*AgentConfig, error)
	GetDefaultAgentConfig() (*AgentConfig, string, error) // 返回 config 和 slug
}

// MemoryProvider 提供长期记忆文本，用于注入 Agent 系统提示词（用户画像/偏好/决策等）。
// longterm.Manager 已实现该接口（RetrieveForPrompt）。
type MemoryProvider interface {
	RetrieveForPrompt(ctx context.Context, userID uint) (string, error)
}

// AgentManager 管理 Agent 实例创建与缓存
type AgentManager struct {
	mcpManager       *mcp.MCPManager
	vectorizer       *mcp.ToolVectorizer
	toolRegistry     *tool.Registry
	skillsMiddleware *skill.SkillsMiddleware
	configFetcher    ConfigFetcher
	runtime          *AgentRuntime
	subagentResolver SubagentLLMResolver // 解析子智能体 LLM
	reverseIndex     *ReverseIndex       // 子→主 反向依赖
	agents           sync.Map
	memoryProvider   MemoryProvider // 长期记忆提供者（可为 nil）
}

// NewAgentManager 创建 AgentManager
func NewAgentManager(mcpManager *mcp.MCPManager, vectorizer *mcp.ToolVectorizer, toolRegistry *tool.Registry, skillsMiddleware *skill.SkillsMiddleware, configFetcher ConfigFetcher, runtime *AgentRuntime, subagentResolver SubagentLLMResolver, memoryProvider MemoryProvider) *AgentManager {
	return &AgentManager{
		mcpManager:       mcpManager,
		vectorizer:       vectorizer,
		toolRegistry:     toolRegistry,
		skillsMiddleware: skillsMiddleware,
		configFetcher:    configFetcher,
		runtime:          runtime,
		subagentResolver: subagentResolver,
		memoryProvider:   memoryProvider,
		reverseIndex:     NewReverseIndex(),
		agents:           sync.Map{},
	}
}

// GetOrCreate 根据 slug 获取或创建 Agent（带缓存）
// 缓存命中 → 直接返回，不查数据库
// 缓存未命中 → 查数据库 → 创建 → 缓存
func (m *AgentManager) GetOrCreate(ctx context.Context, slug string, llm model.ToolCallingChatModel) (*Agent, error) {
	// 解析 slug（空值用默认）
	if slug == "" {
		_, defaultSlug, err := m.configFetcher.GetDefaultAgentConfig()
		if err != nil {
			return nil, err
		}
		slug = defaultSlug
	}

	// 1. 先查缓存
	if cached, ok := m.agents.Load(slug); ok {
		logger.Debug("Agent 缓存命中", zap.String("slug", slug))
		return cached.(*Agent), nil
	}

	// 2. 缓存未命中，查数据库获取配置
	cfg, err := m.configFetcher.GetAgentConfig(slug)
	if err != nil {
		return nil, err
	}

	// 3. 技能渐进式披露：注入 skill 名称列表到 instruction
	if len(cfg.Skills) > 0 && m.skillsMiddleware != nil {
		var skills []*skill.SkillMeta
		for _, slug := range cfg.Skills {
			meta, err := m.skillsMiddleware.GetLoader().LoadMeta(slug)
			if err != nil {
				continue
			}
			skills = append(skills, meta)
		}
		cfg.Instruction, _ = m.skillsMiddleware.BuildPrompt(ctx, cfg.Instruction, skills)
	}

	// 4. 注入长期记忆到系统提示词：用户画像/偏好/决策/项目事实等跨会话背景。
	// 说明：Qavor 使用 Eino Deep Agent，其系统提示词仅来自 Instruction（默认 GenModelInput
	// 会剥离历史首条 system 消息并以 Instruction 前置），因此长期记忆必须并入 Instruction
	// 才能被 LLM 看到；若仅放在历史消息里会被 Eino 丢弃，导致「记录了用户是 Go 工程师却答不出
	// 主要编程语言」这类问题。记忆为全局池（userID=0），并入全局缓存的 Agent Instruction。
	if m.memoryProvider != nil {
		if memText, memErr := m.memoryProvider.RetrieveForPrompt(ctx, 0); memErr == nil && memText != "" {
			cfg.Instruction += "\n\n以下是关于当前用户的长期记忆，回答涉及用户背景时请优先参考：\n" + memText
		}
	}

	// 4. 按需连接 MCP 服务器
	if len(cfg.MCPServers) > 0 {
		m.mcpManager.EnsureConnected(cfg.MCPServers)
	}

	// 5. 组装子智能体 specs（仅主智能体；子智能体自身无 Subagents 配置）。
	//    单个子智能体失败仅记录 warning，不阻塞主智能体可用性。
	subagents, err := m.buildSubagentSpecs(ctx, cfg, slug)
	if err != nil {
		logger.Warn("解析子智能体失败", zap.String("slug", slug), zap.Error(err))
	}

	// 6. 创建 Agent
	a, err := NewAgent(cfg, llm, m.mcpManager, m.toolRegistry, m.vectorizer, m.skillsMiddleware, m.runtime, subagents)
	if err != nil {
		logger.Error("创建 Agent 失败", zap.String("slug", slug), zap.Error(err))
		return nil, err
	}

	// 7. 缓存
	m.agents.Store(slug, a)
	logger.Info("Agent 已创建并缓存", zap.String("slug", slug))

	return a, nil
}

// buildSubagentSpecs 解析主智能体挂载的子智能体 specs。
// 为每个子 slug 查配置、解析 LLM，并记录反向索引。
// 单个子智能体失败仅记录 warning，不阻塞整体。
func (m *AgentManager) buildSubagentSpecs(ctx context.Context, parentCfg *AgentConfig, parentSlug string) ([]*subagentSpec, error) {
	var specs []*subagentSpec
	for _, subSlug := range parentCfg.Subagents {
		subCfg, err := m.configFetcher.GetAgentConfig(subSlug)
		if err != nil {
			logger.Warn("获取子智能体配置失败，跳过", zap.String("sub_slug", subSlug), zap.Error(err))
			continue
		}
		subLLM, err := m.resolveSubagentLLM(ctx, subCfg)
		if err != nil {
			logger.Warn("解析子智能体 LLM 失败，跳过", zap.String("sub_slug", subSlug), zap.Error(err))
			continue
		}
		specs = append(specs, &subagentSpec{cfg: subCfg, llm: subLLM})
		m.reverseIndex.AddParent(subSlug, parentSlug)
	}
	return specs, nil
}

// resolveSubagentLLM 解析子智能体的 LLM。
func (m *AgentManager) resolveSubagentLLM(ctx context.Context, subCfg *AgentConfig) (model.ToolCallingChatModel, error) {
	if m.subagentResolver == nil {
		return nil, fmt.Errorf("subagent resolver is nil")
	}
	if subCfg.ModelID == "" {
		return nil, fmt.Errorf("subagent %s model_id is empty", subCfg.Slug)
	}
	id, err := strconv.ParseUint(subCfg.ModelID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("subagent %s invalid model_id %q: %w", subCfg.Slug, subCfg.ModelID, err)
	}
	return m.subagentResolver.ResolveChatModel(ctx, uint(id))
}

// InvalidateBySubagent 子智能体配置变更/增删时：失效所有引用它的主智能体。
func (m *AgentManager) InvalidateBySubagent(subSlug string) {
	for _, parent := range m.reverseIndex.ParentsOf(subSlug) {
		m.ClearAgentCache(parent)
	}
}

// GetConfig 获取 Agent 配置（不创建 Agent）
func (m *AgentManager) GetConfig(_ context.Context, slug string) (map[string]interface{}, error) {
	// 解析 slug（空值用默认）
	if slug == "" {
		_, defaultSlug, err := m.configFetcher.GetDefaultAgentConfig()
		if err != nil {
			return nil, err
		}
		slug = defaultSlug
	}

	cfg, err := m.configFetcher.GetAgentConfig(slug)
	if err != nil {
		return nil, err
	}

	// 转换为 map
	result := map[string]interface{}{
		"name":        cfg.Name,
		"description": cfg.Description,
		"instruction": cfg.Instruction,
		"model_id":    cfg.ModelID,
		"tools":       cfg.Tools,
		"mcp_servers": cfg.MCPServers,
		"skills":      cfg.Skills,
	}

	return result, nil
}

// ClearAgentCache 清除指定 Agent 缓存，强制下次请求重新创建
func (m *AgentManager) ClearAgentCache(slug string) {
	m.agents.Delete(slug)
	logger.Info("Agent 缓存已清除", zap.String("slug", slug))
}

// ClearAllAgentCaches 清除所有 Agent 缓存
func (m *AgentManager) ClearAllAgentCaches() {
	m.agents.Range(func(key, value interface{}) bool {
		m.agents.Delete(key)
		return true
	})
	logger.Info("所有 Agent 缓存已清除")
}
