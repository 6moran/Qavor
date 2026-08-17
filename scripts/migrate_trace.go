//go:build ignore

// 临时脚本：为链路追踪创建 agent_traces / agent_trace_spans（旧表）和 trace_records / trace_spans（新表）。
// AutoMigrate 幂等，不删除旧表。
// 用法: go run scripts/migrate_trace.go
package main

import (
	"fmt"
	"os"

	"Qavor/internal/model/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("QAVOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = "host=39.105.40.22 port=5432 user=postgres password=1458963 dbname=qavor sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("open db error:", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(
		&entity.AgentTrace{}, &entity.AgentTraceSpan{},
		&entity.TraceRecord{}, &entity.TraceSpan{},
	); err != nil {
		fmt.Println("migrate error:", err)
		os.Exit(1)
	}
	fmt.Println("agent_traces / agent_trace_spans（旧表）和 trace_records / trace_spans（新表）已就绪")
}
