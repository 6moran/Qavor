package repository

import (
	"os"
	"testing"
	"time"

	"Qavor/internal/model/entity"

	"github.com/pgvector/pgvector-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openCascadeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	d := os.Getenv("QAVOR_TEST_POSTGRES_DSN")
	if d == "" {
		t.Skip("QAVOR_TEST_POSTGRES_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(d), &gorm.Config{
		// 生产表已手工维护外键（chunks.file_id -> files.file_id），
		// AutoMigrate 自动建外键会因 file_id 非唯一键而失败，这里跳过。
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&entity.KnowledgeBase{}, &entity.KnowledgeFile{}, &entity.KnowledgeChunk{}, &entity.DocumentProcessingJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedCascadeData 为指定知识库播种一条知识库、两条文件、两条分块和两条处理任务记录。
func seedCascadeData(t *testing.T, db *gorm.DB, kbID string) {
	t.Helper()
	now := time.Now()
	base := &entity.KnowledgeBase{KBID: kbID, Name: "级联删除测试库", EmbeddingModelID: 1, ChatModelID: 2}
	base.CreatedAt, base.UpdatedAt = now, now
	if err := db.Create(base).Error; err != nil {
		t.Fatalf("seed base: %v", err)
	}
	for i, fileID := range []string{"cascade-file-1", "cascade-file-2"} {
		f := &entity.KnowledgeFile{FileID: kbID + "-" + fileID, KBID: kbID, Filename: "doc.txt", Status: entity.FileIndexed, Path: "knowledge/" + kbID + "/" + fileID}
		f.CreatedAt, f.UpdatedAt = now, now
		if err := db.Create(f).Error; err != nil {
			t.Fatalf("seed file %d: %v", i, err)
		}
		chunk := &entity.KnowledgeChunk{ChunkID: kbID + "-chunk-" + fileID, FileID: f.FileID, KBID: kbID, ChunkIndex: 0, Content: "content", TokenCount: 3, Embedding: pgvector.NewVector([]float32{1, 2, 3})}
		chunk.CreatedAt, chunk.UpdatedAt = now, now
		if err := db.Create(chunk).Error; err != nil {
			t.Fatalf("seed chunk %d: %v", i, err)
		}
		job := &entity.DocumentProcessingJob{JobID: kbID + "-job-" + fileID, KBID: kbID, FileID: f.FileID, JobType: entity.JobTypeIndex, Status: entity.JobSucceeded, AvailableAt: now}
		job.CreatedAt, job.UpdatedAt = now, now
		if err := db.Create(job).Error; err != nil {
			t.Fatalf("seed job %d: %v", i, err)
		}
	}
}

func cleanupCascadeTest(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Where("kb_id LIKE ?", "kb-cascade-test%").Delete(&entity.KnowledgeChunk{})
	db.Where("kb_id LIKE ?", "kb-cascade-test%").Delete(&entity.KnowledgeFile{})
	db.Where("kb_id LIKE ?", "kb-cascade-test%").Delete(&entity.DocumentProcessingJob{})
	db.Where("kb_id LIKE ?", "kb-cascade-test%").Delete(&entity.KnowledgeBase{})
}

func countByKBID(t *testing.T, db *gorm.DB, model any, kbID string) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Where("kb_id = ?", kbID).Count(&n).Error; err != nil {
		t.Fatalf("count %T by kb_id: %v", model, err)
	}
	return n
}

func TestDeleteByKBID_CascadesAllRelatedTables(t *testing.T) {
	db := openCascadeTestDB(t)
	defer cleanupCascadeTest(t, db)
	kbID := "kb-cascade-test-1"
	seedCascadeData(t, db, kbID)

	repo := &knowledgeBaseRepository{db: db}
	if err := repo.DeleteByKBID(kbID); err != nil {
		t.Fatalf("DeleteByKBID: %v", err)
	}

	var baseCount int64
	db.Model(&entity.KnowledgeBase{}).Where("kb_id = ?", kbID).Count(&baseCount)
	if baseCount != 0 {
		t.Errorf("knowledge_bases left=%d want 0", baseCount)
	}
	if n := countByKBID(t, db, &entity.KnowledgeFile{}, kbID); n != 0 {
		t.Errorf("knowledge_files left=%d want 0", n)
	}
	if n := countByKBID(t, db, &entity.KnowledgeChunk{}, kbID); n != 0 {
		t.Errorf("knowledge_chunks left=%d want 0", n)
	}
	if n := countByKBID(t, db, &entity.DocumentProcessingJob{}, kbID); n != 0 {
		t.Errorf("document_processing_jobs left=%d want 0", n)
	}
}

func TestDeleteByKBID_KeepsOtherKnowledgeBase(t *testing.T) {
	db := openCascadeTestDB(t)
	defer cleanupCascadeTest(t, db)
	kbA := "kb-cascade-test-a"
	kbB := "kb-cascade-test-b"
	seedCascadeData(t, db, kbA)
	seedCascadeData(t, db, kbB)

	repo := &knowledgeBaseRepository{db: db}
	if err := repo.DeleteByKBID(kbA); err != nil {
		t.Fatalf("DeleteByKBID: %v", err)
	}

	// 被删除的知识库清空，另一个知识库的数据不受影响。
	var baseCount int64
	db.Model(&entity.KnowledgeBase{}).Where("kb_id = ?", kbA).Count(&baseCount)
	if baseCount != 0 {
		t.Errorf("deleted kb left=%d want 0", baseCount)
	}
	if n := countByKBID(t, db, &entity.KnowledgeFile{}, kbB); n != 2 {
		t.Errorf("other kb files=%d want 2", n)
	}
	if n := countByKBID(t, db, &entity.KnowledgeChunk{}, kbB); n != 2 {
		t.Errorf("other kb chunks=%d want 2", n)
	}
	if n := countByKBID(t, db, &entity.DocumentProcessingJob{}, kbB); n != 2 {
		t.Errorf("other kb jobs=%d want 2", n)
	}
	var bBase entity.KnowledgeBase
	if err := db.Where("kb_id = ?", kbB).First(&bBase).Error; err != nil {
		t.Fatalf("other kb base missing: %v", err)
	}
}
