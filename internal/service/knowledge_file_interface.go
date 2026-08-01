package service

import (
	"errors"
	"io"
	"mime/multipart"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
)

// ErrObjectNotFound 对象不存在错误，用于删除时忽略已不存在的对象
var ErrObjectNotFound = errors.New("object not found")

// UploadedObject 上传成功后返回的对象元数据
type UploadedObject struct {
	Path        string // 文件存储路径
	URL         string // 文件访问URL
	Filename    string // 文件名
	Size        int64  // 文件大小（字节）
	ContentType string // 文件MIME类型
}

// ObjectStorage 对象存储接口，抽象文件上传和删除操作
type ObjectStorage interface {
	// Upload 上传文件到指定目录
	Upload(folder string, file *multipart.FileHeader) (*UploadedObject, error)
	// Delete 删除指定路径的文件
	Delete(path string) error
	// Read 返回对象内容，由调用方负责关闭。
	Read(path string) (io.ReadCloser, error)
	UploadReader(folder, filename, contentType string, reader io.Reader, size int64) (*UploadedObject, error)
}

// FileDownload 是原始文件下载所需的元数据和内容流。
type FileDownload struct {
	Filename    string
	ContentType string
	Size        int64
	Reader      io.ReadCloser
}

// KnowledgeFileService 知识文件服务接口，定义文件的业务操作
type KnowledgeFileService interface {
	// Upload 上传文件到知识库
	Upload(kbID, parentID string, autoIndex bool, file *multipart.FileHeader) (*response.KnowledgeFileResponse, error)
	// Get 获取文件详情
	Get(kbID, fileID string) (*response.KnowledgeFileResponse, error)
	// List 分页获取知识库中的文件列表
	List(kbID string, req *request.KnowledgeFileListRequest) (*response.KnowledgeFileListResponse, error)
	// Delete 删除文件
	Delete(kbID, fileID string) error
	BatchDelete(kbID string, fileIDs []string) (*response.KnowledgeFileBatchDeleteResponse, error)
	CreateFolder(kbID string, req *request.CreateKnowledgeFolderRequest) (*response.KnowledgeFileResponse, error)
	Search(kbID, query string, offset, limit int) (*response.KnowledgeFileListResponse, error)
	Preview(kbID, fileID string) (*response.KnowledgeFilePreviewResponse, error)
	Download(kbID, fileID string) (*FileDownload, error)
}
