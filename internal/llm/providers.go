package llm

// Provider 供应商配置信息
type Provider struct {
	Name             string   `json:"name"`               // 供应商标识符
	DisplayName      string   `json:"displayName"`        // 显示名称
	BaseURL          string   `json:"baseURL"`            // 默认 API 基础 URL
	Protocol         string   `json:"protocol"`           // 协议类型 (openai/ollama)
	Models           []string `json:"models"`             // 推荐的模型列表
	MaxContextTokens int      `json:"max_context_tokens"` // 默认最大上下文 token 数（模型级别可覆盖）
}

// ProviderRegistry 供应商注册表
var ProviderRegistry = []Provider{
	// OpenAI 兼容协议
	{
		Name:             "openai",
		DisplayName:      "OpenAI",
		BaseURL:          "https://api.openai.com/v1",
		Protocol:         "openai",
		Models:           []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini"},
		MaxContextTokens: 128000, // gpt-4o 默认 128k
	},
	{
		Name:             "deepseek",
		DisplayName:      "DeepSeek",
		BaseURL:          "https://api.deepseek.com",
		Protocol:         "openai",
		Models:           []string{"deepseek-chat", "deepseek-reasoner", "deepseek-v4-flash"},
		MaxContextTokens: 64000, // DeepSeek-V3 64k
	},
	{
		Name:             "moonshot",
		DisplayName:      "Moonshot (月之暗面)",
		BaseURL:          "https://api.moonshot.cn",
		Protocol:         "openai",
		Models:           []string{"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
		MaxContextTokens: 131072, // moonshot-v1-128k 默认 128k
	},
	{
		Name:             "zhipu",
		DisplayName:      "智谱 (ZhipuAI/GLM)",
		BaseURL:          "https://open.bigmodel.cn/api/paas/v4",
		Protocol:         "openai",
		Models:           []string{"glm-4", "glm-4-flash", "glm-4-long", "glm-4-plus"},
		MaxContextTokens: 128000, // GLM-4 128k
	},
	{
		Name:             "alibaba",
		DisplayName:      "阿里百炼 (Qwen)",
		BaseURL:          "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Protocol:         "openai",
		Models:           []string{"qwen-max", "qwen-plus", "qwen-turbo", "qwen-long"},
		MaxContextTokens: 131072, // qwen-plus/qwen-turbo 默认 128k
	},
	{
		Name:             "tencent",
		DisplayName:      "腾讯混元 (Hunyuan)",
		BaseURL:          "https://api.hunyuan.cloud.tencent.com/v1",
		Protocol:         "openai",
		Models:           []string{"hunyuan-pro", "hunyuan-standard", "hunyuan-lite"},
		MaxContextTokens: 32768, // hunyuan-pro 默认 32k
	},
	{
		Name:             "minimax",
		DisplayName:      "MiniMax",
		BaseURL:          "https://api.minimax.chat/v1",
		Protocol:         "openai",
		Models:           []string{"abab6.5s-chat", "abab5.5-chat", "abab4-chat"},
		MaxContextTokens: 1000000, // abab6.5s-chat 1M
	},
	{
		Name:             "groq",
		DisplayName:      "Groq",
		BaseURL:          "https://api.groq.com/openai/v1",
		Protocol:         "openai",
		Models:           []string{"llama-3.3-70b-versatile", "mixtral-8x7b-32768", "gemma2-9b-it"},
		MaxContextTokens: 131072, // llama-3.3-70b-versatile 128k
	},
	{
		Name:             "siliconflow",
		DisplayName:      "SiliconFlow (硅基流动)",
		BaseURL:          "https://api.siliconflow.cn/v1",
		Protocol:         "openai",
		Models:           []string{"Qwen/Qwen2.5-72B-Instruct", "deepseek-ai/DeepSeek-V3", "THUDM/glm-4-9b-chat"},
		MaxContextTokens: 131072, // 默认 128k，具体模型可覆盖
	},

	// Ollama 本地部署
	{
		Name:             "ollama",
		DisplayName:      "Ollama (本地部署)",
		BaseURL:          "http://localhost:11434",
		Protocol:         "ollama",
		Models:           []string{"qwen2.5:72b", "llama3.3:70b", "deepseek-v3", "glm4:9b"},
		MaxContextTokens: 8192, // 本地模型默认 8k
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

// modelContextTokens 模型级别的上下文 token 配置
// 当模型的上下文窗口与 Provider 默认值不同时，在此配置
var modelContextTokens = map[string]int{
	// OpenAI 特殊模型
	"openai:o1":          200000, // o1 支持 200k
	"openai:o1-mini":     128000, // o1-mini 支持 128k
	"openai:gpt-4-turbo": 128000, // gpt-4-turbo 128k
	"openai:gpt-4o":      128000, // gpt-4o 128k
	"openai:gpt-4o-mini": 128000, // gpt-4o-mini 128k

	// Moonshot 不同模型
	"moonshot:moonshot-v1-8k":   8192,
	"moonshot:moonshot-v1-32k":  32768,
	"moonshot:moonshot-v1-128k": 131072,

	// Alibaba (Qwen) 不同模型 - 用户特别关注的"阿胡"
	"alibaba:qwen-max":   32768,    // qwen-max 32k
	"alibaba:qwen-plus":  131072,   // qwen-plus 128k
	"alibaba:qwen-turbo": 131072,   // qwen-turbo 128k
	"alibaba:qwen-long":  10000000, // qwen-long 10M (千万级)

	// DeepSeek
	"deepseek:deepseek-chat":     64000, // deepseek-chat 64k
	"deepseek:deepseek-reasoner": 64000, // deepseek-reasoner 64k
	"deepseek:deepseek-v4-flash": 64000, // deepseek-v4-flash 64k

	// Tencent (Hunyuan)
	"tencent:hunyuan-pro":      32768,
	"tencent:hunyuan-standard": 32768,
	"tencent:hunyuan-lite":     8192,

	// SiliconFlow 特殊模型
	"siliconflow:Qwen/Qwen2.5-72B-Instruct": 131072,
	"siliconflow:deepseek-ai/DeepSeek-V3":   64000,
	"siliconflow:THUDM/glm-4-9b-chat":       128000,
}

// GetMaxContextTokens 根据 provider 和 model name 获取最大上下文 token 数
// 优先使用模型级别配置，其次使用 Provider 默认值，最后返回默认值 8192
func GetMaxContextTokens(provider, modelName string) int {
	// 1. 精确匹配 provider:modelName
	if tokens, ok := modelContextTokens[provider+":"+modelName]; ok {
		return tokens
	}

	// 2. 使用 Provider 默认值
	if p, ok := GetProviderByName(provider); ok && p.MaxContextTokens > 0 {
		return p.MaxContextTokens
	}

	// 3. 返回默认值
	return 8192
}
