package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	qerrors "Qavor/pkg/errors"
)

type ragSettingsModelRepository interface {
	FindByID(id uint) (*entity.Model, error)
}

type ragSettingsService struct {
	settingsRepo repository.SystemSettingRepository
	modelRepo    ragSettingsModelRepository

	mu          sync.Mutex
	cacheLoaded bool
	cached      RAGSettings
}

// NewRAGSettingsService 创建全局 RAG 设置服务。
func NewRAGSettingsService(settingsRepo repository.SystemSettingRepository, modelRepo ragSettingsModelRepository) RAGSettingsService {
	return &ragSettingsService{settingsRepo: settingsRepo, modelRepo: modelRepo}
}

// Get 读取设置并按需填充进程内缓存。
func (s *ragSettingsService) Get(ctx context.Context) (*RAGSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheLoaded {
		return cloneRAGSettings(s.cached), nil
	}

	value, found, err := s.settingsRepo.Get(ctx, SettingKeyRAGRerankModelID)
	if err != nil {
		return nil, fmt.Errorf("读取全局 Rerank 设置: %w", err)
	}
	value = strings.TrimSpace(value)
	if !found || value == "" {
		s.cached = RAGSettings{}
		s.cacheLoaded = true
		return cloneRAGSettings(s.cached), nil
	}

	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed == 0 {
		return nil, qerrors.New(qerrors.CodeInvalidParam, "全局 Rerank 模型配置无效")
	}
	model, err := s.validRerankModel(uint(parsed))
	if err != nil {
		return nil, err
	}
	s.cached = settingsFromModel(model)
	s.cacheLoaded = true
	return cloneRAGSettings(s.cached), nil
}

// UpdateRerankModel 校验并更新全局重排模型。
func (s *ragSettingsService) UpdateRerankModel(ctx context.Context, modelID *uint) (*RAGSettings, error) {
	var next RAGSettings
	value := ""
	if modelID != nil {
		if *modelID == 0 {
			return nil, qerrors.New(qerrors.CodeInvalidParam, "Rerank 模型 ID 无效")
		}
		model, err := s.validRerankModel(*modelID)
		if err != nil {
			return nil, err
		}
		next = settingsFromModel(model)
		value = strconv.FormatUint(uint64(*modelID), 10)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.settingsRepo.Upsert(ctx, SettingKeyRAGRerankModelID, value); err != nil {
		return nil, fmt.Errorf("更新全局 Rerank 设置: %w", err)
	}
	s.cached = next
	s.cacheLoaded = true
	return cloneRAGSettings(s.cached), nil
}

// RerankModelID 返回运行时可用的全局重排模型 ID。
func (s *ragSettingsService) RerankModelID(ctx context.Context) (uint, bool, error) {
	settings, err := s.Get(ctx)
	if err != nil {
		return 0, false, err
	}
	if settings.RerankModelID == nil {
		return 0, false, nil
	}
	return *settings.RerankModelID, true, nil
}

func (s *ragSettingsService) validRerankModel(id uint) (*entity.Model, error) {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("读取 Rerank 模型: %w", err)
	}
	if model == nil {
		return nil, qerrors.New(qerrors.CodeInvalidParam, "Rerank 模型不存在")
	}
	if !model.Enabled {
		return nil, qerrors.New(qerrors.CodeInvalidParam, "Rerank 模型未启用")
	}
	if model.ModelType != "rerank" {
		return nil, qerrors.New(qerrors.CodeInvalidParam, "模型类型不是 rerank")
	}
	return model, nil
}

func settingsFromModel(model *entity.Model) RAGSettings {
	id := model.ID
	return RAGSettings{RerankModelID: &id, RerankModelName: model.Name}
}

func cloneRAGSettings(settings RAGSettings) *RAGSettings {
	cloned := settings
	if settings.RerankModelID != nil {
		id := *settings.RerankModelID
		cloned.RerankModelID = &id
	}
	return &cloned
}
