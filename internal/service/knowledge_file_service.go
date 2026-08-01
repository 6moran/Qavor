package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"
	storagepkg "Qavor/pkg/minio"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// knowledgeFileService 知识文件服务实现
type knowledgeFileService struct {
	baseRepo repository.KnowledgeBaseRepository // 知识库仓库
	fileRepo repository.KnowledgeFileRepository // 文件仓库
	jobRepo  repository.DocumentProcessingJobRepository
	storage  ObjectStorage // 对象存储
	queue    documentqueue.DocumentQueue
}

const maxKnowledgeFilePreviewSize = 2 << 20

// NewKnowledgeFileService 创建知识文件服务实例
func NewKnowledgeFileService(baseRepo repository.KnowledgeBaseRepository, fileRepo repository.KnowledgeFileRepository, jobRepo repository.DocumentProcessingJobRepository, storage ObjectStorage, queue documentqueue.DocumentQueue) KnowledgeFileService {
	return &knowledgeFileService{baseRepo: baseRepo, fileRepo: fileRepo, jobRepo: jobRepo, storage: storage, queue: queue}
}

// Upload 上传文件到知识库
func (s *knowledgeFileService) Upload(kbID, parentID string, autoIndex bool, file *multipart.FileHeader) (*response.KnowledgeFileResponse, error) {
	// 参数校验
	if file == nil {
		return nil, bizerrors.New(bizerrors.CodeMissingParam, "缺少上传文件")
	}
	if kbID != "" {
		// 指定知识库时才校验知识库存在。
		base, err := s.baseRepo.FindByKBID(kbID)
		if err != nil {
			return nil, err
		}
		if base == nil {
			return nil, knowledgeBaseNotFoundError()
		}
		if parentID != "" {
			parent, err := s.fileRepo.FindByKBIDAndFileID(kbID, parentID)
			if err != nil {
				return nil, err
			}
			if parent == nil || !parent.IsFolder {
				return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "父文件夹不存在")
			}
		}
	}
	if s.jobRepo != nil && s.queue == nil {
		return nil, bizerrors.New(bizerrors.CodeServiceUnavailable, "文档处理队列暂不可用")
	}
	// 指定知识库的文件,存储到对应的id下
	folder := "knowledge/uploads"
	if kbID != "" {
		folder = fmt.Sprintf("knowledge/%s", kbID)
	}
	// 上传文件到对象存储
	object, err := s.storage.Upload(folder, file)
	if err != nil {
		return nil, err
	}
	fileSize := object.Size

	// 构建文件实体
	knowledgeFile := &entity.KnowledgeFile{
		FileID:           uuid.NewString(), // 生成唯一标识
		KBID:             kbID,
		ParentID:         parentID,
		Filename:         storagepkg.SanitizeFileName(object.Filename), // 清理文件名
		OriginalFilename: object.Filename,
		FileType:         object.ContentType,
		Path:             object.Path,
		MinioURL:         object.URL,
		Status:           "uploaded", // 初始状态为已上传
		FileSize:         &fileSize,
		ContentType:      object.ContentType,
		ProcessingParams: entity.JSON{"auto_index": autoIndex},
	}
	// 保存文件记录到数据库
	if err := s.fileRepo.Create(knowledgeFile); err != nil {
		return nil, err
	}
	result := knowledgeFileResponse(knowledgeFile)
	if !knowledgeFile.IsFolder && s.jobRepo != nil {
		job := &entity.DocumentProcessingJob{JobID: uuid.NewString(), KBID: kbID, FileID: knowledgeFile.FileID, Status: entity.JobPending, MaxAttempts: 1, AvailableAt: time.Now()}
		if err := s.jobRepo.Create(job); err != nil {
			_ = s.fileRepo.DeleteByKBIDAndFileID(kbID, knowledgeFile.FileID)
			_ = s.storage.Delete(object.Path)
			return nil, err
		}
		result.ProcessingJobID = job.JobID
		if err := s.queue.Publish(context.Background(), documentqueue.Message{
			JobID:     job.JobID,
			KBID:      job.KBID,
			FileID:    job.FileID,
			CreatedAt: time.Now(),
			Schema:    1,
		}); err != nil {
			_ = s.jobRepo.MarkFailed(job.JobID, "QUEUE_ENQUEUE_FAILED", "文档处理任务投递失败")
			_ = s.fileRepo.DeleteByKBIDAndFileID(kbID, knowledgeFile.FileID)
			_ = s.storage.Delete(object.Path)
			return nil, bizerrors.NewWithErr(bizerrors.CodeServiceUnavailable, "文档处理队列暂不可用", err)
		}
	}
	return result, nil
}

// Get 获取文件详情
func (s *knowledgeFileService) Get(kbID, fileID string) (*response.KnowledgeFileResponse, error) {
	// 根据知识库ID和文件ID查询文件
	file, err := s.fileRepo.FindByKBIDAndFileID(kbID, fileID)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "文件不存在")
	}
	return knowledgeFileResponse(file), nil
}

// List 分页获取知识库中的文件列表
func (s *knowledgeFileService) List(kbID string, req *request.KnowledgeFileListRequest) (*response.KnowledgeFileListResponse, error) {
	// 参数校验和默认值设置
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	// 处理状态过滤，"all" 表示不过滤
	status := req.Status
	if status == "all" {
		status = ""
	}
	// 查询文件列表
	files, total, err := s.fileRepo.ListByKBID(kbID, (page-1)*pageSize, pageSize, req.ParentID, req.PathPrefix, req.Recursive, status)
	if err != nil {
		return nil, err
	}
	// 转换为响应格式
	items := make([]response.KnowledgeFileResponse, 0, len(files))
	for _, file := range files {
		items = append(items, *knowledgeFileResponse(file))
	}
	return &response.KnowledgeFileListResponse{Total: total, Items: items}, nil
}

// Delete 删除文件
func (s *knowledgeFileService) Delete(kbID, fileID string) error {
	// 查询要删除的文件
	file, err := s.fileRepo.FindByKBIDAndFileID(kbID, fileID)
	if err != nil {
		return err
	}
	if file == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "文件不存在")
	}
	// 删除对象存储中的文件（忽略文件不存在的错误）
	if file.Path != "" {
		if err := s.storage.Delete(file.Path); err != nil && !errors.Is(err, ErrObjectNotFound) {
			return err
		}
	}
	// 删除数据库中的文件记录
	return s.fileRepo.DeleteByKBIDAndFileID(kbID, fileID)
}

// BatchDelete 逐个删除文件；单项失败会返回在结果中，不影响其余文件删除。
func (s *knowledgeFileService) BatchDelete(kbID string, fileIDs []string) (*response.KnowledgeFileBatchDeleteResponse, error) {
	if len(fileIDs) == 0 {
		return nil, bizerrors.New(bizerrors.CodeMissingParam, "缺少文件 ID")
	}
	if len(fileIDs) > 50 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "单次最多删除 50 个文件")
	}
	result := &response.KnowledgeFileBatchDeleteResponse{}
	seen := make(map[string]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" {
			result.FailedItems = append(result.FailedItems, response.KnowledgeFileDeleteFailure{Message: "文件 ID 不能为空"})
			continue
		}
		if _, duplicate := seen[fileID]; duplicate {
			continue
		}
		seen[fileID] = struct{}{}
		if err := s.Delete(kbID, fileID); err != nil {
			result.FailedItems = append(result.FailedItems, response.KnowledgeFileDeleteFailure{FileID: fileID, Message: err.Error()})
			continue
		}
		result.DeletedCount++
	}
	return result, nil
}

// CreateFolder 在指定知识库中创建元数据文件夹。
func (s *knowledgeFileService) CreateFolder(kbID string, req *request.CreateKnowledgeFolderRequest) (*response.KnowledgeFileResponse, error) {
	if req == nil || strings.TrimSpace(req.FolderName) == "" {
		return nil, bizerrors.New(bizerrors.CodeMissingParam, "缺少文件夹名称")
	}
	name := strings.TrimSpace(req.FolderName)
	if strings.ContainsAny(name, "/\\") {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "文件夹名称不能包含路径分隔符")
	}
	base, err := s.baseRepo.FindByKBID(kbID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, knowledgeBaseNotFoundError()
	}
	if req.ParentID != "" {
		parent, err := s.fileRepo.FindByKBIDAndFileID(kbID, req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || !parent.IsFolder {
			return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "父文件夹不存在")
		}
	}
	folder := &entity.KnowledgeFile{
		FileID:           uuid.NewString(),
		KBID:             kbID,
		ParentID:         req.ParentID,
		Filename:         name,
		OriginalFilename: name,
		FileType:         "folder",
		ContentType:      "inode/directory",
		Status:           "completed",
		IsFolder:         true,
	}
	if err := s.fileRepo.Create(folder); err != nil {
		return nil, err
	}
	return knowledgeFileResponse(folder), nil
}

// Search 在指定知识库的文件管理列表中按名称检索。
func (s *knowledgeFileService) Search(kbID, query string, offset, limit int) (*response.KnowledgeFileListResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, bizerrors.New(bizerrors.CodeMissingParam, "缺少搜索关键词")
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	files, total, err := s.fileRepo.SearchByKBID(kbID, query, offset, limit)
	if err != nil {
		return nil, err
	}
	items := make([]response.KnowledgeFileResponse, 0, len(files))
	for _, file := range files {
		items = append(items, *knowledgeFileResponse(file))
	}
	return &response.KnowledgeFileListResponse{Total: total, Items: items}, nil
}

// Preview 读取解析后的 Markdown（存在时）或原始文件的有限文本内容。
func (s *knowledgeFileService) Preview(kbID, fileID string) (*response.KnowledgeFilePreviewResponse, error) {
	file, err := s.fileRepo.FindByKBIDAndFileID(kbID, fileID)
	if err != nil {
		return nil, err
	}
	if file == nil || file.IsFolder {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "文件不存在")
	}
	path := file.MarkdownFile
	if path == "" {
		path = file.Path
	}
	if path == "" {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "文件内容不存在")
	}
	if file.MarkdownFile == "" && !isTextPreviewable(file) {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "该文件类型不支持文本预览")
	}
	if file.FileSize != nil && *file.FileSize > maxKnowledgeFilePreviewSize && file.MarkdownFile == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "文件过大，无法文本预览")
	}
	reader, err := s.storage.Read(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	content, err := io.ReadAll(io.LimitReader(reader, maxKnowledgeFilePreviewSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxKnowledgeFilePreviewSize {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "文件过大，无法文本预览")
	}
	return &response.KnowledgeFilePreviewResponse{Content: string(content)}, nil
}

// Download 返回知识库中原始文件的受控读取流。
func (s *knowledgeFileService) Download(kbID, fileID string) (*FileDownload, error) {
	file, err := s.fileRepo.FindByKBIDAndFileID(kbID, fileID)
	if err != nil {
		return nil, err
	}
	if file == nil || file.IsFolder || file.Path == "" {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "文件不存在")
	}
	reader, err := s.storage.Read(file.Path)
	if err != nil {
		return nil, err
	}
	size := int64(-1)
	if file.FileSize != nil {
		size = *file.FileSize
	}
	return &FileDownload{Filename: file.OriginalFilename, ContentType: file.ContentType, Size: size, Reader: reader}, nil
}

func isTextPreviewable(file *entity.KnowledgeFile) bool {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(file.ContentType, ";")[0]))
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	switch contentType {
	case "application/json", "application/xml", "application/javascript", "application/x-javascript", "application/yaml", "application/x-yaml":
		return true
	}
	switch strings.ToLower(filepath.Ext(file.Filename)) {
	case ".md", ".txt", ".csv", ".json", ".xml", ".yaml", ".yml", ".html", ".htm", ".css", ".js", ".ts", ".vue", ".go", ".py", ".java", ".sql", ".sh", ".log":
		return true
	default:
		return false
	}
}

// knowledgeFileResponse 将文件实体转换为响应格式
func knowledgeFileResponse(file *entity.KnowledgeFile) *response.KnowledgeFileResponse {
	return &response.KnowledgeFileResponse{
		ID:               file.ID,
		FileID:           file.FileID,
		KBID:             file.KBID,
		ParentID:         file.ParentID,
		Filename:         file.Filename,
		OriginalFilename: file.OriginalFilename,
		FileType:         file.FileType,
		Path:             file.Path,
		MinioURL:         file.MinioURL,
		MarkdownFile:     file.MarkdownFile,
		Status:           file.Status,
		ContentHash:      file.ContentHash,
		FileSize:         file.FileSize,
		ChunkCount:       file.ChunkCount,
		TokenCount:       file.TokenCount,
		ContentType:      file.ContentType,
		IsFolder:         file.IsFolder,
		ErrorMessage:     file.ErrorMessage,
		CreatedAt:        file.CreatedAt,
		UpdatedAt:        file.UpdatedAt,
	}
}

// minIOObjectStorage MinIO 对象存储实现
type minIOObjectStorage struct{}

// NewMinIOObjectStorage 创建 MinIO 对象存储服务实例
func NewMinIOObjectStorage() ObjectStorage {
	return minIOObjectStorage{}
}

// Upload 上传文件到 MinIO
func (minIOObjectStorage) Upload(folder string, file *multipart.FileHeader) (*UploadedObject, error) {
	// 调用 MinIO 客户端上传文件
	result, err := storagepkg.Get().Upload(folder, file)
	if err != nil {
		return nil, err
	}
	// 返回上传结果
	return &UploadedObject{
		Path:        result.RelativePath,
		URL:         result.FullURL,
		Filename:    result.FileName,
		Size:        result.FileSize,
		ContentType: result.ContentType,
	}, nil
}

// Delete 从 MinIO 删除文件
func (minIOObjectStorage) Delete(path string) error {
	client := storagepkg.Get()
	// 检查文件是否存在
	exists, err := client.Exists(path)
	if err != nil {
		return err
	}
	if !exists {
		return ErrObjectNotFound
	}
	// 执行删除
	return client.Delete(path)
}

// Read 从 MinIO 打开对象内容流。
func (minIOObjectStorage) Read(path string) (io.ReadCloser, error) {
	return storagepkg.Get().Client().GetObject(context.Background(), storagepkg.Get().Config().Bucket, path, minio.GetObjectOptions{})
}

func (minIOObjectStorage) UploadReader(folder, filename, contentType string, reader io.Reader, size int64) (*UploadedObject, error) {
	result, err := storagepkg.Get().UploadFromReader(folder, filename, contentType, reader, size)
	if err != nil {
		return nil, err
	}
	return &UploadedObject{Path: result.RelativePath, URL: result.FullURL, Filename: result.FileName, Size: result.FileSize, ContentType: result.ContentType}, nil
}
