package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	apperrors "Qavor/pkg/errors"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// 评估模块业务错误码（71xxx）。
const (
	CodeEvaluationKBNotFound       = 71001
	CodeEvaluationDatasetNotFound  = 71002
	CodeEvaluationInvalidFile      = 71003
	CodeEvaluationGenerateFailed   = 71004
	CodeEvaluationModelInvalid     = 71005
	CodeEvaluationRunNotFound      = 71006
	CodeEvaluationDatasetBusy      = 71007
	CodeEvaluationGraphUnsupported = 71008
	CodeEvaluationRunConflict      = 71009
)

// maxDatasetFileSize 上传数据集文件大小上限（100MB）。
const maxDatasetFileSize = 100 * 1024 * 1024

// datasetBuildStatus 数据集构建状态。
const (
	datasetBuildPending   = "pending"
	datasetBuildRunning   = "running"
	datasetBuildCompleted = "completed"
	datasetBuildFailed    = "failed"
)

// runStatus 评估运行状态。
const (
	runStatusRunning   = "running"
	runStatusCompleted = "completed"
	runStatusFailed    = "failed"
)

// datasetSource 数据集来源。
const (
	datasetSourceUploaded  = "uploaded"
	datasetSourceGenerated = "generated"
)

// evaluationService 评估基准与评估运行服务实现。
type evaluationService struct {
	repo      repository.EvaluationRepository
	kbRepo    repository.KnowledgeBaseRepository
	chunkRepo repository.KnowledgeChunkRepository
	ragSvc    RAGService
	modelSvc  ModelService
	llm       *evalLLM
	runner    *evaluationRunner
}

// NewEvaluationService 创建评估服务。runner 由 Start 启动。
func NewEvaluationService(
	repo repository.EvaluationRepository,
	kbRepo repository.KnowledgeBaseRepository,
	chunkRepo repository.KnowledgeChunkRepository,
	ragSvc RAGService,
	modelSvc ModelService,
) EvaluationService {
	svc := &evaluationService{
		repo:      repo,
		kbRepo:    kbRepo,
		chunkRepo: chunkRepo,
		ragSvc:    ragSvc,
		modelSvc:  modelSvc,
		llm:       &evalLLM{modelSvc: modelSvc},
	}
	svc.runner = newEvaluationRunner(svc)
	return svc
}

// Start 启动后台执行器，处理挂起的生成任务与评估运行。
func (s *evaluationService) Start(ctx context.Context) {
	if s.runner != nil {
		s.runner.Start(ctx)
	}
}

// Stop 停止后台执行器。
func (s *evaluationService) Stop() {
	if s.runner != nil {
		s.runner.Stop()
	}
}

// UploadDataset 上传 JSONL 评测数据集文件并解析入库。
func (s *evaluationService) UploadDataset(ctx context.Context, kbID, name, description string, file []byte, filename string) (*EvaluationDatasetDTO, error) {
	kbID = strings.TrimSpace(kbID)
	if _, err := s.requireKB(ctx, kbID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperrors.New(CodeEvaluationInvalidFile, "基准名称不能为空")
	}
	if len(file) == 0 {
		return nil, apperrors.New(CodeEvaluationInvalidFile, "基准文件不能为空")
	}
	if len(file) > maxDatasetFileSize {
		return nil, apperrors.New(CodeEvaluationInvalidFile, "基准文件大小不能超过 100MB")
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".jsonl") {
		return nil, apperrors.New(CodeEvaluationInvalidFile, "仅支持 JSONL 格式文件")
	}

	items, hasGoldChunks, hasGoldAnswers, err := parseDatasetJSONL(file)
	if err != nil {
		return nil, apperrors.New(CodeEvaluationInvalidFile, err.Error())
	}

	dataset := &entity.EvaluationDataset{
		DatasetID:     newEvaluationID("ds"),
		KBID:          kbID,
		Name:          name,
		Description:   description,
		ItemCount:     len(items),
		HasGoldChunks: hasGoldChunks,
		HasGoldAnswer: hasGoldAnswers,
		BuildMetadata: entity.JSON{
			"status": datasetBuildCompleted,
			"source": datasetSourceUploaded,
		},
	}
	if err := s.repo.CreateDataset(dataset); err != nil {
		return nil, err
	}
	// 补充分区键后批量写入条目
	for _, item := range items {
		item.DatasetID = dataset.DatasetID
	}
	if err := s.repo.ReplaceDatasetItems(dataset.DatasetID, items); err != nil {
		return nil, err
	}
	return datasetToDTO(dataset), nil
}

// ListDatasets 获取知识库下的数据集列表。
func (s *evaluationService) ListDatasets(kbID string) ([]*EvaluationDatasetDTO, error) {
	datasets, err := s.repo.ListDatasetsByKBID(kbID)
	if err != nil {
		return nil, err
	}
	result := make([]*EvaluationDatasetDTO, 0, len(datasets))
	for _, d := range datasets {
		result = append(result, datasetToDTO(d))
	}
	return result, nil
}

// GetDataset 获取数据集详情（分页查看问答条目）。
func (s *evaluationService) GetDataset(ctx context.Context, kbID, datasetID string, page, pageSize int) (*EvaluationDatasetDetailDTO, error) {
	if _, err := s.requireKB(ctx, kbID); err != nil {
		return nil, err
	}
	dataset, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	if dataset.KBID != kbID {
		return nil, apperrors.New(CodeEvaluationDatasetNotFound, "数据集不存在")
	}

	page, pageSize = normalizePage(page, pageSize)
	total, err := s.repo.CountDatasetItems(datasetID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListDatasetItems(datasetID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}

	dto := &EvaluationDatasetDetailDTO{
		EvaluationDatasetDTO: *datasetToDTO(dataset),
		Items:                make([]EvaluationDatasetItemDTO, 0, len(rows)),
		Pagination: &EvaluationPagination{
			Total:      int(total),
			TotalPages: totalPages(int(total), pageSize),
			Page:       page,
			PageSize:   pageSize,
		},
	}
	for _, item := range rows {
		dto.Items = append(dto.Items, datasetItemToDTO(item))
	}
	return dto, nil
}

// DeleteDataset 删除数据集及其条目。
func (s *evaluationService) DeleteDataset(datasetID string) error {
	dataset, err := s.requireDataset(datasetID)
	if err != nil {
		return err
	}
	if dataset.BuildMetadata["status"] == datasetBuildPending || dataset.BuildMetadata["status"] == datasetBuildRunning {
		return apperrors.New(CodeEvaluationDatasetBusy, "评估基准生成中，暂不能删除")
	}
	return s.repo.DeleteDatasetByID(datasetID)
}

// DownloadDataset 导出数据集为 JSONL。
func (s *evaluationService) DownloadDataset(datasetID string) (string, []byte, error) {
	dataset, err := s.requireDataset(datasetID)
	if err != nil {
		return "", nil, err
	}
	if dataset.BuildMetadata["status"] != datasetBuildCompleted {
		return "", nil, apperrors.New(CodeEvaluationDatasetBusy, "评估基准生成完成后才能下载")
	}
	items, err := s.repo.ListAllDatasetItems(datasetID)
	if err != nil {
		return "", nil, err
	}

	var buf bytes.Buffer
	for _, item := range items {
		line := map[string]any{
			"query":          item.Query,
			"gold_chunk_ids": item.GoldChunkIDs,
			"gold_answer":    item.GoldAnswer,
		}
		raw, err := json.Marshal(line)
		if err != nil {
			return "", nil, err
		}
		buf.Write(raw)
		buf.WriteByte('\n')
	}

	filename := dataset.Name + ".jsonl"
	if strings.ContainsAny(filename, "\r\n\"") {
		filename = dataset.DatasetID + ".jsonl"
	}
	return filename, buf.Bytes(), nil
}

// GenerateDataset 提交 AI 自动生成评测数据集任务（异步执行）。
func (s *evaluationService) GenerateDataset(ctx context.Context, kbID string, req *GenerateDatasetRequest) (*EvaluationDatasetDTO, error) {
	if _, err := s.requireKB(ctx, kbID); err != nil {
		return nil, err
	}
	if err := validateGenerateRequest(req); err != nil {
		return nil, err
	}
	modelID, err := s.resolveModelID(req.LLMModelSpec, "chat")
	if err != nil {
		return nil, err
	}
	if err := s.checkGenerateModel(ctx, modelID); err != nil {
		return nil, err
	}

	params := map[string]any{
		"count":              req.Count,
		"neighbors_count":    req.NeighborsCount,
		"concurrency_count":  req.ConcurrencyCount,
		"generation_mode":    req.GenerationMode,
		"graph_expand_top_k": req.GraphExpandTopK,
		"llm_model_spec":     req.LLMModelSpec,
		"llm_model_id":       modelID,
	}
	dataset := &entity.EvaluationDataset{
		DatasetID:   newEvaluationID("ds"),
		KBID:        kbID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		BuildMetadata: entity.JSON{
			"status":   datasetBuildPending,
			"source":   datasetSourceGenerated,
			"progress": 0,
			"params":   params,
		},
	}
	if err := s.repo.CreateDataset(dataset); err != nil {
		return nil, err
	}
	return datasetToDTO(dataset), nil
}

// ResumeDatasetGeneration 恢复失败的自动生成任务。
func (s *evaluationService) ResumeDatasetGeneration(ctx context.Context, kbID, datasetID string) (*ResumeDatasetResponse, error) {
	dataset, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	if dataset.KBID != kbID {
		return nil, apperrors.New(CodeEvaluationDatasetNotFound, "数据集不存在")
	}
	metadata := dataset.BuildMetadata
	if metadata["source"] != datasetSourceGenerated {
		return nil, apperrors.New(CodeEvaluationInvalidFile, "只有自动生成的基准才能恢复")
	}
	if metadata["status"] == datasetBuildPending || metadata["status"] == datasetBuildRunning {
		return nil, apperrors.New(CodeEvaluationDatasetBusy, "基准生成任务已在进行中")
	}
	metadata["status"] = datasetBuildPending
	metadata["progress"] = 0
	delete(metadata, "error_message")
	delete(metadata, "message")
	dataset.BuildMetadata = metadata
	if err := s.repo.UpdateDataset(dataset); err != nil {
		return nil, err
	}
	return &ResumeDatasetResponse{Message: "已恢复生成"}, nil
}

// RunEvaluation 发起一次 RAG 评估运行（异步执行）。
func (s *evaluationService) RunEvaluation(ctx context.Context, kbID string, req *RunEvaluationRequest) (*EvaluationRunDTO, error) {
	if _, err := s.requireKB(ctx, kbID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperrors.New(apperrors.CodeInvalidParam, "评估名称不能为空")
	}
	dataset, err := s.requireDataset(req.DatasetID)
	if err != nil {
		return nil, err
	}
	if dataset.KBID != kbID {
		return nil, apperrors.New(CodeEvaluationDatasetNotFound, "数据集不存在")
	}
	if dataset.BuildMetadata["status"] != datasetBuildCompleted {
		return nil, apperrors.New(CodeEvaluationDatasetBusy, "评估基准尚未生成完成")
	}

	retrievalConfig := map[string]any{
		"top_k": evalTopK,
	}
	answerLLM := strings.TrimSpace(req.ModelConfig.AnswerLLM)
	judgeLLM := strings.TrimSpace(req.ModelConfig.JudgeLLM)
	if dataset.HasGoldAnswer {
		if (answerLLM == "") != (judgeLLM == "") {
			return nil, apperrors.New(apperrors.CodeInvalidParam, "答案生成模型和答案评判模型需要同时提供")
		}
		if answerLLM != "" {
			if _, err := s.resolveModelID(answerLLM, "chat"); err != nil {
				return nil, err
			}
			if _, err := s.resolveModelID(judgeLLM, "chat"); err != nil {
				return nil, err
			}
			retrievalConfig["answer_llm"] = answerLLM
			retrievalConfig["judge_llm"] = judgeLLM
		}
	}

	now := time.Now()
	run := &entity.EvaluationRun{
		RunID:           newEvaluationID("run"),
		KBID:            kbID,
		DatasetID:       dataset.DatasetID,
		Name:            name,
		Status:          runStatusRunning,
		StartedAt:       &now,
		TotalItems:      dataset.ItemCount,
		RetrievalConfig: entity.JSON(retrievalConfig),
	}
	if err := s.repo.CreateRun(run); err != nil {
		return nil, err
	}
	return runToDTO(run), nil
}

// ListRuns 获取知识库下的评估运行列表。
func (s *evaluationService) ListRuns(kbID string) ([]*EvaluationRunDTO, error) {
	runs, err := s.repo.ListRunsByKBID(kbID)
	if err != nil {
		return nil, err
	}
	result := make([]*EvaluationRunDTO, 0, len(runs))
	for _, r := range runs {
		result = append(result, runToDTO(r))
	}
	return result, nil
}

// GetRunResults 获取单次评估运行的结果（分页，支持 error_only 过滤）。
func (s *evaluationService) GetRunResults(ctx context.Context, kbID, runID string, page, pageSize int, errorOnly bool) (*EvaluationRunDetailDTO, error) {
	run, err := s.requireRun(runID)
	if err != nil {
		return nil, err
	}
	if run.KBID != kbID {
		return nil, apperrors.New(CodeEvaluationRunNotFound, "评估运行不存在")
	}

	page, pageSize = normalizePage(page, pageSize)
	total, err := s.repo.CountRunResults(runID, errorOnly)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListRunResults(runID, (page-1)*pageSize, pageSize, errorOnly)
	if err != nil {
		return nil, err
	}

	dto := &EvaluationRunDetailDTO{
		RunID:           run.RunID,
		Name:            run.Name,
		Status:          run.Status,
		StartedAt:       formatTime(run.StartedAt),
		CompletedAt:     formatTime(run.CompletedAt),
		TotalItems:      run.TotalItems,
		CompletedItems:  run.CompletedItems,
		OverallScore:    run.OverallScore,
		RetrievalConfig: run.RetrievalConfig,
		Items:           make([]EvaluationRunResultDTO, 0, len(rows)),
		Pagination: &EvaluationRunPagination{
			Total:      int(total),
			TotalPages: totalPages(int(total), pageSize),
			Page:       page,
			PageSize:   pageSize,
		},
	}
	for _, row := range rows {
		dto.Items = append(dto.Items, runResultToDTO(row))
	}
	return dto, nil
}

// DeleteRun 删除评估运行及其结果。
func (s *evaluationService) DeleteRun(kbID, runID string) error {
	run, err := s.requireRun(runID)
	if err != nil {
		return err
	}
	if run.KBID != kbID {
		return apperrors.New(CodeEvaluationRunNotFound, "评估运行不存在")
	}
	return s.repo.DeleteRunByID(runID)
}

// requireKB 校验知识库存在。
func (s *evaluationService) requireKB(ctx context.Context, kbID string) (*entity.KnowledgeBase, error) {
	base, err := s.kbRepo.FindByKBID(kbID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, apperrors.New(CodeEvaluationKBNotFound, "知识库不存在")
	}
	return base, nil
}

// requireDataset 查询数据集，不存在时返回业务错误。
func (s *evaluationService) requireDataset(datasetID string) (*entity.EvaluationDataset, error) {
	dataset, err := s.repo.FindDatasetByID(datasetID)
	if err != nil {
		return nil, err
	}
	if dataset == nil {
		return nil, apperrors.New(CodeEvaluationDatasetNotFound, "评估基准不存在")
	}
	return dataset, nil
}

// requireRun 查询评估运行，不存在时返回业务错误。
func (s *evaluationService) requireRun(runID string) (*entity.EvaluationRun, error) {
	run, err := s.repo.FindRunByID(runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, apperrors.New(CodeEvaluationRunNotFound, "评估运行不存在")
	}
	return run, nil
}

// resolveModelID 将模型 ID 字符串解析为 uint，并校验模型存在且类型匹配。
func (s *evaluationService) resolveModelID(spec, modelType string) (uint, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, apperrors.New(CodeEvaluationModelInvalid, "请选择模型")
	}
	id, err := strconv.ParseUint(spec, 10, 64)
	if err != nil || id == 0 {
		return 0, apperrors.New(CodeEvaluationModelInvalid, "模型参数无效")
	}
	return uint(id), nil
}

// checkGenerateModel 校验生成基准使用的模型可用（chat 类型）。
func (s *evaluationService) checkGenerateModel(ctx context.Context, modelID uint) error {
	model, err := s.modelSvc.GetModel(modelID)
	if err != nil {
		return err
	}
	if model == nil {
		return apperrors.New(CodeEvaluationModelInvalid, "模型不存在")
	}
	if model.ModelType != "chat" {
		return apperrors.New(CodeEvaluationModelInvalid, "生成基准需要 chat 类型模型")
	}
	return nil
}

// validateGenerateRequest 校验自动生成基准的请求参数。
func validateGenerateRequest(req *GenerateDatasetRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return apperrors.New(apperrors.CodeInvalidParam, "基准名称不能为空")
	}
	if req.Count < 1 || req.Count > 100 {
		return apperrors.New(apperrors.CodeInvalidParam, "问题数量需在 1-100 之间")
	}
	if req.NeighborsCount < 0 || req.NeighborsCount > 10 {
		return apperrors.New(apperrors.CodeInvalidParam, "候选 Chunk 数量需在 0-10 之间")
	}
	if req.ConcurrencyCount < 1 || req.ConcurrencyCount > 20 {
		return apperrors.New(apperrors.CodeInvalidParam, "构建并发数需在 1-20 之间")
	}
	switch req.GenerationMode {
	case "vector":
	case "graph_enhanced":
		if req.GraphExpandTopK < 1 || req.GraphExpandTopK > 10 {
			return apperrors.New(apperrors.CodeInvalidParam, "图增强扩展数量需在 1-10 之间")
		}
	default:
		return apperrors.New(apperrors.CodeInvalidParam, "构建方式无效")
	}
	return nil
}

// parseDatasetJSONL 解析 JSONL 评测数据集。
// 每行 JSON：{"query": "...", "gold_chunk_ids": [...], "gold_answer": "..."}。
func parseDatasetJSONL(file []byte) (items []*entity.EvaluationDatasetItem, hasGoldChunks, hasGoldAnswers bool, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(file))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 单行最大 4MB

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			Query        string   `json:"query"`
			GoldChunkIDs []string `json:"gold_chunk_ids"`
			GoldAnswer   string   `json:"gold_answer"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, false, false, fmt.Errorf("第 %d 行 JSON 格式错误: %v", lineNo, err)
		}
		if strings.TrimSpace(raw.Query) == "" {
			return nil, false, false, fmt.Errorf("第 %d 行缺少 query 字段", lineNo)
		}
		item := &entity.EvaluationDatasetItem{
			Query:        strings.TrimSpace(raw.Query),
			GoldChunkIDs: entity.JSONArray{},
			GoldAnswer:   strings.TrimSpace(raw.GoldAnswer),
			SortOrder:    len(items),
		}
		for _, id := range raw.GoldChunkIDs {
			if strings.TrimSpace(id) != "" {
				item.GoldChunkIDs = append(item.GoldChunkIDs, strings.TrimSpace(id))
			}
		}
		if len(item.GoldChunkIDs) > 0 {
			hasGoldChunks = true
		}
		if item.GoldAnswer != "" {
			hasGoldAnswers = true
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, false, false, fmt.Errorf("读取文件失败: %v", err)
	}
	if len(items) == 0 {
		return nil, false, false, errors.New("文件不能为空")
	}
	return items, hasGoldChunks, hasGoldAnswers, nil
}

// newEvaluationID 生成评估模块唯一标识。
func newEvaluationID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// normalizePage 归一化分页参数。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// totalPages 计算总页数。
func totalPages(total, pageSize int) int {
	if total == 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}

// formatTime 将时间指针格式化为 RFC3339 字符串。
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// datasetToDTO 数据集实体转响应。
func datasetToDTO(dataset *entity.EvaluationDataset) *EvaluationDatasetDTO {
	metadata := make(map[string]any, len(dataset.BuildMetadata))
	for k, v := range dataset.BuildMetadata {
		metadata[k] = v
	}
	return &EvaluationDatasetDTO{
		DatasetID:      dataset.DatasetID,
		KBID:           dataset.KBID,
		Name:           dataset.Name,
		Description:    dataset.Description,
		ItemCount:      dataset.ItemCount,
		HasGoldChunks:  dataset.HasGoldChunks,
		HasGoldAnswers: dataset.HasGoldAnswer,
		BuildMetadata:  metadata,
		CreatedAt:      dataset.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      dataset.UpdatedAt.Format(time.RFC3339),
	}
}

// datasetItemToDTO 数据集条目转响应。
func datasetItemToDTO(item *entity.EvaluationDatasetItem) EvaluationDatasetItemDTO {
	chunkIDs := make([]string, 0, len(item.GoldChunkIDs))
	for _, id := range item.GoldChunkIDs {
		if s, ok := id.(string); ok {
			chunkIDs = append(chunkIDs, s)
		}
	}
	return EvaluationDatasetItemDTO{
		Query:        item.Query,
		GoldChunkIDs: chunkIDs,
		GoldAnswer:   item.GoldAnswer,
	}
}

// runToDTO 评估运行实体转响应。
func runToDTO(run *entity.EvaluationRun) *EvaluationRunDTO {
	metrics := make(map[string]any, len(run.Metrics))
	for k, v := range run.Metrics {
		metrics[k] = v
	}
	retrievalConfig := make(map[string]any, len(run.RetrievalConfig))
	for k, v := range run.RetrievalConfig {
		retrievalConfig[k] = v
	}
	return &EvaluationRunDTO{
		RunID:           run.RunID,
		KBID:            run.KBID,
		DatasetID:       run.DatasetID,
		Name:            run.Name,
		Status:          run.Status,
		StartedAt:       formatTime(run.StartedAt),
		CompletedAt:     formatTime(run.CompletedAt),
		TotalItems:      run.TotalItems,
		CompletedItems:  run.CompletedItems,
		OverallScore:    run.OverallScore,
		Metrics:         metrics,
		RetrievalConfig: retrievalConfig,
		Progress:        run.Progress,
		Message:         run.Message,
		CreatedAt:       run.CreatedAt.Format(time.RFC3339),
	}
}

// runResultToDTO 运行单项结果转响应。
func runResultToDTO(row *entity.EvaluationRunResult) EvaluationRunResultDTO {
	metrics := make(map[string]any, len(row.Metrics))
	for k, v := range row.Metrics {
		metrics[k] = v
	}
	return EvaluationRunResultDTO{
		Query:           row.Query,
		GeneratedAnswer: row.GeneratedAnswer,
		Metrics:         metrics,
		AnswerScore:     row.AnswerScore,
		Error:           row.ErrorMessage,
		Status:          row.Status,
	}
}

// logEvalError 记录评估模块错误日志。
func logEvalError(stage string, err error) {
	if logger.Initialized() {
		logger.Warn("评估模块执行失败", zap.String("stage", stage), zap.Error(err))
	}
}
