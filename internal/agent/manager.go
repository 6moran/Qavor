package agent

import (
	"context"
	"sync"

	einotool "github.com/cloudwego/eino/components/tool"

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

// AgentManager 管理 Agent 实例创建与缓存
type AgentManager struct {
	mcpManager       *mcp.MCPManager
	vectorizer       *mcp.ToolVectorizer
	toolRegistry     *tool.Registry
	skillsMiddleware *skill.SkillsMiddleware
	skillResolver    skill.SkillResolver
	configFetcher    ConfigFetcher
	agents           sync.Map
}

// NewAgentManager 创建 AgentManager
func NewAgentManager(mcpManager *mcp.MCPManager, vectorizer *mcp.ToolVectorizer, toolRegistry *tool.Registry, skillsMiddleware *skill.SkillsMiddleware, skillResolver skill.SkillResolver, configFetcher ConfigFetcher) *AgentManager {
	return &AgentManager{
		mcpManager:       mcpManager,
		vectorizer:       vectorizer,
		toolRegistry:     toolRegistry,
		skillsMiddleware: skillsMiddleware,
		skillResolver:    skillResolver,
		configFetcher:    configFetcher,
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

	// 3. 技能解析 + 收集依赖工具
	skillTools := make(map[string][]einotool.BaseTool)
	if len(cfg.Skills) > 0 && m.skillResolver != nil {
		skillIndex, toolOwnership, err := m.skillResolver.DFSClosure(cfg.Skills)
		if err != nil {
			logger.Warn("Skill 依赖解析失败", zap.Error(err))
		} else if len(skillIndex) > 0 {
			// 增强 instruction
			if m.skillsMiddleware != nil {
				var skills []*skill.SkillMeta
				for _, meta := range skillIndex {
					skills = append(skills, meta)
				}
				cfg.Instruction, _ = m.skillsMiddleware.BuildPrompt(ctx, cfg.Instruction, skills)
			}
			// 收集每个技能的依赖工具
			for toolName, ownerSlug := range toolOwnership {
				if t, ok := m.toolRegistry.Get(toolName); ok {
					einoTool := tool.NewBuiltinToolAdapter(t)
					skillTools[ownerSlug] = append(skillTools[ownerSlug], einoTool)
				}
			}
		}
	}

	// 4. 按需连接 MCP 服务器
	if len(cfg.MCPServers) > 0 {
		m.mcpManager.EnsureConnected(cfg.MCPServers)
	}

	// 5. 创建 Agent
	a, err := NewAgent(cfg, llm, m.mcpManager, m.toolRegistry, m.vectorizer, skillTools)
	if err != nil {
		logger.Error("创建 Agent 失败", zap.String("slug", slug), zap.Error(err))
		return nil, err
	}

	// 6. 缓存
	m.agents.Store(slug, a)
	logger.Info("Agent 已创建并缓存", zap.String("slug", slug))

	return a, nil
}
