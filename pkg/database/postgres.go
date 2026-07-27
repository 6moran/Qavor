package database

import (
	"fmt"
	"time"

	"Qavor/pkg/config"
	"Qavor/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var postgresDB *gorm.DB

// InitPostgres 初始化 PostgreSQL 连接
func InitPostgres(cfg *config.PostgresConfig) (*gorm.DB, error) {
	// 构建 DSN
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Host,
		cfg.Username,
		cfg.Password,
		cfg.Database,
		cfg.Port,
		cfg.SSLMode,
	)

	// GORM 配置
	gormConfig := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	}

	// 根据应用模式设置日志级别
	if config.Get().App.Mode == "debug" {
		gormConfig.Logger = gormlogger.Default.LogMode(gormlogger.Info)
	} else {
		gormConfig.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
	}

	// 连接数据库
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		logger.Error("PostgreSQL 连接失败", zap.Error(err))
		return nil, fmt.Errorf("PostgreSQL 连接失败: %w", err)
	}

	// 获取底层 sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("获取 SQL DB 失败", zap.Error(err))
		return nil, fmt.Errorf("获取 SQL DB 失败: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		logger.Error("PostgreSQL Ping 失败", zap.Error(err))
		return nil, fmt.Errorf("PostgreSQL Ping 失败: %w", err)
	}

	postgresDB = db
	logger.Info("PostgreSQL 连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
	)

	return db, nil
}

// GetPostgres 获取 PostgreSQL 实例
func GetPostgres() *gorm.DB {
	if postgresDB == nil {
		panic("PostgreSQL 未初始化")
	}
	return postgresDB
}

// ClosePostgres 关闭 PostgreSQL 连接
func ClosePostgres() error {
	if postgresDB != nil {
		sqlDB, err := postgresDB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
