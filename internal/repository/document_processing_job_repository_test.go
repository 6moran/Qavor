package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"Qavor/internal/model/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func dsn(t *testing.T) string {
	t.Helper()
	d := os.Getenv("QAVOR_TEST_POSTGRES_DSN")
	if d == "" {
		return ""
	}
	return d
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	d := dsn(t)
	if d == "" {
		t.Skip("QAVOR_TEST_POSTGRES_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(d), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&entity.KnowledgeFile{}, &entity.DocumentProcessingJob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedParsedFile(t *testing.T, db *gorm.DB) *entity.KnowledgeFile {
	t.Helper()
	now := time.Now()
	f := &entity.KnowledgeFile{
		FileID:   "test-file-tx-1",
		KBID:     "test-kb-tx",
		Filename: "guide.txt",
		Status:   entity.FileParsed,
		Path:     "guide.txt",
	}
	f.CreatedAt = now
	f.UpdatedAt = now
	if err := db.Create(f).Error; err != nil {
		t.Fatalf("seed file: %v", err)
	}
	return f
}

func cleanupTx(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Where("kb_id = ?", "test-kb-tx").Delete(&entity.DocumentProcessingJob{})
	db.Where("kb_id = ?", "test-kb-tx").Delete(&entity.KnowledgeFile{})
}

func TestCreateForFileTransition(t *testing.T) {
	db := openTestDB(t)
	defer cleanupTx(t, db)

	f := seedParsedFile(t, db)
	repo := &documentProcessingJobRepository{db: db}
	ctx := context.Background()

	job := &entity.DocumentProcessingJob{
		JobID:       "job-tx-1",
		KBID:        f.KBID,
		FileID:      f.FileID,
		JobType:     entity.JobTypeIndex,
		Status:      entity.JobPending,
		AvailableAt: time.Now(),
	}

	created, err := repo.CreateForFileTransition(ctx, job, []string{entity.FileParsed}, entity.FileIndexQueued)
	if err != nil {
		t.Fatalf("CreateForFileTransition: %v", err)
	}
	if !created {
		t.Fatal("expected created=true on first call")
	}

	// Verify file status changed.
	var updated entity.KnowledgeFile
	db.Where("kb_id = ? AND file_id = ?", f.KBID, f.FileID).First(&updated)
	if updated.Status != entity.FileIndexQueued {
		t.Fatalf("file status=%q want %q", updated.Status, entity.FileIndexQueued)
	}

	// Verify one pending index job exists.
	active, err := repo.FindActiveByFileAndType(ctx, f.KBID, f.FileID, entity.JobTypeIndex)
	if err != nil {
		t.Fatalf("FindActiveByFileAndType: %v", err)
	}
	if active == nil || active.JobID != "job-tx-1" {
		t.Fatal("expected active index job")
	}

	// Second call with a new job ID should return false (active job exists).
	job2 := &entity.DocumentProcessingJob{
		JobID:       "job-tx-2",
		KBID:        f.KBID,
		FileID:      f.FileID,
		JobType:     entity.JobTypeIndex,
		Status:      entity.JobPending,
		AvailableAt: time.Now(),
	}
	created2, err := repo.CreateForFileTransition(ctx, job2, []string{entity.FileIndexQueued}, entity.FileIndexQueued)
	if err != nil {
		t.Fatalf("CreateForFileTransition second: %v", err)
	}
	if created2 {
		t.Fatal("expected created=false on duplicate call")
	}
}
