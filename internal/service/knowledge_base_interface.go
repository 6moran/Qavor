package service

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
)

// KnowledgeBaseService 知识库服务接口，定义知识库的业务操作
type KnowledgeBaseService interface {
	// Create 创建知识库
	Create(req *request.CreateKnowledgeBaseRequest) (*response.KnowledgeBaseResponse, error)
	// Get 根据ID获取知识库详情
	Get(kbID string) (*response.KnowledgeBaseResponse, error)
	// List 分页获取知识库列表
	List(req *request.KnowledgeBaseListRequest) (*response.KnowledgeBaseListResponse, error)
	// Update 更新知识库信息
	Update(kbID string, req *request.UpdateKnowledgeBaseRequest) (*response.KnowledgeBaseResponse, error)
	// Delete 删除知识库
	Delete(kbID string) error
}
