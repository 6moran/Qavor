package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"go.uber.org/zap"
)

// generateParams 数据集生成任务参数（持久化在 build_metadata.params 中）。
type generateParams struct {
	Count            int
	NeighborsCount   int
	ConcurrencyCount int
	LLMModelID       uint
	GenerationMode   string
	GraphExpandTopK  int
}

// parseGenerateParams 从构建元数据中解析生成参数。
func parseGenerateParams(metadata entity.JSON) (generateParams, error) {
	params, ok := metadata["params"].(map[string]any)
	if !ok {
		return generateParams{}, fmt.Errorf("生成参数缺失")
	}
	p := generateParams{
		Count:            toInt(params["count"]),
		NeighborsCount:   toInt(params["neighbors_count"]),
		ConcurrencyCount: toInt(params["concurrency_count"]),
		LLMModelID:       uint(toInt(params["llm_model_id"])),
		GenerationMode:   stringValue(params["generation_mode"]),
		GraphExpandTopK:  toInt(params["graph_expand_top_k"]),
	}
	if p.Count < 1 {
		return generateParams{}, fmt.Errorf("生成参数无效: count")
	}
	if p.NeighborsCount < 1 {
		p.NeighborsCount = 1
	}
	if p.ConcurrencyCount < 1 {
		p.ConcurrencyCount = 1
	}
	if p.LLMModelID == 0 {
		return generateParams{}, fmt.Errorf("生成参数无效: 模型缺失")
	}
	if p.GenerationMode == "" {
		p.GenerationMode = "vector"
	}
	if p.GenerationMode == "graph_enhanced" && p.GraphExpandTopK < 1 {
		p.GraphExpandTopK = p.NeighborsCount
	}
	return p, nil
}

// toInt 将 JSON 数字（float64）或整数安全转为 int。
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// stringValue 将任务元数据中的字符串安全转换出来。
func stringValue(v any) string {
	if value, ok := v.(string); ok {
		return value
	}
	return ""
}

// runGenerateTask 执行数据集自动生成任务。
func (s *evaluationService) runGenerateTask(ctx context.Context, dataset *entity.EvaluationDataset) {
	datasetID := dataset.DatasetID
	params, err := parseGenerateParams(dataset.BuildMetadata)
	if err != nil {
		s.failDataset(datasetID, "生成参数无效: "+err.Error())
		return
	}

	// 置为运行中
	if err := s.updateDatasetStatus(datasetID, datasetBuildRunning, 0, "开始生成评估基准"); err != nil {
		logEvalError("更新数据集状态失败", err)
		return
	}

	groups, err := s.buildEvaluationGroups(ctx, dataset.KBID, params)
	if err != nil {
		s.failDataset(datasetID, "采样知识片段失败: "+err.Error())
		return
	}
	if len(groups) == 0 {
		s.failDataset(datasetID, fmt.Sprintf("知识片段不足（需要至少 %d 个）", params.NeighborsCount))
		return
	}

	logRunner("开始生成评估基准",
		zap.String("dataset_id", datasetID),
		zap.Int("groups", len(groups)),
		zap.Int("concurrency", params.ConcurrencyCount),
	)

	// 并发生成问答对
	type groupResult struct {
		query      string
		goldAnswer string
		chunkIDs   []string
		err        error
	}
	results := make([]groupResult, len(groups))
	sem := make(chan struct{}, params.ConcurrencyCount)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	for i, group := range groups {
		select {
		case <-ctx.Done():
			s.failDataset(datasetID, "任务已取消")
			return
		default:
		}
		wg.Add(1)
		go func(idx int, group []repository.ChunkWithFile) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			refs := make([]chunkRef, 0, len(group))
			ids := make([]string, 0, len(group))
			for _, c := range group {
				refs = append(refs, chunkRef{ID: c.ChunkID, Content: c.Content})
				ids = append(ids, c.ChunkID)
			}
			query, answer, err := s.llm.generateQAPair(ctx, params.LLMModelID, refs)
			results[idx] = groupResult{query: query, goldAnswer: answer, chunkIDs: ids, err: err}
			if err != nil {
				logLLMError("生成问答对", err)
			}

			mu.Lock()
			completed++
			// 节流：每 3 组或每 2 秒更新一次进度
			if completed%3 == 0 || time.Now().UnixMilli()%2000 < 100 {
				progress := float64(completed) / float64(len(groups)) * 100
				_ = s.updateDatasetStatus(datasetID, datasetBuildRunning, progress, fmt.Sprintf("已生成 %d/%d", completed, len(groups)))
			}
			mu.Unlock()
		}(i, group)
	}
	wg.Wait()

	// 汇总结果
	items := make([]*entity.EvaluationDatasetItem, 0, len(groups))
	hasGoldAnswer := false
	failedCount := 0
	for _, r := range results {
		if r.err != nil || strings.TrimSpace(r.query) == "" {
			failedCount++
			continue
		}
		hasGoldAnswer = hasGoldAnswer || r.goldAnswer != ""
		items = append(items, &entity.EvaluationDatasetItem{
			DatasetID:    datasetID,
			Query:        r.query,
			GoldChunkIDs: toJSONArray(r.chunkIDs),
			GoldAnswer:   r.goldAnswer,
			SortOrder:    len(items),
		})
	}

	if len(items) == 0 {
		s.failDataset(datasetID, "问答对生成全部失败，请检查模型配置后重试")
		return
	}

	// 写入条目并更新数据集
	if err := s.repo.ReplaceDatasetItems(datasetID, items); err != nil {
		s.failDataset(datasetID, "保存生成结果失败: "+err.Error())
		return
	}
	dataset.ItemCount = len(items)
	dataset.HasGoldChunks = true
	dataset.HasGoldAnswer = hasGoldAnswer
	metadata := dataset.BuildMetadata
	metadata["status"] = datasetBuildCompleted
	metadata["progress"] = 100
	if failedCount > 0 {
		metadata["message"] = fmt.Sprintf("生成完成，%d/%d 组失败已跳过", failedCount, len(groups))
	}
	delete(metadata, "error_message")
	dataset.BuildMetadata = metadata
	if err := s.repo.UpdateDataset(dataset); err != nil {
		logEvalError("更新数据集完成状态失败", err)
		return
	}
	logRunner("评估基准生成完成", zap.String("dataset_id", datasetID), zap.Int("items", len(items)))
}

// buildEvaluationGroups 按生成模式构造用于生成问题的知识片段组。
func (s *evaluationService) buildEvaluationGroups(ctx context.Context, kbID string, params generateParams) ([][]repository.ChunkWithFile, error) {
	if params.GenerationMode != "graph_enhanced" {
		sampleCount := params.Count * params.NeighborsCount
		chunks, err := s.chunkRepo.FindRandomByKBIDs(ctx, []string{kbID}, sampleCount)
		if err != nil {
			return nil, err
		}
		groups := make([][]repository.ChunkWithFile, 0, params.Count)
		for i := 0; i+params.NeighborsCount <= len(chunks); i += params.NeighborsCount {
			groups = append(groups, chunks[i:i+params.NeighborsCount])
		}
		return groups, nil
	}

	// 图增强模式在当前存储模型中使用同一文件的相邻分块扩展上下文，
	// 这样不依赖未落地的外部图数据库，也能让基准覆盖跨分块上下文。
	seeds, err := s.chunkRepo.FindRandomByKBIDs(ctx, []string{kbID}, params.Count)
	if err != nil {
		return nil, err
	}
	groups := make([][]repository.ChunkWithFile, 0, len(seeds))
	windowSize := params.GraphExpandTopK
	if windowSize < params.NeighborsCount {
		windowSize = params.NeighborsCount
	}
	for _, seed := range seeds {
		allChunks, err := s.chunkRepo.FindByFileID(ctx, kbID, seed.FileID)
		if err != nil {
			return nil, err
		}
		seedIndex := -1
		for i, chunk := range allChunks {
			if chunk.ChunkID == seed.ChunkID {
				seedIndex = i
				break
			}
		}
		if seedIndex < 0 {
			continue
		}
		start := seedIndex - windowSize/2
		if start < 0 {
			start = 0
		}
		end := start + windowSize
		if end > len(allChunks) {
			end = len(allChunks)
			start = end - windowSize
			if start < 0 {
				start = 0
			}
		}
		group := make([]repository.ChunkWithFile, 0, end-start)
		for _, chunk := range allChunks[start:end] {
			group = append(group, repository.ChunkWithFile{
				ChunkID: chunk.ChunkID, KBID: chunk.KBID, FileID: chunk.FileID,
				Filename: seed.Filename, Content: chunk.Content,
			})
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// updateDatasetStatus 更新数据集构建状态与进度。
func (s *evaluationService) updateDatasetStatus(datasetID, status string, progress float64, message string) error {
	dataset, err := s.repo.FindDatasetByID(datasetID)
	if err != nil || dataset == nil {
		return fmt.Errorf("数据集不存在")
	}
	metadata := dataset.BuildMetadata
	metadata["status"] = status
	if progress >= 0 {
		metadata["progress"] = progress
	}
	if message != "" {
		metadata["message"] = message
	}
	dataset.BuildMetadata = metadata
	return s.repo.UpdateDataset(dataset)
}

// failDataset 将数据集生成任务标记为失败。
func (s *evaluationService) failDataset(datasetID, message string) {
	if err := s.updateDatasetStatus(datasetID, datasetBuildFailed, -1, message); err != nil {
		logEvalError("标记数据集任务失败", err)
		return
	}
	// 补充 error_message 字段（前端优先展示）
	dataset, err := s.repo.FindDatasetByID(datasetID)
	if err != nil || dataset == nil {
		return
	}
	metadata := dataset.BuildMetadata
	metadata["error_message"] = message
	dataset.BuildMetadata = metadata
	if err := s.repo.UpdateDataset(dataset); err != nil {
		logEvalError("写入数据集错误信息失败", err)
	}
}

// toJSONArray 将字符串切片转换为实体 JSON 数组。
func toJSONArray(ids []string) entity.JSONArray {
	arr := make(entity.JSONArray, 0, len(ids))
	for _, id := range ids {
		arr = append(arr, id)
	}
	return arr
}

// itemResult 评估运行单项执行结果（内存中转，最终写入数据库）。
type itemResult struct {
	query           string
	metrics         map[string]any
	answerScore     *float64
	generatedAnswer string
	err             string
	status          string
}

// runEvaluationTask 执行一次评估运行。
func (s *evaluationService) runEvaluationTask(ctx context.Context, run *entity.EvaluationRun) {
	runID := run.RunID

	// 加载数据集条目
	items, err := s.repo.ListAllDatasetItems(run.DatasetID)
	if err != nil {
		s.failRun(runID, "加载数据集失败: "+err.Error())
		return
	}
	if len(items) == 0 {
		s.failRun(runID, "评估基准为空")
		return
	}
	run.TotalItems = len(items)
	if err := s.repo.UpdateRun(run); err != nil {
		logEvalError("更新运行失败", err)
		return
	}

	answerModelID, judgeModelID, useAnswerEval := parseRunModels(run.RetrievalConfig)
	logRunner("开始评估运行",
		zap.String("run_id", runID),
		zap.Int("items", len(items)),
		zap.Bool("answer_eval", useAnswerEval),
	)

	// 逐条执行（并发上限 4）
	results := make([]itemResult, len(items))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	for i, item := range items {
		select {
		case <-ctx.Done():
			s.failRun(runID, "任务已取消")
			return
		default:
		}
		wg.Add(1)
		go func(idx int, item *entity.EvaluationDatasetItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := itemResult{query: item.Query, status: "completed"}
			retrieved, err := s.ragSvc.Retrieve(ctx, []string{run.KBID}, item.Query, evalTopK)
			if err != nil {
				result.status = "error"
				result.err = "检索失败: " + err.Error()
			} else {
				retrievedIDs := make([]string, 0, len(retrieved.Chunks))
				refs := make([]chunkRef, 0, len(retrieved.Chunks))
				for _, c := range retrieved.Chunks {
					retrievedIDs = append(retrievedIDs, c.ChunkID)
					if c.Content != "" {
						refs = append(refs, chunkRef{ID: c.ChunkID, Content: c.Content})
					}
				}
				metrics := computeRetrievalMetrics(retrievedIDs, toStringSlice(item.GoldChunkIDs))
				result.metrics = metrics.toMap()

				// 答案评估：基准含 Gold Answer 且配置了生成/评判模型
				if useAnswerEval && item.GoldAnswer != "" {
					answer, genErr := s.llm.generateAnswer(ctx, answerModelID, item.Query, refs)
					if genErr != nil {
						result.status = "error"
						result.err = "生成答案失败: " + genErr.Error()
					} else {
						result.generatedAnswer = answer
						score, judgeErr := s.llm.judgeAnswer(ctx, judgeModelID, item.Query, item.GoldAnswer, answer)
						if judgeErr != nil {
							result.status = "error"
							result.err = "评判答案失败: " + judgeErr.Error()
						} else {
							result.answerScore = &score
							result.metrics["score"] = score
						}
					}
				}
			}

			results[idx] = result

			mu.Lock()
			completed++
			if completed%5 == 0 || completed == len(items) {
				run.CompletedItems = completed
				run.Progress = float64(completed) / float64(len(items)) * 100
				_ = s.repo.UpdateRun(run)
			}
			mu.Unlock()
		}(i, item)
	}
	wg.Wait()

	// 汇总指标
	s.finalizeRun(run, items, results)
}

// parseRunModels 从检索配置中解析答案生成与评判模型 ID。
func parseRunModels(config entity.JSON) (answerModelID, judgeModelID uint, useAnswerEval bool) {
	if answer, ok := config["answer_llm"].(string); ok && answer != "" {
		if id, err := parseModelIDString(answer); err == nil {
			answerModelID = id
		}
	}
	if judge, ok := config["judge_llm"].(string); ok && judge != "" {
		if id, err := parseModelIDString(judge); err == nil {
			judgeModelID = id
		}
	}
	useAnswerEval = answerModelID != 0 && judgeModelID != 0
	return answerModelID, judgeModelID, useAnswerEval
}

// parseModelIDString 将模型 ID 字符串转为 uint。
func parseModelIDString(s string) (uint, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("无效模型 ID: %s", s)
	}
	return uint(id), nil
}

// finalizeRun 汇总评估结果并写入运行与单项结果。
func (s *evaluationService) finalizeRun(run *entity.EvaluationRun, items []*entity.EvaluationDatasetItem, results []itemResult) {
	completedResults := make([]*entity.EvaluationRunResult, 0, len(items))
	errorCount := 0
	var scoreSum, scoreCount float64
	var recallSum, recallCount float64

	for i, item := range items {
		// 按索引位置对齐并发完成的结果（保持数据集条目顺序）
		result := results[i]
		row := &entity.EvaluationRunResult{
			RunID:           run.RunID,
			Query:           item.Query,
			GeneratedAnswer: result.generatedAnswer,
			Metrics:         entity.JSON(result.metrics),
			AnswerScore:     result.answerScore,
			ErrorMessage:    result.err,
			Status:          result.status,
			SortOrder:       i,
		}
		completedResults = append(completedResults, row)
		if result.status == "error" {
			errorCount++
			continue
		}
		if result.answerScore != nil {
			scoreSum += *result.answerScore
			scoreCount++
		}
		if v, ok := result.metrics["recall@10"].(float64); ok {
			recallSum += v
			recallCount++
		}
	}

	// 汇总指标
	aggregated := make(map[string]any)
	if recallCount > 0 {
		aggregated["recall@10"] = round4(recallSum / recallCount)
	}
	overall := 0.0
	if scoreCount > 0 {
		overall = round4(scoreSum / scoreCount)
		aggregated["answer_score"] = overall
	} else if v, ok := aggregated["recall@10"].(float64); ok {
		overall = v
	}

	if err := s.repo.ReplaceRunResults(run.RunID, completedResults); err != nil {
		s.failRun(run.RunID, "保存评估结果失败: "+err.Error())
		return
	}

	now := time.Now()
	run.Status = runStatusCompleted
	run.CompletedAt = &now
	run.CompletedItems = len(items)
	run.Progress = 100
	run.OverallScore = overall
	run.Metrics = entity.JSON(aggregated)
	if errorCount > 0 {
		run.Message = fmt.Sprintf("评估完成，%d/%d 条执行失败", errorCount, len(items))
	}
	if err := s.repo.UpdateRun(run); err != nil {
		logEvalError("更新运行完成状态失败", err)
		return
	}
	logRunner("评估运行完成",
		zap.String("run_id", run.RunID),
		zap.Float64("overall_score", overall),
		zap.Int("error_count", errorCount),
	)
}

// failRun 将评估运行标记为失败。
func (s *evaluationService) failRun(runID, message string) {
	run, err := s.repo.FindRunByID(runID)
	if err != nil || run == nil {
		return
	}
	now := time.Now()
	run.Status = runStatusFailed
	run.CompletedAt = &now
	run.Message = message
	if err := s.repo.UpdateRun(run); err != nil {
		logEvalError("标记运行失败", err)
	}
}

// toStringSlice 将 JSON 数组转为字符串切片。
func toStringSlice(arr entity.JSONArray) []string {
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
