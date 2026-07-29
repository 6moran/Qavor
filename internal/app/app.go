package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Qavor/internal/api"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/service"
	"Qavor/pkg/config"
	"Qavor/pkg/database"
	"Qavor/pkg/email"
	"Qavor/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 应用结构体
type App struct {
	cfg        *config.Config
	postgresDB *gorm.DB
	redis      *redis.Client
	router     *api.Router
	server     *http.Server
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 1. 加载配置
	if err := a.initConfig(); err != nil {
		return err
	}

	// 2. 初始化日志
	if err := a.initLogger(); err != nil {
		return err
	}

	// 3. 初始化数据库
	if err := a.initDatabase(); err != nil {
		return err
	}

	// 4. 初始化依赖
	a.initDependencies()

	// 5. 初始化路由
	a.initRouter()

	// 6. 初始化服务器
	a.initServer()

	return nil
}

// initConfig 加载配置
func (a *App) initConfig() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	a.cfg = cfg
	return nil
}

// initLogger 初始化日志
func (a *App) initLogger() error {
	if err := logger.Init(&a.cfg.Log); err != nil {
		return fmt.Errorf("日志初始化失败: %w", err)
	}

	// 打印启动横幅
	logger.Info("=========================================")
	logger.Info(fmt.Sprintf("欢迎使用 %s", a.cfg.App.Name))
	logger.Info(fmt.Sprintf("版本: %s", a.cfg.App.Version))
	logger.Info(fmt.Sprintf("模式: %s", a.cfg.App.Mode))
	logger.Info("配置加载成功")
	logger.Info("=========================================")

	return nil
}

// initDatabase 初始化数据库
func (a *App) initDatabase() error {
	// 初始化 PostgreSQL
	postgresDB, err := database.InitPostgres(&a.cfg.Database.Postgres)
	if err != nil {
		return fmt.Errorf("PostgreSQL 初始化失败: %w", err)
	}
	a.postgresDB = postgresDB

	// 根据配置决定是否自动迁移数据库表
	if a.cfg.Database.AutoMigrate {
		logger.Info("开始数据库迁移...")
		if err := a.postgresDB.AutoMigrate(
			&entity.User{},
			&entity.Agent{},
			&entity.AgentEnv{},
			&entity.Conversation{},
			&entity.ConversationStats{},
			&entity.Message{},
			&entity.MessageFeedback{},
			&entity.ToolCall{},
			&entity.APIKey{},
			&entity.OperationLog{},
			&entity.AgentRun{},
			&entity.SubagentThread{},
			&entity.TaskRecord{},
			&entity.ModelProvider{},
			&entity.MCPServer{},
			&entity.Skill{},
			&entity.KnowledgeBase{},
			&entity.KnowledgeFile{},
			&entity.KnowledgeChunk{},
		); err != nil {
			logger.Warn("数据库迁移警告", zap.Error(err))
		} else {
			logger.Info("数据库迁移完成")
		}
	} else {
		logger.Info("数据库自动迁移已禁用，跳过迁移步骤")
	}

	// 初始化 Redis（可选）
	rs, err := database.InitRedis(&a.cfg.Database.Redis)
	if err != nil {
		logger.Warn("Redis 初始化失败，将不影响核心功能", zap.Error(err))
	}
	a.redis = rs

	return nil
}

// initDependencies 初始化依赖注入
func (a *App) initDependencies() {
	// 创建 Repository
	userRepo := repository.NewUserRepository(a.postgresDB)
	providerRepo := repository.NewModelProviderRepository(a.postgresDB)

	// 创建邮件客户端
	emailClient := email.NewSMTPClient(&a.cfg.Email)

	// 创建 Service
	userSvc := service.NewUserService(userRepo)
	authSvc := service.NewAuthService(userRepo, userSvc, emailClient)
	providerSvc := service.NewModelProviderService(providerRepo)

	// 创建 Router
	a.router = api.NewRouter(userSvc, authSvc, providerSvc)
}

// initRouter 初始化路由
func (a *App) initRouter() {
	// 设置 Gin 模式
	gin.SetMode(a.cfg.App.Mode)
}

// initServer 初始化 HTTP 服务器
func (a *App) initServer() {
	engine := gin.New()

	// 注册路由
	a.router.Setup(engine)

	// 创建 HTTP 服务器
	a.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.App.Port),
		Handler:        engine,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}

// Run 运行应用
func (a *App) Run() {
	// 启动 HTTP 服务器
	go func() {
		logger.Info("HTTP 服务器启动",
			zap.String("addr", a.server.Addr),
			zap.String("mode", a.cfg.App.Mode),
		)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	a.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (a *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭 HTTP 服务器
	if err := a.server.Shutdown(ctx); err != nil {
		logger.Error("服务器关闭失败", zap.Error(err))
	}

	// 关闭数据库连接
	_ = database.ClosePostgres()
	_ = database.CloseRedis()

	// 同步日志
	_ = logger.Sync()

	logger.Info("服务器已关闭")
	logger.Info("=========================================")
}
