package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const mindmapMaxInputChars = 120000

// mindmapChatTimeout 思维导图生成调用 Chat 模型的宽松超时下限。
// 导图 prompt 可拼接整个知识库（最多 12 万字符）且要求模型输出整棵 JSON 树，
// 推理耗时远超默认 60s 客户端超时；模型自身超时配置取较大值。
const mindmapChatTimeout = 5 * time.Minute

// MindmapChatResolver 是导图服务所需的最小模型能力接口。
type MindmapChatResolver interface {
	ResolveChatModel(ctx context.Context, modelID uint) (model.ToolCallingChatModel, error)
	ResolveChatModelWithTimeout(ctx context.Context, modelID uint, minTimeout time.Duration) (model.ToolCallingChatModel, error)
}

type mindmapService struct {
	kbRepo    repository.KnowledgeBaseRepository
	fileRepo  repository.KnowledgeFileRepository
	chunkRepo repository.KnowledgeChunkRepository
	modelSvc  MindmapChatResolver

	// taskMu 保护 tasks 表；tasks 记录每个知识库的后台生成任务状态（进程内）。
	taskMu sync.Mutex
	tasks  map[string]*mindmapTask
}

// mindmapTask 记录单个知识库后台生成任务的状态。
type mindmapTask struct {
	mu       sync.Mutex
	running  bool
	errorMsg string
}

// NewMindmapService 创建知识导图服务。
func NewMindmapService(
	kbRepo repository.KnowledgeBaseRepository,
	fileRepo repository.KnowledgeFileRepository,
	chunkRepo repository.KnowledgeChunkRepository,
	modelSvc MindmapChatResolver,
) MindmapService {
	return &mindmapService{
		kbRepo:    kbRepo,
		fileRepo:  fileRepo,
		chunkRepo: chunkRepo,
		modelSvc:  modelSvc,
		tasks:     make(map[string]*mindmapTask),
	}
}

// ListDatabases 返回至少存在已入库文件的知识库列表。
func (s *mindmapService) ListDatabases(ctx context.Context) ([]MindmapDatabaseDTO, error) {
	// 列表接口没有分页需求，使用较大的上限并复用现有仓储接口。
	bases, _, err := s.kbRepo.List(0, 1000, "")
	if err != nil {
		return nil, err
	}
	result := make([]MindmapDatabaseDTO, 0, len(bases))
	for _, base := range bases {
		files, err := s.fileRepo.ListByKBIDAndStatuses(ctx, base.KBID, []string{entity.FileIndexed}, 1)
		if err != nil {
			return nil, err
		}
		if len(files) > 0 {
			result = append(result, MindmapDatabaseDTO{KBID: base.KBID, Name: base.Name})
		}
	}
	return result, nil
}

// ListFiles 返回知识库中可以参与导图生成的文件。
func (s *mindmapService) ListFiles(ctx context.Context, kbID string) ([]MindmapFileDTO, error) {
	if _, err := s.requireKB(kbID); err != nil {
		return nil, err
	}
	files, err := s.fileRepo.ListByKBIDAndStatuses(ctx, kbID, []string{entity.FileIndexed}, 1000)
	if err != nil {
		return nil, err
	}
	result := make([]MindmapFileDTO, 0, len(files))
	for _, file := range files {
		if file.IsFolder {
			continue
		}
		result = append(result, MindmapFileDTO{FileID: file.FileID, Filename: file.Filename, Status: file.Status})
	}
	return result, nil
}

// Get 返回知识库当前保存的导图。
func (s *mindmapService) Get(ctx context.Context, kbID string) (*MindmapDTO, error) {
	base, err := s.requireKB(kbID)
	if err != nil {
		return nil, err
	}
	result := &MindmapDTO{FileIDs: toStringIDs(base.MindmapFileIDs)}
	if base.Mindmap != nil && len(base.Mindmap) > 0 {
		content, err := json.Marshal(base.Mindmap)
		if err != nil {
			return nil, err
		}
		var node MindmapNode
		if err := json.Unmarshal(content, &node); err != nil {
			return nil, fmt.Errorf("解析已保存知识导图失败: %w", err)
		}
		result.Mindmap = &node
		result.HasMindmap = true
	}
	if generatedAt, ok := base.MindmapMetadata["generated_at"].(string); ok {
		result.GeneratedAt = generatedAt
	}
	result.FileCount = len(result.FileIDs)
	if task := s.getTask(kbID); task != nil {
		task.mu.Lock()
		result.Generating = task.running
		result.GenerateError = task.errorMsg
		task.mu.Unlock()
	}
	return result, nil
}

// GetDiff 返回当前已入库文件与导图来源文件之间的差异。
func (s *mindmapService) GetDiff(ctx context.Context, kbID string) (*MindmapDiffDTO, error) {
	base, err := s.requireKB(kbID)
	if err != nil {
		return nil, err
	}
	files, err := s.fileRepo.ListByKBIDAndStatuses(ctx, kbID, []string{entity.FileIndexed}, 1000)
	if err != nil {
		return nil, err
	}
	current := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsFolder {
			current = append(current, file.FileID)
		}
	}
	result := diffMindmapFileIDs(toStringIDs(base.MindmapFileIDs), current)
	return &MindmapDiffDTO{AddedFiles: result.AddedFiles, RemovedFiles: result.RemovedFiles, NeedsUpdate: result.NeedsUpdate}, nil
}

// Generate 校验请求后立即返回,后台异步调用模型生成并保存导图。
// 这样长耗时的模型调用不会占用 HTTP 连接,前端通过轮询 Get 感知完成状态。
func (s *mindmapService) Generate(ctx context.Context, kbID string, req *GenerateMindmapRequest) (*GenerateMindmapResponse, error) {
	if req == nil {
		req = &GenerateMindmapRequest{}
	}
	base, err := s.requireKB(kbID)
	if err != nil {
		return nil, err
	}
	if base.ChatModelID == 0 {
		return nil, errors.New("知识库未配置 Chat 模型")
	}
	files, err := s.selectFiles(ctx, kbID, req.FileIDs)
	if err != nil {
		return nil, err
	}
	fileIDs := make([]string, 0, len(files))
	for _, file := range files {
		fileIDs = append(fileIDs, file.FileID)
	}
	if len(fileIDs) == 0 {
		return nil, errors.New("知识库没有可生成导图的已入库文件")
	}
	// 增量更新且文件集合无变化:无需调用 AI,直接返回已保存导图。
	if req.Incremental && base.Mindmap != nil && len(base.Mindmap) > 0 && sameStringSet(toStringIDs(base.MindmapFileIDs), fileIDs) {
		current, getErr := s.Get(ctx, kbID)
		if getErr != nil {
			return nil, getErr
		}
		return &GenerateMindmapResponse{Mindmap: current.Mindmap, FileIDs: fileIDs, Incremental: true, NoAINeeded: true}, nil
	}

	// 同一知识库已有生成任务在跑:不重复触发,直接返回进行中状态。
	if !s.startTask(kbID) {
		return &GenerateMindmapResponse{FileIDs: fileIDs, Incremental: req.Incremental, Generating: true}, nil
	}

	// 后台异步生成,接口立即返回。
	go s.generateInBackground(kbID, req, files, fileIDs, base.ChatModelID)
	return &GenerateMindmapResponse{FileIDs: fileIDs, Incremental: req.Incremental, Generating: true}, nil
}

// generateInBackground 在后台执行导图生成,模型调用、JSON 解析和落库完成后更新任务状态。
// 注意:不使用请求的 context（请求结束后会被取消）,统一使用 context.Background()。
func (s *mindmapService) generateInBackground(kbID string, req *GenerateMindmapRequest, files []*entity.KnowledgeFile, fileIDs []string, chatModelID uint) {
	ctx := context.Background()
	runErr := s.runGenerate(ctx, kbID, req, files, fileIDs, chatModelID)
	s.finishTask(kbID, runErr)
}

// runGenerate 执行导图生成的核心逻辑（同步版 Generate 的实体部分）。
func (s *mindmapService) runGenerate(ctx context.Context, kbID string, req *GenerateMindmapRequest, files []*entity.KnowledgeFile, fileIDs []string, chatModelID uint) error {
	base, err := s.requireKB(kbID)
	if err != nil {
		return err
	}
	source, err := s.buildMindmapSource(ctx, kbID, files)
	if err != nil {
		return err
	}
	chat, err := s.modelSvc.ResolveChatModelWithTimeout(ctx, chatModelID, mindmapChatTimeout)
	if err != nil {
		return fmt.Errorf("解析知识导图 Chat 模型失败: %w", err)
	}
	prompt := buildMindmapPrompt(source, req.UserPrompt)
	reply, err := chat.Generate(ctx, []*schema.Message{
		schema.SystemMessage("你是一个严谨的知识结构化助手，只能根据提供的知识片段生成思维导图。"),
		schema.UserMessage(prompt),
	})
	if err != nil {
		return fmt.Errorf("生成知识导图失败: %w", err)
	}
	if reply == nil {
		return errors.New("生成知识导图失败: 模型返回为空")
	}
	node, err := parseMindmapJSON(reply.Content)
	if err != nil {
		return fmt.Errorf("解析知识导图失败: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	base.Mindmap = entity.JSON{"content": node.Content, "children": node.Children}
	base.MindmapFileIDs = stringIDsToJSONArray(fileIDs)
	base.MindmapMetadata = entity.JSON{"generated_at": now, "file_count": len(fileIDs)}
	if err := s.kbRepo.Update(base); err != nil {
		return fmt.Errorf("保存知识导图失败: %w", err)
	}
	return nil
}

// getTask 返回知识库对应的任务状态（不存在时返回 nil）。
func (s *mindmapService) getTask(kbID string) *mindmapTask {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	return s.tasks[kbID]
}

// startTask 尝试抢占知识库的后台生成任务。
// 返回 false 表示该知识库已有任务在运行，调用方不应再触发新的生成。
func (s *mindmapService) startTask(kbID string) bool {
	s.taskMu.Lock()
	task := s.tasks[kbID]
	if task == nil {
		task = &mindmapTask{}
		s.tasks[kbID] = task
	}
	s.taskMu.Unlock()

	task.mu.Lock()
	defer task.mu.Unlock()
	if task.running {
		return false
	}
	task.running = true
	task.errorMsg = ""
	return true
}

// finishTask 结束任务并记录错误信息（成功时 err 为 nil）。
func (s *mindmapService) finishTask(kbID string, err error) {
	task := s.getTask(kbID)
	if task == nil {
		return
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.running = false
	if err != nil {
		task.errorMsg = err.Error()
	}
}

func (s *mindmapService) requireKB(kbID string) (*entity.KnowledgeBase, error) {
	if strings.TrimSpace(kbID) == "" {
		return nil, errors.New("知识库 ID 不能为空")
	}
	base, err := s.kbRepo.FindByKBID(kbID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, errors.New("知识库不存在")
	}
	return base, nil
}

func (s *mindmapService) selectFiles(ctx context.Context, kbID string, requested []string) ([]*entity.KnowledgeFile, error) {
	if len(requested) == 0 {
		return s.fileRepo.ListByKBIDAndStatuses(ctx, kbID, []string{entity.FileIndexed}, 1000)
	}
	files := make([]*entity.KnowledgeFile, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, fileID := range requested {
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		file, err := s.fileRepo.FindByKBIDAndFileID(kbID, fileID)
		if err != nil {
			return nil, err
		}
		if file == nil || file.Status != entity.FileIndexed || file.IsFolder {
			return nil, fmt.Errorf("文件不存在或尚未入库: %s", fileID)
		}
		files = append(files, file)
	}
	return files, nil
}

func (s *mindmapService) buildMindmapSource(ctx context.Context, kbID string, files []*entity.KnowledgeFile) (string, error) {
	var b strings.Builder
	for _, file := range files {
		chunks, err := s.chunkRepo.FindByFileID(ctx, kbID, file.FileID)
		if err != nil {
			return "", fmt.Errorf("读取文件分块失败: %w", err)
		}
		b.WriteString("\n## 文件：")
		b.WriteString(file.Filename)
		b.WriteString("\n")
		for _, chunk := range chunks {
			if b.Len() >= mindmapMaxInputChars {
				break
			}
			b.WriteString(chunk.Content)
			b.WriteString("\n")
		}
		if b.Len() >= mindmapMaxInputChars {
			break
		}
	}
	return b.String(), nil
}

func buildMindmapPrompt(source, userPrompt string) string {
	var b strings.Builder
	b.WriteString("请把下面的知识内容整理为层级清晰的思维导图。只输出 JSON，不要输出 Markdown、解释文字或代码块。")
	b.WriteString("JSON 格式必须是 {\"content\":\"根主题\",\"children\":[{\"content\":\"子主题\",\"children\":[]}]}。")
	b.WriteString("节点内容要简洁，最多使用四层，不能凭空添加资料中没有的事实。")
	if strings.TrimSpace(userPrompt) != "" {
		b.WriteString("补充要求：")
		b.WriteString(strings.TrimSpace(userPrompt))
	}
	b.WriteString("\n知识内容：\n")
	b.WriteString(source)
	return b.String()
}

// parseMindmapJSON 兼容模型返回裸 JSON、代码块和夹带说明文字的情况。
func parseMindmapJSON(content string) (*MindmapNode, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, errors.New("模型输出中未找到 JSON 对象")
	}
	var node MindmapNode
	if err := json.Unmarshal([]byte(content[start:end+1]), &node); err != nil {
		return nil, err
	}
	if strings.TrimSpace(node.Content) == "" {
		return nil, errors.New("导图根节点内容为空")
	}
	return &node, nil
}

type mindmapFileDiff struct {
	AddedFiles   []string
	RemovedFiles []string
	NeedsUpdate  bool
}

func diffMindmapFileIDs(oldIDs, currentIDs []string) mindmapFileDiff {
	oldSet := make(map[string]struct{}, len(oldIDs))
	currentSet := make(map[string]struct{}, len(currentIDs))
	for _, id := range oldIDs {
		oldSet[id] = struct{}{}
	}
	for _, id := range currentIDs {
		currentSet[id] = struct{}{}
	}
	result := mindmapFileDiff{AddedFiles: []string{}, RemovedFiles: []string{}}
	for id := range currentSet {
		if _, ok := oldSet[id]; !ok {
			result.AddedFiles = append(result.AddedFiles, id)
		}
	}
	for id := range oldSet {
		if _, ok := currentSet[id]; !ok {
			result.RemovedFiles = append(result.RemovedFiles, id)
		}
	}
	sort.Strings(result.AddedFiles)
	sort.Strings(result.RemovedFiles)
	result.NeedsUpdate = len(result.AddedFiles) > 0 || len(result.RemovedFiles) > 0
	return result
}

func sameStringSet(left, right []string) bool {
	diff := diffMindmapFileIDs(left, right)
	return !diff.NeedsUpdate
}

func toStringIDs(values entity.JSONArray) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if id, ok := value.(string); ok && id != "" {
			result = append(result, id)
		}
	}
	return result
}

func stringIDsToJSONArray(values []string) entity.JSONArray {
	result := make(entity.JSONArray, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}
