// Package websearch 实现联网搜索内置工具。
// 工具名 web_search，前端已有 tavily_search 映射兼容（见 ToolCallRenderer.vue）。
// 通过环境变量 WEB_SEARCH_PROVIDER / WEB_SEARCH_BASE_URL / WEB_SEARCH_API_KEY 配置，
// 代码中不内置 BaseURL 默认值。
package websearch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Qavor/internal/tool"
	"Qavor/pkg/config"
)

// WebSearchToolName 工具名。前端 ToolCallRenderer 同时映射 web_search 与 tavily_search。
const WebSearchToolName = "web_search"

// WebSearchTool 联网搜索工具，实现 tool.BuiltinTool 接口。
type WebSearchTool struct {
	provider SearchProvider
}

// NewWebSearchTool 用给定 Provider 创建工具。Provider 由 NewTool 按 config 选定。
func NewWebSearchTool(provider SearchProvider) *WebSearchTool {
	return &WebSearchTool{provider: provider}
}

// Meta 返回工具元数据。
func (t *WebSearchTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        WebSearchToolName,
		Label:       "联网搜索",
		Description: "搜索互联网获取实时信息。当用户询问最新新闻、天气、技术文档或其他需要联网的内容时使用此工具。返回搜索结果列表，包含标题、URL、内容摘要和相关度评分。",
		Category:    tool.CategorySystem,
		Tags:        []string{"search", "web", "internet"},
		Args: []tool.ArgDef{
			{
				Name:        "query",
				Type:        "string",
				Description: "搜索关键词，使用简洁精准的自然语言",
				Required:    true,
			},
			{
				Name:        "max_results",
				Type:        "integer",
				Description: "最大返回结果数量，默认 5，范围 1-10",
				Required:    false,
			},
		},
		ConfigGuide: "工具名: web_search\n参数: query(必填), max_results(可选)\n示例: {\"query\": \"Go 1.22 新特性\", \"max_results\": 5}",
	}
}

// Execute 执行搜索。
// 错误由 eino_adapter 统一包装为 {"error":true,"message":"..."} 返回 LLM，不中断流。
func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	if t == nil || t.provider == nil {
		return nil, errors.New("search provider is not configured")
	}

	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("query 参数不能为空")
	}

	maxResults := 5
	if v, ok := args["max_results"]; ok {
		if n, ok := toInt(v); ok && n > 0 && n <= 10 {
			maxResults = n
		}
	}

	req := &SearchRequest{
		Query:      query,
		MaxResults: maxResults,
	}

	resp, err := t.provider.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	return resp, nil
}

// toInt 容错将 LLM 传入的数值参数转为 int。JSON 反序列化后数值通常是 float64。
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

// NewTool 根据配置与运行模式构造 WebSearchTool。
//
// 决策逻辑：
//   - API Key 非空 + BaseURL 非空 → 按 Provider 创建真实 Provider（tavily/brave）
//   - API Key 非空 + BaseURL 空    → 返回 (nil, err)，配置不完整
//   - API Key 空 + debug 模式      → Mock Provider（前端联调）
//   - API Key 空 + release 模式    → 返回 (nil, nil)，静默不注册
//
// appMode 取自 cfg.App.Mode（debug/release/test），release/test 视为非 debug。
func NewTool(cfg config.WebSearchConfig, appMode string) (tool.BuiltinTool, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "tavily"
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	baseURL := strings.TrimSpace(cfg.BaseURL)

	// 有 API Key：必须配 BaseURL（代码中无默认值），按 provider 创建真实 Provider
	if apiKey != "" {
		if baseURL == "" {
			return nil, fmt.Errorf("web_search.api_key 已配置但 base_url 为空，请通过环境变量 WEB_SEARCH_BASE_URL 或 config.yaml 显式提供（%s 官方地址见文档）", provider)
		}
		var p SearchProvider
		switch provider {
		case "tavily":
			p = NewTavilyProvider(baseURL, apiKey)
		case "brave":
			p = NewBraveProvider(baseURL, apiKey)
		default:
			return nil, fmt.Errorf("不支持的 web_search.provider: %s（仅支持 tavily | brave）", provider)
		}
		return NewWebSearchTool(p), nil
	}

	// 无 API Key：debug 模式用 Mock，其余模式不注册
	if appMode == "debug" {
		return NewWebSearchTool(NewMockProvider()), nil
	}
	return nil, nil
}
