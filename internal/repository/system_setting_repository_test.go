package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"Qavor/internal/model/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openSystemSettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("QAVOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("QAVOR_TEST_POSTGRES_DSN 未配置")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
	if err := db.AutoMigrate(&entity.SystemSetting{}); err != nil {
		t.Fatalf("迁移 system_settings: %v", err)
	}
	return db
}

func TestSystemSettingRepository_GetMissingAndUpsert(t *testing.T) {
	db := openSystemSettingTestDB(t)
	ctx := context.Background()
	key := "test.rag.rerank_model_id"
	db.Where("key = ?", key).Delete(&entity.SystemSetting{})
	t.Cleanup(func() { db.Where("key = ?", key).Delete(&entity.SystemSetting{}) })

	repo := NewSystemSettingRepository(db)
	value, found, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("读取缺失设置: %v", err)
	}
	if found || value != "" {
		t.Fatalf("缺失设置返回 value=%q found=%v，期望空值且不存在", value, found)
	}

	if err := repo.Upsert(ctx, key, "7"); err != nil {
		t.Fatalf("首次写入设置: %v", err)
	}
	value, found, err = repo.Get(ctx, key)
	if err != nil || !found || value != "7" {
		t.Fatalf("首次写入后读取 value=%q found=%v err=%v", value, found, err)
	}

	if err := repo.Upsert(ctx, key, "9"); err != nil {
		t.Fatalf("更新设置: %v", err)
	}
	value, found, err = repo.Get(ctx, key)
	if err != nil || !found || value != "9" {
		t.Fatalf("更新后读取 value=%q found=%v err=%v", value, found, err)
	}
	var count int64
	if err := db.Model(&entity.SystemSetting{}).Where("key = ?", key).Count(&count).Error; err != nil {
		t.Fatalf("统计设置行数: %v", err)
	}
	if count != 1 {
		t.Fatalf("重复 upsert 后行数=%d，期望 1", count)
	}
}
