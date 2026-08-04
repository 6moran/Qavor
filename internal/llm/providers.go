package llm

// Provider 供应商配置信息
type Provider struct {
	Name        string   `json:"name"`        // 供应商标识符
	DisplayName string   `json:"displayName"` // 显示名称
	BaseURL     string   `json:"baseURL"`     // 默认 API 基础 URL
	Protocol    string   `json:"protocol"`    // 协议类型 (openai/anthropic/ollama)
	Models      []string `json:"models"`      // 推荐的模型列表
}

// ProviderRegistry 供应商注册表
var ProviderRegistry = []Provider{
	// OpenAI 兼容协议
	{
		Name:        "openai",
		DisplayName: "OpenAI",
		BaseURL:     "https://api.openai.com/v1",
		Protocol:    "openai",
		Models:      []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini"},
	},
	{
		Name:        "deepseek",
		DisplayName: "DeepSeek",
		BaseURL:     "https://api.deepseek.com",
		Protocol:    "openai",
		Models:      []string{"deepseek-chat", "deepseek-reasoner"},
	},
	{
		Name:        "moonshot",
		DisplayName: "Moonshot (月之暗面)",
		BaseURL:     "https://api.moonshot.cn",
		Protocol:    "openai",
		Models:      []string{"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
	},
	{
		Name:        "zhipu",
		DisplayName: "智谱 (ZhipuAI/GLM)",
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		Protocol:    "openai",
		Models:      []string{"glm-4", "glm-4-flash", "glm-4-long", "glm-4-plus"},
	},
	{
		Name:        "alibaba",
		DisplayName: "阿里百炼 (Qwen)",
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Protocol:    "openai",
		Models:      []string{"qwen-max", "qwen-plus", "qwen-turbo", "qwen-long"},
	},
	{
		Name:        "tencent",
		DisplayName: "腾讯混元 (Hunyuan)",
		BaseURL:     "https://api.hunyuan.cloud.tencent.com/v1",
		Protocol:    "openai",
		Models:      []string{"hunyuan-pro", "hunyuan-standard", "hunyuan-lite"},
	},
	{
		Name:        "minimax",
		DisplayName: "MiniMax",
		BaseURL:     "https://api.minimax.chat/v1",
		Protocol:    "openai",
		Models:      []string{"abab6.5s-chat", "abab5.5-chat", "abab4-chat"},
	},
	{
		Name:        "groq",
		DisplayName: "Groq",
		BaseURL:     "https://api.groq.com/openai/v1",
		Protocol:    "openai",
		Models:      []string{"llama-3.3-70b-versatile", "mixtral-8x7b-32768", "gemma2-9b-it"},
	},
	{
		Name:        "siliconflow",
		DisplayName: "SiliconFlow (硅基流动)",
		BaseURL:     "https://api.siliconflow.cn/v1",
		Protocol:    "openai",
		Models:      []string{"Qwen/Qwen2.5-72B-Instruct", "deepseek-ai/DeepSeek-V3", "THUDM/glm-4-9b-chat"},
	},

	// Ollama 本地部署
	{
		Name:        "ollama",
		DisplayName: "Ollama (本地部署)",
		BaseURL:     "http://localhost:11434",
		Protocol:    "ollama",
		Models:      []string{"qwen2.5:72b", "llama3.3:70b", "deepseek-v3", "glm4:9b"},
	},
}

// GetProviderByName 根据名称获取供应商配置
func GetProviderByName(name string) (*Provider, bool) {
	for _, p := range ProviderRegistry {
		if p.Name == name {
			return &p, true
		}
	}
	return nil, false
}

// GetProvidersByProtocol 根据协议类型获取供应商列表
func GetProvidersByProtocol(protocol string) []Provider {
	var result []Provider
	for _, p := range ProviderRegistry {
		if p.Protocol == protocol {
			result = append(result, p)
		}
	}
	return result
}
