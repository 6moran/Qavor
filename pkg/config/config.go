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
	Ollama        OllamaConfig        `mapstructure:"ollama"` // Ollama 配置（可选）
	RAG           RAGConfig           `mapstructure:"rag"`
	MCP           MCPConfig           `mapstructure:"mcp"`
	SSE           SSEConfig           `mapstructure:"sse"` // SSE 流式服务配置
	Agent         AgentConfig         `mapstructure:"agent"`
}

// AgentConfig agent 运行时配置（本地文件系统与安全管控）。
type AgentConfig struct {
	// WorkspaceRoot agent 默认工作区根目录，每个 agent 在其下建 <slug> 子目录。
	WorkspaceRoot string `mapstructure:"workspace_root"`
	// Security 文件与 Shell 安全管控配置。
	Security SecurityConfig `mapstructure:"security"`
}

// SecurityConfig 文件与 Shell 安全管控配置，全部机制默认开启。
// 所有 *bool 字段：nil 视为开启，显式 false 关闭。
type SecurityConfig struct {
	Enabled     *bool             `mapstructure:"enabled"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
	Command     CommandConfig     `mapstructure:"command"`
	Redaction   RedactionConfig   `mapstructure:"redaction"`
	Syntax      SyntaxConfig      `mapstructure:"syntax"`
	Staleness   StalenessConfig   `mapstructure:"staleness"`
	LinePrefix  LinePrefixConfig  `mapstructure:"line_prefix"`
	ExitCode    ExitCodeConfig    `mapstructure:"exit_code"`
	Output      OutputConfig      `mapstructure:"output"`
	// ShellTimeoutSeconds 单条 shell 命令超时秒数（0=无显式超时，依赖后台任务管理器的前台超时兜底）。
	ShellTimeoutSeconds int `mapstructure:"shell_timeout_seconds"`
}

// CredentialsConfig 凭据路径守卫配置。
type CredentialsConfig struct {
	Enabled       *bool    `mapstructure:"enabled"`
	ExtraPatterns []string `mapstructure:"extra_patterns"`
}

// CommandConfig 高危命令黑名单配置。
type CommandConfig struct {
	Enabled   *bool    `mapstructure:"enabled"`
	ExtraBans []string `mapstructure:"extra_bans"`
}

// RedactionConfig 输出脱敏配置。
type RedactionConfig struct {
	Enabled      *bool    `mapstructure:"enabled"`
	ExtraEnvKeys []string `mapstructure:"extra_env_keys"`
}

// SyntaxConfig 写前语法预检配置。
type SyntaxConfig struct {
	Enabled *bool `mapstructure:"enabled"`
}

// StalenessConfig 陈旧警告配置。
type StalenessConfig struct {
	Enabled *bool `mapstructure:"enabled"`
}

// LinePrefixConfig read 行号前缀检测配置。
type LinePrefixConfig struct {
	Enabled *bool `mapstructure:"enabled"`
}

// ExitCodeConfig 退出码语义解释配置。
type ExitCodeConfig struct {
	Enabled *bool `mapstructure:"enabled"`
}

// OutputConfig 输出截断配置。
type OutputConfig struct {
	MaxBytes          int `mapstructure:"max_bytes"`           // shell 输出截断阈值，默认 50KB
	OffloadTokenLimit int `mapstructure:"offload_token_limit"` // 大工具结果落盘 token 阈值，默认 20000
}

// ApplyDefaults 为 agent 安全配置设置默认值（默认开启安全基线）。
func (c *AgentConfig) ApplyDefaults() {
	if c.WorkspaceRoot == "" {
		c.WorkspaceRoot = "data/workspaces"
	}
	enabled := true
	setDefaultBool := func(p **bool) {
		if *p == nil {
			*p = &enabled
		}
	}
	setDefaultBool(&c.Security.Enabled)
	setDefaultBool(&c.Security.Credentials.Enabled)
	setDefaultBool(&c.Security.Command.Enabled)
	setDefaultBool(&c.Security.Redaction.Enabled)
	setDefaultBool(&c.Security.Syntax.Enabled)
	setDefaultBool(&c.Security.Staleness.Enabled)
	setDefaultBool(&c.Security.LinePrefix.Enabled)
	setDefaultBool(&c.Security.ExitCode.Enabled)
	if c.Security.Output.MaxBytes <= 0 {
		c.Security.Output.MaxBytes = 51200
	}
	if c.Security.Output.OffloadTokenLimit <= 0 {
		c.Security.Output.OffloadTokenLimit = 20000
	}
	Run           RunConfig           `mapstructure:"run"` // Run 执行器 / 队列配置
}

// RunConfig Run 执行器与请求队列配置
type RunConfig struct {
	WorkerCount    int   `mapstructure:"worker_count"`     // Worker 池大小
	StreamMaxLen   int64 `mapstructure:"stream_max_len"`   // Redis Stream 近似最大长度
	LockTTLSeconds int   `mapstructure:"lock_ttl_seconds"` // 每线程执行锁 TTL（秒）
	BlockSeconds   int   `mapstructure:"block_seconds"`    // XREAD/BRPOP 阻塞时长（秒）
	RetentionHours int   `mapstructure:"retention_hours"`  // Run 事件流保留时长（小时）
}

// ApplyDefaults 为 Run 配置设置安全默认值
func (c *RunConfig) ApplyDefaults() {
	if c.WorkerCount <= 0 {
		c.WorkerCount = 3
	}
	if c.StreamMaxLen <= 0 {
		c.StreamMaxLen = 10000
	}
	if c.LockTTLSeconds <= 0 {
		c.LockTTLSeconds = 1800
	}
	if c.BlockSeconds <= 0 {
		c.BlockSeconds = 5
	}
	if c.RetentionHours <= 0 {
		c.RetentionHours = 24
	}
}

// RAGConfig RAG 功能配置。第一版仅支持文档索引和问答同步接口。
type RAGConfig struct {
	HistoryLimit          int `mapstructure:"history_limit"`
	ChunkTokens           int `mapstructure:"chunk_tokens"`
	ChunkOverlapTokens    int `mapstructure:"chunk_overlap_tokens"`
	VectorTopK            int `mapstructure:"vector_top_k"`
	KeywordTopK           int `mapstructure:"keyword_top_k"`
	FusedTopK             int `mapstructure:"fused_top_k"`
	RerankTopK            int `mapstructure:"rerank_top_k"`
	RRFK                  int `mapstructure:"rrf_k"`
	TopK                  int `mapstructure:"top_k"` // 兼容字段：未启用融合/重排时使用
	RequestTimeoutSeconds int `mapstructure:"request_timeout_seconds"`
	// Chat/Embedding 模型由知识库绑定的模型 ID 决定，不从这里读取。
	// Embedding 仅保留批处理参数，旧字段保留用于兼容已有 Go 调用方。
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Reranker  RerankerConfig  `mapstructure:"reranker"`
}

// ApplyDefaults 为 RAG 参数设置安全默认值。
// 模型不在配置文件中判断；每个知识库绑定模型后，运行时按 KBID 解析。
func (c *RAGConfig) ApplyDefaults() {
	if c.HistoryLimit <= 0 {
		c.HistoryLimit = 10
	}
	if c.ChunkTokens <= 0 {
		c.ChunkTokens = 800
	}
	if c.ChunkOverlapTokens <= 0 {
		c.ChunkOverlapTokens = 100
	}
	if c.VectorTopK <= 0 {
		c.VectorTopK = 20
	}
	if c.KeywordTopK <= 0 {
		c.KeywordTopK = 20
	}
	if c.FusedTopK <= 0 {
		c.FusedTopK = 20
	}
	if c.RerankTopK <= 0 {
		c.RerankTopK = 5
	}
	if c.RRFK <= 0 {
		c.RRFK = 60
	}
	if c.TopK <= 0 {
		c.TopK = 5
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = 60
	}
	if c.Embedding.BatchSize <= 0 {
		// 模型目前最大只支持到20
		c.Embedding.BatchSize = 19
	}
	if c.Reranker.TimeoutSeconds <= 0 {
		c.Reranker.TimeoutSeconds = 20
	}
}

// IsConfigured 当 Embedding 关键字段齐全时认为 RAG 已配置可用。
// MVP 不要求 Reranker；问答是否可用还取决于 ChatModelID 是否配置。
func (c *RAGConfig) IsConfigured() bool {
	return c.ChunkTokens > 0 && c.RequestTimeoutSeconds > 0
}

// IsAnswerReady 当 Embedding 与 ChatModel 均配置时问答可用。
func (c *RAGConfig) IsAnswerReady() bool {
	return c.IsConfigured()
}

// EmbeddingConfig Embedding 模型配置。
type EmbeddingConfig struct {
	// 以下连接字段仅用于兼容旧调用方，生产 RAG 不再读取它们。
	Model     string `mapstructure:"model"`
	BaseURL   string `mapstructure:"base_url"`
	APIKey    string `mapstructure:"api_key"`
	Dimension int    `mapstructure:"dimension"`
	BatchSize int    `mapstructure:"batch_size"`
}

// RerankerConfig 重排器配置。
type RerankerConfig struct {
	Model          string       `mapstructure:"model"`
	BaseURL        string       `mapstructure:"base_url"`
	APIKey         string       `mapstructure:"api_key"`
	TimeoutSeconds int          `mapstructure:"timeout_seconds"`
	Ollama         OllamaConfig `mapstructure:"ollama"` // Ollama 配置（可选）
	MCP            MCPConfig    `mapstructure:"mcp"`
	SSE            SSEConfig    `mapstructure:"sse"` // SSE 流式服务配置
}

// SSEConfig SSE 流式服务配置
type SSEConfig struct {
	MaxStreamTime      int `mapstructure:"max_stream_time"`      // 单次流式最大时长（秒）
	HeartbeatInterval  int `mapstructure:"heartbeat_interval"`   // 心跳间隔（秒）
	MaxConcurrentTasks int `mapstructure:"max_concurrent_tasks"` // 单用户最大并发任务数
	TaskTTL            int `mapstructure:"task_ttl"`             // 任务过期时间（秒），默认3600
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
	Name      string `mapstructure:"name"`
	Version   string `mapstructure:"version"`
	Mode      string `mapstructure:"mode"` // debug, release, test
	Port      int    `mapstructure:"port"`
	SkillsDir string `mapstructure:"skills_dir"` // Skill 文件目录
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
