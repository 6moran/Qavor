package rag

// ChunkPreset 分块预设元数据，供知识库创建/入库选择与 API 列表返回。
// 前端按 {value, label, description} 格式渲染下拉选项。
type ChunkPreset struct {
	ID          string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

const (
	// PresetGeneral 通用文档：结构感知分块。
	PresetGeneral = "general"
	// PresetFAQ 问答对切分。
	PresetFAQ = "faq"
)

// DefaultChunkPresetID 未指定预设时的回退值。
const DefaultChunkPresetID = PresetGeneral

// chunkPresets 已注册的分块预设，顺序即 API 返回顺序。
var chunkPresets = []ChunkPreset{
	{
		ID:          PresetGeneral,
		Label:       "通用文档",
		Description: "结构感知分块：代码块、表格整体保留，以标题为边界，适合普通文档、技术文档",
	},
	{
		ID:          PresetFAQ,
		Label:       "FAQ 问答对",
		Description: "按问答对切分：识别「Q:/A:」「问:/答:」格式，每个问答对独立成块，适合常见问题手册",
	},
}

// ChunkPresetList 返回分块预设列表副本，调用方可安全修改。
func ChunkPresetList() []ChunkPreset {
	out := make([]ChunkPreset, len(chunkPresets))
	copy(out, chunkPresets)
	return out
}

// IsValidChunkPreset 判断预设 ID 是否已注册。
func IsValidChunkPreset(id string) bool {
	for _, p := range chunkPresets {
		if p.ID == id {
			return true
		}
	}
	return false
}

// NormalizeChunkPreset 空值或未知预设回退为默认预设。
func NormalizeChunkPreset(id string) string {
	if IsValidChunkPreset(id) {
		return id
	}
	return DefaultChunkPresetID
}

// NewSplitter 按预设 ID 创建分块器，未知预设回退通用分块。
func NewSplitter(presetID string, maxTokens, overlapTokens int) markdownSplitter {
	switch NormalizeChunkPreset(presetID) {
	case PresetFAQ:
		return NewFAQChunker(maxTokens, overlapTokens)
	default:
		return NewChunker(maxTokens, overlapTokens)
	}
}
