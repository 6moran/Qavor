package service

import (
	"errors"
	"fmt"
	"mime/multipart"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"
	storagepkg "Qavor/pkg/minio"

	"github.com/google/uuid"
)

// knowledgeFileService 知识文件服务实现
type knowledgeFileService struct {
	baseRepo repository.KnowledgeBaseRepository // 知识库仓库
	fileRepo repository.KnowledgeFileRepository // 文件仓库
	storage  ObjectStorage                      // 对象存储
}

// NewKnowledgeFileService 创建知识文件服务实例
func NewKnowledgeFileService(baseRepo repository.KnowledgeBaseRepository, fileRepo repository.KnowledgeFileRepository, storage ObjectStorage) KnowledgeFileService {
	return &knowledgeFileService{baseRepo: baseRepo, fileRepo: fileRepo, storage: storage}
}

// Upload 上传文件到知识库
func (s *knowledgeFileService) Upload(kbID, createdBy string, file *multipart.FileHeader) (*response.KnowledgeFileResponse, error) {
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
		Filename:         storagepkg.SanitizeFileName(object.Filename), // 清理文件名
		OriginalFilename: object.Filename,
		FileType:         object.ContentType,
		Path:             object.Path,
		MinioURL:         object.URL,
		Status:           "uploaded", // 初始状态为已上传
		FileSize:         &fileSize,
		ContentType:      object.ContentType,
		CreatedBy:        createdBy,
		UpdatedBy:        createdBy,
	}
	// 保存文件记录到数据库
	if err := s.fileRepo.Create(knowledgeFile); err != nil {
		return nil, err
	}
	return knowledgeFileResponse(knowledgeFile), nil
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
		CreatedBy:        file.CreatedBy,
		UpdatedBy:        file.UpdatedBy,
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
