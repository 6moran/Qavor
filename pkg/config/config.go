package config

import (
	"errors"
	"strings"
	"time"
)

// Config 应用配置结构体
type Config struct {
	App           AppConfig           `mapstructure:"app"`
	Auth          AuthConfig          `mapstructure:"auth"`
	Database      DatabaseConfig      `mapstructure:"database"`
	DocumentQueue DocumentQueueConfig `mapstructure:"document_queue"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	Log           LogConfig           `mapstructure:"log"`
	CORS          CORSConfig          `mapstructure:"cors"`
	Email         EmailConfig         `mapstructure:"email"`
	Ollama        OllamaConfig        `mapstructure:"ollama"` // Ollama 配置（可选）
	MCP           MCPConfig           `mapstructure:"mcp"`
	SSE           SSEConfig           `mapstructure:"sse"` // SSE 流式服务配置
}

// SSEConfig SSE 流式服务配置
type SSEConfig struct {
	MaxStreamTime      int `mapstructure:"max_stream_time"`      // 单次流式最大时长（秒）
	HeartbeatInterval  int `mapstructure:"heartbeat_interval"`  // 心跳间隔（秒）
	MaxConcurrentTasks int `mapstructure:"max_concurrent_tasks"` // 单用户最大并发任务数
}

// ApplyDefaults 为未显式配置的 SSE 参数设置默认值
func (c *SSEConfig) ApplyDefaults() {
	if c.MaxStreamTime <= 0 {
		c.MaxStreamTime = 300 // 5分钟
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 15 // 15秒
	}
	if c.MaxConcurrentTasks <= 0 {
		c.MaxConcurrentTasks = 5 // 5个并发任务
	}
}

// MCPConfig MCP 配置
type MCPConfig struct {
	ToolRetrieval MCPToolRetrievalConfig `mapstructure:"tool_retrieval"`
}

// MCPToolRetrievalConfig MCP 工具向量检索配置
type MCPToolRetrievalConfig struct {
	Enabled           bool         `mapstructure:"enabled"`
	Threshold         int          `mapstructure:"threshold"`
	TopK              int          `mapstructure:"top_k"`
	EmbeddingProvider string       `mapstructure:"embedding_provider"`
	EmbeddingModel    string       `mapstructure:"embedding_model"`
	Ollama            OllamaConfig `mapstructure:"ollama"` // Ollama 配置（可选）
}

// OllamaConfig Ollama 配置
type OllamaConfig struct {
	BaseURL string `mapstructure:"base_url"` // Ollama 服务地址
	Model   string `mapstructure:"model"`    // 默认模型
}

// DocumentQueueConfig 配置文档异步处理使用的 Redis Stream。
type DocumentQueueConfig struct {
	ParseStream           string `mapstructure:"parse_stream"`
	ParseGroup            string `mapstructure:"parse_group"`
	ReadBlockSeconds      int    `mapstructure:"read_block_seconds"`
	PendingCheckSeconds   int    `mapstructure:"pending_check_seconds"`
	PendingMinIdleMinutes int    `mapstructure:"pending_min_idle_minutes"`
	PendingClaimCount     int64  `mapstructure:"pending_claim_count"`
	MaxStreamLength       int64  `mapstructure:"max_stream_length"`
}

// ApplyDefaults 为未显式配置的文档队列参数设置安全默认值。
func (c *DocumentQueueConfig) ApplyDefaults() {
	if c.ParseStream == "" {
		c.ParseStream = "qavor:document:parse"
	}
	if c.ParseGroup == "" {
		c.ParseGroup = "qavor-document-parse-workers"
	}
	if c.ReadBlockSeconds <= 0 {
		c.ReadBlockSeconds = 5
	}
	if c.PendingCheckSeconds <= 0 {
		c.PendingCheckSeconds = 60
	}
	if c.PendingMinIdleMinutes <= 0 {
		c.PendingMinIdleMinutes = 30
	}
	if c.PendingClaimCount <= 0 {
		c.PendingClaimCount = 10
	}
	if c.MaxStreamLength <= 0 {
		c.MaxStreamLength = 100000
	}
}

// AuthConfig 单实例管理员认证配置。
type AuthConfig struct {
	AdminUsername string `mapstructure:"admin_username"`
	AdminPassword string `mapstructure:"admin_password"`
}

// ValidateAuth 校验单实例认证启动所需配置。
func (c *Config) ValidateAuth() error {
	if strings.TrimSpace(c.Auth.AdminUsername) == "" {
		return errors.New("缺少 auth.admin_username")
	}
	if c.Auth.AdminPassword == "" {
		return errors.New("缺少 auth.admin_password")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return errors.New("缺少 jwt.secret")
	}
	return nil
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Mode    string `mapstructure:"mode"` // debug, release, test
	Port    int    `mapstructure:"port"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Redis       RedisConfig    `mapstructure:"redis"`
	Postgres    PostgresConfig `mapstructure:"postgres"`
	MinIO       MinIOConfig    `mapstructure:"minio"`
	AutoMigrate bool           `mapstructure:"auto_migrate"` // 是否自动迁移数据库
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// MinIOConfig MinIO 配置
type MinIOConfig struct {
	Endpoint       string   `mapstructure:"endpoint"`
	AccessKey      string   `mapstructure:"access_key"`
	SecretKey      string   `mapstructure:"secret_key"`
	Bucket         string   `mapstructure:"bucket"`
	UseSSL         bool     `mapstructure:"use_ssl"`
	Region         string   `mapstructure:"region"`
	PublicEndpoint string   `mapstructure:"public_endpoint"` // 对外访问地址，如 https://cdn.example.com
	MaxFileSize    int64    `mapstructure:"max_file_size"`   // 最大文件大小(bytes)，默认 50MB
	AllowedTypes   []string `mapstructure:"allowed_types"`   // 允许的 MIME 类型，空=不限制
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret      string        `mapstructure:"secret"`       // JWT 密钥
	ExpireHours time.Duration `mapstructure:"expire_hours"` // 访问令牌过期时间（小时）
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Filename   string `mapstructure:"filename"`    // 日志文件路径
	MaxSize    int    `mapstructure:"max_size"`    // 单个日志文件最大大小(MB)
	MaxBackups int    `mapstructure:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `mapstructure:"max_age"`     // 保留旧日志文件的最大天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTP     SMTPConfig `mapstructure:"smtp"`
	From     string     `mapstructure:"from"`      // 发件人邮箱
	FromName string     `mapstructure:"from_name"` // 发件人名称
}

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host     string `mapstructure:"host"`      // SMTP 服务器地址
	Port     int    `mapstructure:"port"`      // SMTP 端口
	Account  string `mapstructure:"account"`   // 邮箱账号
	AuthCode string `mapstructure:"auth_code"` // 邮箱授权码（非登录密码）
}
