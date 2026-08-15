package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"Qavor/internal/model/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openKeywordTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("QAVOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("QAVOR_TEST_POSTGRES_DSN 未配置")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("连接测试数据库: %v", err)
	}
	var databaseName string
	if err := db.Raw("SELECT current_database()").Scan(&databaseName).Error; err != nil {
		t.Fatalf("读取数据库名: %v", err)
	}
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		t.Fatalf("拒绝在非测试数据库 %q 中运行", databaseName)
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		t.Fatalf("启用 pg_trgm: %v", err)
	}
	if err := db.AutoMigrate(&entity.KnowledgeFile{}, &entity.KnowledgeChunk{}); err != nil {
		t.Fatalf("迁移关键词测试表: %v", err)
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_content_trgm ON knowledge_chunks USING gist (content gist_trgm_ops(siglen=64))`).Error; err != nil {
		t.Fatalf("创建 trigram 索引: %v", err)
	}
	return db
}

func TestKnowledgeChunkRepository_FindKeywordByKBIDs(t *testing.T) {
	db := openKeywordTestDB(t)
	prefix := "keyword-test-"
	db.Where("kb_id LIKE ?", prefix+"%").Delete(&entity.KnowledgeChunk{})
	db.Where("kb_id LIKE ?", prefix+"%").Delete(&entity.KnowledgeFile{})
	t.Cleanup(func() {
		db.Where("kb_id LIKE ?", prefix+"%").Delete(&entity.KnowledgeChunk{})
		db.Where("kb_id LIKE ?", prefix+"%").Delete(&entity.KnowledgeFile{})
	})
	now := time.Now()
	files := []*entity.KnowledgeFile{
		{FileID: prefix + "refund-file", KBID: prefix + "allowed", Filename: "refund.md", Status: entity.FileIndexed},
		{FileID: prefix + "api-file", KBID: prefix + "allowed", Filename: "api.md", Status: entity.FileIndexed},
		{FileID: prefix + "other-file", KBID: prefix + "other", Filename: "other.md", Status: entity.FileIndexed},
		{FileID: prefix + "pending-file", KBID: prefix + "allowed", Filename: "pending.md", Status: entity.FileParsed},
	}
	for _, file := range files {
		file.CreatedAt, file.UpdatedAt = now, now
		if err := db.Create(file).Error; err != nil {
			t.Fatalf("写入测试文件 %s: %v", file.FileID, err)
		}
	}
	chunks := []*entity.KnowledgeChunk{
		{ChunkID: prefix + "refund", FileID: files[0].FileID, KBID: files[0].KBID, ChunkIndex: 0, Content: "退款流程需要先提交订单编号", TokenCount: 12},
		{ChunkID: prefix + "api", FileID: files[1].FileID, KBID: files[1].KBID, ChunkIndex: 0, Content: "接口编号 API-2026 用于批量查询", TokenCount: 12},
		{ChunkID: prefix + "other", FileID: files[2].FileID, KBID: files[2].KBID, ChunkIndex: 0, Content: "退款流程属于其他知识库", TokenCount: 10},
		{ChunkID: prefix + "pending", FileID: files[3].FileID, KBID: files[3].KBID, ChunkIndex: 0, Content: "退款流程尚未完成入库", TokenCount: 10},
	}
	for _, chunk := range chunks {
		chunk.CreatedAt, chunk.UpdatedAt = now, now
		if err := db.Create(chunk).Error; err != nil {
			t.Fatalf("写入测试分块 %s: %v", chunk.ChunkID, err)
		}
	}

	repo := NewKnowledgeChunkRepository(db)
	refund, err := repo.FindKeywordByKBIDs(context.Background(), []string{prefix + "allowed"}, "退款流程", 1)
	if err != nil {
		t.Fatalf("检索退款流程: %v", err)
	}
	if len(refund) != 1 || refund[0].ChunkID != prefix+"refund" || refund[0].Score <= 0 {
		t.Fatalf("退款流程结果=%+v", refund)
	}
	api, err := repo.FindKeywordByKBIDs(context.Background(), []string{prefix + "allowed"}, "API-2026", 1)
	if err != nil {
		t.Fatalf("检索 API-2026: %v", err)
	}
	if len(api) != 1 || api[0].ChunkID != prefix+"api" || api[0].Score <= 0 {
		t.Fatalf("API-2026 结果=%+v", api)
	}
}
