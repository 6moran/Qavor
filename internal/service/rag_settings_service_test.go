package service

import (
	"context"
	"errors"
	"testing"

	"Qavor/internal/model/entity"
)

type fakeSystemSettingRepository struct {
	values    map[string]string
	getCalls  int
	upsertErr error
}

func (r *fakeSystemSettingRepository) Get(_ context.Context, key string) (string, bool, error) {
	r.getCalls++
	value, found := r.values[key]
	return value, found, nil
}

func (r *fakeSystemSettingRepository) Upsert(_ context.Context, key, value string) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.values[key] = value
	return nil
}

type fakeRAGSettingsModelRepository struct {
	models map[uint]*entity.Model
}

func (r *fakeRAGSettingsModelRepository) FindByID(id uint) (*entity.Model, error) {
	return r.models[id], nil
}

func testRerankModel(id uint, name string) *entity.Model {
	return &entity.Model{BaseEntity: entity.BaseEntity{ID: id}, Name: name, Enabled: true, ModelType: "rerank"}
}

func uintPointer(value uint) *uint {
	return &value
}

func TestRAGSettingsService_UpdateAcceptsOnlyEnabledRerankModel(t *testing.T) {
	tests := []struct {
		name    string
		modelID uint
		model   *entity.Model
		wantErr bool
	}{
		{name: "启用的重排模型", modelID: 7, model: testRerankModel(7, "bge-reranker-v2-m3")},
		{name: "模型不存在", modelID: 8, wantErr: true},
		{name: "模型已禁用", modelID: 9, model: &entity.Model{BaseEntity: entity.BaseEntity{ID: 9}, Enabled: false, ModelType: "rerank"}, wantErr: true},
		{name: "聊天模型", modelID: 10, model: &entity.Model{BaseEntity: entity.BaseEntity{ID: 10}, Enabled: true, ModelType: "chat"}, wantErr: true},
		{name: "向量模型", modelID: 11, model: &entity.Model{BaseEntity: entity.BaseEntity{ID: 11}, Enabled: true, ModelType: "embedding"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settingsRepo := &fakeSystemSettingRepository{values: map[string]string{}}
			modelRepo := &fakeRAGSettingsModelRepository{models: map[uint]*entity.Model{}}
			if tt.model != nil {
				modelRepo.models[tt.modelID] = tt.model
			}
			svc := NewRAGSettingsService(settingsRepo, modelRepo)
			settings, err := svc.UpdateRerankModel(context.Background(), uintPointer(tt.modelID))
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回业务错误")
				}
				if _, found := settingsRepo.values[SettingKeyRAGRerankModelID]; found {
					t.Fatal("非法模型不应写入设置")
				}
				return
			}
			if err != nil {
				t.Fatalf("更新合法模型: %v", err)
			}
			if settings.RerankModelID == nil || *settings.RerankModelID != tt.modelID || settings.RerankModelName != tt.model.Name {
				t.Fatalf("更新结果=%+v", settings)
			}
			if got := settingsRepo.values[SettingKeyRAGRerankModelID]; got != "7" {
				t.Fatalf("持久化值=%q，期望 7", got)
			}
		})
	}
}

func TestRAGSettingsService_GetCachesAndUpdateInvalidatesImmediately(t *testing.T) {
	settingsRepo := &fakeSystemSettingRepository{values: map[string]string{SettingKeyRAGRerankModelID: "7"}}
	modelRepo := &fakeRAGSettingsModelRepository{models: map[uint]*entity.Model{
		7: testRerankModel(7, "旧模型"),
		8: testRerankModel(8, "新模型"),
	}}
	svc := NewRAGSettingsService(settingsRepo, modelRepo)

	first, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("首次读取: %v", err)
	}
	second, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("缓存读取: %v", err)
	}
	if settingsRepo.getCalls != 1 || first.RerankModelName != "旧模型" || second.RerankModelName != "旧模型" {
		t.Fatalf("缓存行为异常 calls=%d first=%+v second=%+v", settingsRepo.getCalls, first, second)
	}

	updated, err := svc.UpdateRerankModel(context.Background(), uintPointer(8))
	if err != nil {
		t.Fatalf("更新模型: %v", err)
	}
	after, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("更新后读取: %v", err)
	}
	if updated.RerankModelName != "新模型" || after.RerankModelName != "新模型" || settingsRepo.getCalls != 1 {
		t.Fatalf("更新未立即刷新缓存 updated=%+v after=%+v calls=%d", updated, after, settingsRepo.getCalls)
	}
}

func TestRAGSettingsService_ClearAndFailedWriteKeepsPreviousCache(t *testing.T) {
	settingsRepo := &fakeSystemSettingRepository{values: map[string]string{SettingKeyRAGRerankModelID: "7"}}
	modelRepo := &fakeRAGSettingsModelRepository{models: map[uint]*entity.Model{7: testRerankModel(7, "保留模型")}}
	svc := NewRAGSettingsService(settingsRepo, modelRepo)
	if _, err := svc.Get(context.Background()); err != nil {
		t.Fatalf("预热缓存: %v", err)
	}

	settingsRepo.upsertErr = errors.New("写入失败")
	if _, err := svc.UpdateRerankModel(context.Background(), nil); err == nil {
		t.Fatal("期望清空写入失败")
	}
	settingsRepo.upsertErr = nil
	kept, err := svc.Get(context.Background())
	if err != nil || kept.RerankModelID == nil || *kept.RerankModelID != 7 {
		t.Fatalf("失败写入后缓存未保留 settings=%+v err=%v", kept, err)
	}

	cleared, err := svc.UpdateRerankModel(context.Background(), nil)
	if err != nil {
		t.Fatalf("清空设置: %v", err)
	}
	if cleared.RerankModelID != nil || cleared.RerankModelName != "" || settingsRepo.values[SettingKeyRAGRerankModelID] != "" {
		t.Fatalf("清空结果=%+v value=%q", cleared, settingsRepo.values[SettingKeyRAGRerankModelID])
	}
}
