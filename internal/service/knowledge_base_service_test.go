package service

import (
	"context"
	"testing"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
)

// deleteKBRepo 记录知识库删除调用。
type deleteKBRepo struct {
	repository.KnowledgeBaseRepository
	found       bool
	deleted     bool
	deleteErr   error
	deletedKBID string
}

func (f *deleteKBRepo) FindByKBID(kbID string) (*entity.KnowledgeBase, error) {
	if !f.found {
		return nil, nil
	}
	return &entity.KnowledgeBase{KBID: kbID}, nil
}

func (f *deleteKBRepo) DeleteByKBID(kbID string) error {
	f.deleted = true
	f.deletedKBID = kbID
	return f.deleteErr
}

// deleteFileRepo 提供 ListAllByKBID 的最小实现，避免 nil 接口在 Delete 中 panic。
type deleteFileRepo struct {
	repository.KnowledgeFileRepository
	files []*entity.KnowledgeFile
}

func (r *deleteFileRepo) ListAllByKBID(_ context.Context, _ string) ([]*entity.KnowledgeFile, error) {
	return r.files, nil
}

func TestDeleteKnowledgeBase_NotFound(t *testing.T) {
	kbRepo := &deleteKBRepo{found: false}
	fileRepo := &deleteFileRepo{}
	svc := NewKnowledgeBaseService(kbRepo, nil, fileRepo, nil, nil)

	err := svc.Delete("kb-missing")
	if err == nil {
		t.Fatal("expected error for missing knowledge base")
	}
	if kbRepo.deleted {
		t.Fatal("must not delete when knowledge base is missing")
	}
}

func TestDeleteKnowledgeBase_Deletes(t *testing.T) {
	kbRepo := &deleteKBRepo{found: true}
	fileRepo := &deleteFileRepo{}
	svc := NewKnowledgeBaseService(kbRepo, nil, fileRepo, nil, nil)

	if err := svc.Delete("kb-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !kbRepo.deleted || kbRepo.deletedKBID != "kb-1" {
		t.Fatalf("expected delete of kb-1, deleted=%v kbID=%q", kbRepo.deleted, kbRepo.deletedKBID)
	}
}

// unbindAgentRepo 记录知识库删除后联动解绑的调用。
type unbindAgentRepo struct {
	repository.AgentRepository
	calledKBID string
	affected   int64
}

func (r *unbindAgentRepo) UnbindKnowledge(_ context.Context, kbID string) (int64, error) {
	r.calledKBID = kbID
	r.affected = 3
	return r.affected, nil
}

// TestDeleteKnowledgeBase_UnbindsAgents 验证删除知识库后会联动解除 Agent 绑定。
func TestDeleteKnowledgeBase_UnbindsAgents(t *testing.T) {
	kbRepo := &deleteKBRepo{found: true}
	agentRepo := &unbindAgentRepo{}
	fileRepo := &deleteFileRepo{}
	svc := NewKnowledgeBaseService(kbRepo, nil, fileRepo, nil, agentRepo)

	if err := svc.Delete("kb-42"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if agentRepo.calledKBID != "kb-42" {
		t.Fatalf("UnbindKnowledge called with %q, want kb-42", agentRepo.calledKBID)
	}
	if agentRepo.affected != 3 {
		t.Fatalf("UnbindKnowledge affected = %d, want 3", agentRepo.affected)
	}
}
