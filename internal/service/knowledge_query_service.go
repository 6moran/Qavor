package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/config"
	bizerrors "Qavor/pkg/errors"

	"github.com/cloudwego/eino/schema"
)

// 检索测试可调参数白名单，与前端 SearchConfigPanel 返回的选项 key 一一对应。
const (
	metaKeyTopK           = "top_k"
	metaKeyVectorTopK     = "vector_top_k"
	metaKeyKeywordTopK    = "keyword_top_k"
	metaKeyFusedTopK      = "fused_top_k"
	metaKeyRerankTopK     = "rerank_top_k"
	metaKeyRRFK           = "rrf_k"
	metaKeyScoreThreshold = "score_threshold"
)

// 生成示例问题时的文件标题参考数量上限。
const sampleQuestionFileTitleLimit = 20

// sampleQuestionTimeout 生成示例问题的宽松超时：模型推理可能超过配置的短超时，
// 批处理场景用独立下限避免失败。
const sampleQuestionTimeout = 120 * time.Second

// knowledgeQueryService 实现 KnowledgeQueryService。
type knowledgeQueryService struct {
	cfg      config.RAGConfig
	kbRepo   repository.KnowledgeBaseRepository
	fileRepo repository.KnowledgeFileRepository
	ragSvc   RAGService
	modelSvc ModelService
}

// NewKnowledgeQueryService 创建知识库查询服务。
// ragSvc 提供检索能力；modelSvc 用于生成示例问题时解析知识库绑定的 Chat 模型。
func NewKnowledgeQueryService(
	cfg config.RAGConfig,
	kbRepo repository.KnowledgeBaseRepository,
	fileRepo repository.KnowledgeFileRepository,
	ragSvc RAGService,
	modelSvc ModelService,
) KnowledgeQueryService {
	return &knowledgeQueryService{cfg: cfg, kbRepo: kbRepo, fileRepo: fileRepo, ragSvc: ragSvc, modelSvc: modelSvc}
}

// QueryTest 执行检索测试，meta 中的白名单参数覆盖检索各阶段配置。
func (s *knowledgeQueryService) QueryTest(ctx context.Context, kbID, query string, meta map[string]any) (*RAGRetrieveResult, error) {
	return s.ragSvc.RetrieveTest(ctx, []string{kbID}, query, buildRetrievalTestConfig(meta))
}

// GetQueryParams 返回检索参数选项定义，default 取知识库保存值，缺失时用系统默认。
func (s *knowledgeQueryService) GetQueryParams(kbID string) (*QueryParamsPayload, error) {
	base, err := s.findBase(kbID)
	if err != nil {
		return nil, err
	}
	return &QueryParamsPayload{Params: QueryParamsView{Options: s.buildOptions(base.QueryParams)}}, nil
}

// UpdateQueryParams 白名单过滤后更新知识库检索参数。
func (s *knowledgeQueryService) UpdateQueryParams(kbID string, values map[string]any) error {
	base, err := s.findBase(kbID)
	if err != nil {
		return err
	}
	next := entity.JSON{}
	for key, value := range values {
		if isMetaKey(key) {
			next[key] = value
		}
	}
	base.QueryParams = next
	return s.kbRepo.Update(base)
}

// GetSampleQuestions 返回已生成的示例问题；未生成时返回 404 语义的业务错误。
func (s *knowledgeQueryService) GetSampleQuestions(kbID string) (*SampleQuestionsPayload, error) {
	base, err := s.findBase(kbID)
	if err != nil {
		return nil, err
	}
	questions := make([]string, 0, len(base.SampleQuestions))
	for _, item := range base.SampleQuestions {
		if q, ok := item.(string); ok && strings.TrimSpace(q) != "" {
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "还没有生成示例问题，请先生成")
	}
	return &SampleQuestionsPayload{Questions: questions}, nil
}

// GenerateSampleQuestions 基于知识库名称/描述与文件标题生成示例问题并持久化。
func (s *knowledgeQueryService) GenerateSampleQuestions(ctx context.Context, kbID string, count int) (*SampleQuestionsPayload, error) {
	if count <= 0 {
		count = 10
	}
	if count > 50 {
		count = 50
	}
	base, err := s.findBase(kbID)
	if err != nil {
		return nil, err
	}
	if base.ChatModelID == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "知识库未绑定 Chat 模型，无法生成示例问题")
	}
	// 生成示例问题是批处理操作：使用独立宽松超时（context + 客户端），
	// 避免模型配置的短超时（如 30s）在模型推理较慢时导致生成失败。
	genCtx, cancel := context.WithTimeout(ctx, sampleQuestionTimeout)
	defer cancel()
	chatModel, err := s.modelSvc.ResolveChatModelWithTimeout(genCtx, base.ChatModelID, sampleQuestionTimeout)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMConfigError, "解析 Chat 模型失败", err)
	}
	prompt := buildSampleQuestionsPrompt(base.Name, base.Description, s.fileTitles(genCtx, kbID), count)
	out, err := chatModel.Generate(genCtx, []*schema.Message{{Role: schema.User, Content: prompt}})
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMRequestFailed, "LLM 生成示例问题失败", err)
	}
	questions, err := parseJSONStringArray(out.Content)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeLLMResponseInvalid, "LLM 返回的示例问题格式无效", err)
	}
	// 去重并限制数量。
	seen := make(map[string]bool, len(questions))
	result := make([]string, 0, len(questions))
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		result = append(result, q)
		if len(result) >= count {
			break
		}
	}
	if len(result) == 0 {
		return nil, bizerrors.New(bizerrors.CodeLLMResponseInvalid, "LLM 未返回有效示例问题")
	}
	items := make(entity.JSONArray, 0, len(result))
	for _, q := range result {
		items = append(items, q)
	}
	base.SampleQuestions = items
	if err := s.kbRepo.Update(base); err != nil {
		return nil, err
	}
	return &SampleQuestionsPayload{Questions: result}, nil
}

// findBase 查询知识库，不存在时返回统一的业务错误。
func (s *knowledgeQueryService) findBase(kbID string) (*entity.KnowledgeBase, error) {
	base, err := s.kbRepo.FindByKBID(kbID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, knowledgeBaseNotFoundError()
	}
	return base, nil
}

// fileTitles 返回知识库文件标题列表（最多 sampleQuestionFileTitleLimit 个），供生成问题参考。
func (s *knowledgeQueryService) fileTitles(ctx context.Context, kbID string) []string {
	if s.fileRepo == nil {
		return nil
	}
	files, err := s.fileRepo.ListAllByKBID(ctx, kbID)
	if err != nil {
		return nil
	}
	titles := make([]string, 0, len(files))
	for _, file := range files {
		if file == nil || file.IsFolder {
			continue
		}
		name := file.OriginalFilename
		if name == "" {
			name = file.Filename
		}
		if name != "" {
			titles = append(titles, name)
		}
		if len(titles) >= sampleQuestionFileTitleLimit {
			break
		}
	}
	return titles
}

// buildOptions 构建检索参数选项定义；default 优先取知识库保存值（限幅到合法范围），否则系统默认。
func (s *knowledgeQueryService) buildOptions(saved entity.JSON) []QueryParamOption {
	systemDefaults := map[string]any{
		metaKeyTopK:           float64(s.defaultInt(s.cfg.TopK, 5)),
		metaKeyVectorTopK:     20.0,
		metaKeyKeywordTopK:    20.0,
		metaKeyFusedTopK:      20.0,
		metaKeyRerankTopK:     float64(s.defaultInt(s.cfg.RerankTopK, 5)),
		metaKeyRRFK:           60.0,
		metaKeyScoreThreshold: s.defaultFloat(s.cfg.ScoreThreshold, 0.3),
	}
	options := []QueryParamOption{
		{Key: metaKeyTopK, Label: "返回条数", Type: "number", Min: f64(1), Max: f64(20), Step: f64(1), Description: "最终返回的检索结果条数"},
		{Key: metaKeyVectorTopK, Label: "向量召回", Type: "number", Min: f64(1), Max: f64(50), Step: f64(1), Description: "向量检索的候选条数"},
		{Key: metaKeyKeywordTopK, Label: "关键词召回", Type: "number", Min: f64(1), Max: f64(50), Step: f64(1), Description: "关键词检索的候选条数"},
		{Key: metaKeyFusedTopK, Label: "融合候选", Type: "number", Min: f64(1), Max: f64(50), Step: f64(1), Description: "RRF 融合后进入排序的候选条数"},
		{Key: metaKeyRerankTopK, Label: "重排返回", Type: "number", Min: f64(1), Max: f64(20), Step: f64(1), Description: "重排后最终返回的条数"},
		{Key: metaKeyRRFK, Label: "RRF 参数", Type: "number", Min: f64(1), Max: f64(200), Step: f64(1), Description: "RRF 融合的平滑参数"},
		{Key: metaKeyScoreThreshold, Label: "相似度阈值", Type: "number", Min: f64(0), Max: f64(1), Step: f64(0.05), Description: "低于该值的检索片段将被过滤"},
	}
	for i := range options {
		options[i].Default = mergeSavedValue(options[i].Key, saved, systemDefaults)
	}
	return options
}

// mergeSavedValue 优先使用知识库保存的合法值，否则回退系统默认。
func mergeSavedValue(key string, saved entity.JSON, defaults map[string]any) any {
	value, ok := saved[key]
	if !ok {
		return defaults[key]
	}
	number, ok := toFloat64(value)
	if !ok {
		return defaults[key]
	}
	return number
}

// buildRetrievalTestConfig 从 meta 白名单解析单次检索参数覆盖。
// 非法类型或超出合法范围的键静默忽略，全部无效时返回 nil（沿用系统默认）。
func buildRetrievalTestConfig(meta map[string]any) *RetrievalTestConfig {
	if len(meta) == 0 {
		return nil
	}
	cfg := &RetrievalTestConfig{
		TopK:           intFromMeta(meta, metaKeyTopK, 1, 20),
		VectorTopK:     intFromMeta(meta, metaKeyVectorTopK, 1, 50),
		KeywordTopK:    intFromMeta(meta, metaKeyKeywordTopK, 1, 50),
		FusedTopK:      intFromMeta(meta, metaKeyFusedTopK, 1, 50),
		RerankTopK:     intFromMeta(meta, metaKeyRerankTopK, 1, 20),
		RRFK:           intFromMeta(meta, metaKeyRRFK, 1, 200),
		ScoreThreshold: floatFromMeta(meta, metaKeyScoreThreshold, 0, 1),
	}
	if cfg.TopK == nil && cfg.VectorTopK == nil && cfg.KeywordTopK == nil &&
		cfg.FusedTopK == nil && cfg.RerankTopK == nil && cfg.RRFK == nil && cfg.ScoreThreshold == nil {
		return nil
	}
	return cfg
}

// intFromMeta 解析 meta 中的整数参数；非数字或超出 [min, max] 时返回 nil。
func intFromMeta(meta map[string]any, key string, min, max int) *int {
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	number, ok := toFloat64(raw)
	if !ok {
		return nil
	}
	value := int(number)
	if float64(value) != number || value < min || value > max {
		return nil
	}
	return &value
}

// floatFromMeta 解析 meta 中的浮点参数；非数字或超出 [min, max] 时返回 nil。
func floatFromMeta(meta map[string]any, key string, min, max float64) *float64 {
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	value, ok := toFloat64(raw)
	if !ok || value < min || value > max {
		return nil
	}
	return &value
}

// toFloat64 将 JSON 数字/整数/数字字符串统一转为 float64。
func toFloat64(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		number, err := value.Float64()
		return number, err == nil
	case string:
		var number float64
		if err := json.Unmarshal([]byte(value), &number); err != nil {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

// isMetaKey 判断键是否属于检索测试参数白名单。
func isMetaKey(key string) bool {
	switch key {
	case metaKeyTopK, metaKeyVectorTopK, metaKeyKeywordTopK, metaKeyFusedTopK,
		metaKeyRerankTopK, metaKeyRRFK, metaKeyScoreThreshold:
		return true
	default:
		return false
	}
}

// buildSampleQuestionsPrompt 构造示例问题生成提示词，要求 LLM 仅输出 JSON 字符串数组。
func buildSampleQuestionsPrompt(name, description string, titles []string, count int) string {
	var sb strings.Builder
	sb.WriteString("你是知识库测试问题生成助手。请根据知识库信息生成适合检索测试的中文问题。\n")
	sb.WriteString("要求：\n")
	sb.WriteString("1. 每个问题都应能通过知识库内容回答，覆盖不同主题，难度适中；\n")
	sb.WriteString("2. 只输出 JSON 字符串数组，不要输出任何解释或标记。\n")
	fmt.Fprintf(&sb, "3. 生成 %d 个问题。\n\n", count)
	fmt.Fprintf(&sb, "知识库名称：%s\n", name)
	if strings.TrimSpace(description) != "" {
		fmt.Fprintf(&sb, "知识库描述：%s\n", strings.TrimSpace(description))
	}
	if len(titles) > 0 {
		sb.WriteString("知识库包含文档：\n")
		for _, title := range titles {
			fmt.Fprintf(&sb, "- %s\n", title)
		}
	}
	return sb.String()
}

// parseJSONStringArray 解析 LLM 返回的 JSON 字符串数组；兼容输出中带代码块或前后解释文字。
func parseJSONStringArray(content string) ([]string, error) {
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "["); start >= 0 {
		if end := strings.LastIndex(content, "]"); end > start {
			content = content[start : end+1]
		}
	}
	var raw []any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}
	questions := make([]string, 0, len(raw))
	for _, item := range raw {
		if q, ok := item.(string); ok {
			questions = append(questions, q)
		}
	}
	return questions, nil
}

func (s *knowledgeQueryService) defaultInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func (s *knowledgeQueryService) defaultFloat(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func f64(value float64) *float64 { return &value }
