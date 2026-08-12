package agent

import (
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/service"
	"Qavor/internal/skill"
	"Qavor/internal/tool"
)

// defaultOptionsProvider 聚合各资源列表，为 configurable_items 提供动态 options。
type defaultOptionsProvider struct {
	toolRegistry     *tool.Registry
	mcpSvc           service.MCPServerService
	skillSvc         skill.SkillService
	knowledgeBaseSvc service.KnowledgeBaseService
	agentSvc         service.AgentService
}

// NewDefaultOptionsProvider 创建默认 OptionsProvider。
func NewDefaultOptionsProvider(
	toolRegistry *tool.Registry,
	mcpSvc service.MCPServerService,
	skillSvc skill.SkillService,
	knowledgeBaseSvc service.KnowledgeBaseService,
	agentSvc service.AgentService,
) OptionsProvider {
	return &defaultOptionsProvider{
		toolRegistry:     toolRegistry,
		mcpSvc:           mcpSvc,
		skillSvc:         skillSvc,
		knowledgeBaseSvc: knowledgeBaseSvc,
		agentSvc:         agentSvc,
	}
}

// ToolOptions 返回内置工具选项（排除运行时自动强制注册的 ask_user/report_need_input）
func (p *defaultOptionsProvider) ToolOptions() []map[string]interface{} {
	if p.toolRegistry == nil {
		return nil
	}
	metas := p.toolRegistry.ListAll()
	hiddenTools := map[string]bool{
		tool.AskUserToolName:         true,
		tool.ReportNeedInputToolName: true,
	}
	out := make([]map[string]interface{}, 0, len(metas))
	for _, meta := range metas {
		if hiddenTools[meta.Name] {
			continue
		}
		label := meta.Label
		if label == "" {
			label = meta.Name
		}
		out = append(out, map[string]interface{}{
			"name":        label,
			"key":         meta.Name,
			"description": meta.Description,
		})
	}
	return out
}

// MCPServerOptions 返回 MCP 服务器选项
func (p *defaultOptionsProvider) MCPServerOptions() []map[string]interface{} {
	if p.mcpSvc == nil {
		return nil
	}
	resp, err := p.mcpSvc.ListMCPServers(&request.MCPServerListRequest{Page: 1, PageSize: 100})
	if err != nil || resp == nil {
		return nil
	}
	items, ok := resp.List.([]dto.MCPServerResponse)
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, m := range items {
		out = append(out, map[string]interface{}{
			"name":        m.Name,
			"key":         m.Name,
			"description": m.Description,
		})
	}
	return out
}

// SkillOptions 返回技能选项
func (p *defaultOptionsProvider) SkillOptions() []map[string]interface{} {
	if p.skillSvc == nil {
		return nil
	}
	opts, err := p.skillSvc.GetOptions()
	if err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(opts))
	for _, s := range opts {
		if s == nil || !s.Enabled {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":        s.Name,
			"slug":        s.Slug,
			"description": s.Description,
		})
	}
	return out
}

// KnowledgeBaseOptions 返回知识库选项
func (p *defaultOptionsProvider) KnowledgeBaseOptions() []map[string]interface{} {
	if p.knowledgeBaseSvc == nil {
		return nil
	}
	resp, err := p.knowledgeBaseSvc.List(&request.KnowledgeBaseListRequest{Page: 1, PageSize: 100})
	if err != nil || resp == nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(resp.Items))
	for _, kb := range resp.Items {
		out = append(out, map[string]interface{}{
			"name":        kb.Name,
			"key":         kb.KBID,
			"description": kb.Description,
		})
	}
	return out
}

// SubagentOptions 返回子智能体选项（从 agent 列表过滤 is_subagent）
func (p *defaultOptionsProvider) SubagentOptions() []map[string]interface{} {
	if p.agentSvc == nil {
		return nil
	}
	resp, err := p.agentSvc.ListAgents(&request.AgentListRequest{Page: 1, PageSize: 100})
	if err != nil || resp == nil {
		return nil
	}
	items, ok := resp.List.([]dto.AgentResponse)
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0)
	for _, a := range items {
		if !a.IsSubagent {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":        a.Name,
			"key":         a.Slug,
			"id":          a.Slug,
			"slug":        a.Slug,
			"description": a.Description,
		})
	}
	return out
}
