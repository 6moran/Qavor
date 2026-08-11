package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// chunkRef 提供给 LLM 的知识片段引用。
type chunkRef struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// llmTimeout 评估场景 LLM 调用超时下限（模型自身超时配置取较大值）。
const llmTimeout = 120 * time.Second

// evalLLM 封装评估模块的 LLM 调用：生成 QA 对、生成答案、评判打分。
type evalLLM struct {
	modelSvc ModelService
}

// generateQAPair 根据一组知识片段生成一个评测问答对。
// 返回的 query 与 goldAnswer 供入库；gold_chunk_ids 由调用方按片段 ID 填充。
func (l *evalLLM) generateQAPair(ctx context.Context, modelID uint, chunks []chunkRef) (query, goldAnswer string, err error) {
	chat, err := l.modelSvc.ResolveChatModelWithTimeout(ctx, modelID, llmTimeout)
	if err != nil {
		return "", "", fmt.Errorf("解析生成模型失败: %w", err)
	}

	var b strings.Builder
	b.WriteString("请根据以下知识片段生成一个评测问答对，要求：\n")
	b.WriteString("1. 问题应自然、有实际意义，且必须能仅凭给定片段回答；\n")
	b.WriteString("2. 答案应准确、完整地覆盖片段中的关键信息；\n")
	b.WriteString("3. 只输出 JSON，格式为 {\"query\": \"问题\", \"gold_answer\": \"标准答案\"}。\n\n")
	b.WriteString("知识片段：\n")
	for i, chunk := range chunks {
		b.WriteString(fmt.Sprintf("[片段 %d]\n%s\n", i+1, chunk.Content))
	}

	messages := []*schema.Message{
		schema.SystemMessage("你是一个严谨的评测数据集生成器。"),
		schema.UserMessage(b.String()),
	}
	resp, err := chat.Generate(ctx, messages)
	if err != nil {
		return "", "", fmt.Errorf("生成问答对失败: %w", err)
	}
	if resp == nil {
		return "", "", fmt.Errorf("生成问答对失败: 模型返回空响应")
	}

	payload, err := parseQAJSON(resp.Content)
	if err != nil {
		return "", "", fmt.Errorf("解析问答对失败: %w", err)
	}
	query = strings.TrimSpace(payload["query"])
	goldAnswer = strings.TrimSpace(payload["gold_answer"])
	if query == "" || goldAnswer == "" {
		return "", "", fmt.Errorf("问答对字段不完整")
	}
	return query, goldAnswer, nil
}

// generateAnswer 根据检索到的知识片段生成答案。
func (l *evalLLM) generateAnswer(ctx context.Context, modelID uint, query string, chunks []chunkRef) (string, error) {
	chat, err := l.modelSvc.ResolveChatModelWithTimeout(ctx, modelID, llmTimeout)
	if err != nil {
		return "", fmt.Errorf("解析答案生成模型失败: %w", err)
	}

	var b strings.Builder
	b.WriteString("请仅根据以下资料回答问题，若资料不足以回答请如实说明。\n\n")
	for i, chunk := range chunks {
		b.WriteString(fmt.Sprintf("[资料 %d]\n%s\n", i+1, chunk.Content))
	}
	b.WriteString(fmt.Sprintf("\n问题：%s", query))

	messages := []*schema.Message{
		schema.SystemMessage("你是严谨的知识库问答助手，只依据给定资料作答。"),
		schema.UserMessage(b.String()),
	}
	resp, err := chat.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("生成答案失败: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("生成答案失败: 模型返回空响应")
	}
	return strings.TrimSpace(resp.Content), nil
}

// judgeAnswer 评判生成答案相对标准答案的质量，返回 0-1 分数。
func (l *evalLLM) judgeAnswer(ctx context.Context, modelID uint, query, goldAnswer, generatedAnswer string) (float64, error) {
	chat, err := l.modelSvc.ResolveChatModelWithTimeout(ctx, modelID, llmTimeout)
	if err != nil {
		return 0, fmt.Errorf("解析评判模型失败: %w", err)
	}

	prompt := fmt.Sprintf(
		"问题：%s\n\n标准答案：%s\n\n生成答案：%s\n\n请从准确性与完整性两个维度评判生成答案相对标准答案的质量，只输出 JSON：{\"score\": 0.0}（score 为 0 到 1 之间的小数，1 表示完全正确）。",
		query, goldAnswer, generatedAnswer,
	)
	messages := []*schema.Message{
		schema.SystemMessage("你是严格的答案质量评估员。"),
		schema.UserMessage(prompt),
	}
	resp, err := chat.Generate(ctx, messages)
	if err != nil {
		return 0, fmt.Errorf("评判答案失败: %w", err)
	}
	if resp == nil {
		return 0, fmt.Errorf("评判答案失败: 模型返回空响应")
	}

	payload, err := parseScoreJSON(resp.Content)
	if err != nil {
		return 0, fmt.Errorf("解析评判结果失败: %w", err)
	}
	score := payload["score"]
	switch v := score.(type) {
	case float64:
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return v, nil
	default:
		return 0, fmt.Errorf("评判结果缺少有效 score 字段")
	}
}

// parseQAJSON 解析 LLM 返回的问答对 JSON，兼容 ```json 代码块包裹。
func parseQAJSON(content string) (map[string]string, error) {
	raw, err := extractJSON(content)
	if err != nil {
		return nil, err
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("JSON 格式错误: %w", err)
	}
	return payload, nil
}

// parseScoreJSON 解析 LLM 返回的评分 JSON，兼容裸数字与 ```json 代码块包裹。
func parseScoreJSON(content string) (map[string]any, error) {
	// 优先兼容直接输出数字的情况（如 0.85）
	if v, parseErr := strconv.ParseFloat(strings.TrimSpace(content), 64); parseErr == nil {
		return map[string]any{"score": v}, nil
	}
	raw, err := extractJSON(content)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("JSON 格式错误: %w", err)
	}
	return payload, nil
}

// extractJSON 从 LLM 输出中提取首个 JSON 对象。
// 模型可能用 ```json 代码块包裹，或夹杂说明文字，这里取首个 { 到最后一个 } 之间的内容。
func extractJSON(content string) ([]byte, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("输出中未找到 JSON 对象")
	}
	return []byte(content[start : end+1]), nil
}

// logLLMError 记录 LLM 调用错误，避免重复的错误处理样板。
func logLLMError(stage string, err error) {
	if logger.Initialized() {
		logger.Warn("评估 LLM 调用失败", zap.String("stage", stage), zap.Error(err))
	}
}
