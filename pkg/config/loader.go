package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"os"
	"strconv"
	"strings"
	"time"
)

// atoiOrDefault 将字符串转换为 int，失败时返回默认值
func atoiOrDefault(s string, defaultVal int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultVal
}

// atoi64OrDefault 将字符串转换为 int64，失败时返回默认值
func atoi64OrDefault(s string, defaultVal int64) int64 {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	return defaultVal
}

var globalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// 环境变量前缀
	v.SetEnvPrefix("QAVOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	config.DocumentQueue.ApplyDefaults()
	config.SSE.ApplyDefaults()

	err := godotenv.Load(".env")
	if err != nil {
		panic("加载.env失败")
	}
	// 从环境变量覆盖配置
	// Redis
	if val := os.Getenv("REDIS_HOST"); val != "" {
		config.Database.Redis.Host = val
	}
	if val := os.Getenv("REDIS_PORT"); val != "" {
		config.Database.Redis.Port = atoiOrDefault(val, config.Database.Redis.Port)
	}
	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		config.Database.Redis.Password = val
	}
	if val := os.Getenv("REDIS_DB"); val != "" {
		config.Database.Redis.DB = atoiOrDefault(val, config.Database.Redis.DB)
	}
	// PostgreSQL 环境变量覆盖
	if val := os.Getenv("POSTGRES_HOST"); val != "" {
		config.Database.Postgres.Host = val
	}
	if val := os.Getenv("POSTGRES_PORT"); val != "" {
		config.Database.Postgres.Port = atoiOrDefault(val, config.Database.Postgres.Port)
	}
	if val := os.Getenv("POSTGRES_USERNAME"); val != "" {
		config.Database.Postgres.Username = val
	}
	if val := os.Getenv("POSTGRES_PASSWORD"); val != "" {
		config.Database.Postgres.Password = val
	}
	if val := os.Getenv("POSTGRES_DATABASE"); val != "" {
		config.Database.Postgres.Database = val
	}
	if val := os.Getenv("POSTGRES_SSLMODE"); val != "" {
		config.Database.Postgres.SSLMode = val
	}
	// MinIO 环境变量覆盖
	if val := os.Getenv("MINIO_ENDPOINT"); val != "" {
		config.Database.MinIO.Endpoint = val
	}
	if val := os.Getenv("MINIO_ACCESS_KEY"); val != "" {
		config.Database.MinIO.AccessKey = val
	}
	if val := os.Getenv("MINIO_SECRET_KEY"); val != "" {
		config.Database.MinIO.SecretKey = val
	}
	if val := os.Getenv("MINIO_BUCKET"); val != "" {
		config.Database.MinIO.Bucket = val
	}
	if val := os.Getenv("MINIO_USE_SSL"); val != "" {
		config.Database.MinIO.UseSSL = val == "true"
	}
	if val := os.Getenv("MINIO_REGION"); val != "" {
		config.Database.MinIO.Region = val
	}
	if val := os.Getenv("MINIO_PUBLIC_ENDPOINT"); val != "" {
		config.Database.MinIO.PublicEndpoint = val
	}
	if val := os.Getenv("MINIO_MAX_FILE_SIZE"); val != "" {
		config.Database.MinIO.MaxFileSize = atoi64OrDefault(val, config.Database.MinIO.MaxFileSize)
	}
	if val := os.Getenv("QAVOR_AUTH_ADMIN_USERNAME"); val != "" {
		config.Auth.AdminUsername = val
	}
	if val := os.Getenv("QAVOR_AUTH_ADMIN_PASSWORD"); val != "" {
		config.Auth.AdminPassword = val
	}
	if val := os.Getenv("JWT_SECRET"); val != "" {
		config.JWT.Secret = val
	}
	if val := os.Getenv("QAVOR_JWT_SECRET"); val != "" {
		config.JWT.Secret = val
	}
	if val := os.Getenv("JWT_EXPIRE_HOURS"); val != "" {
		config.JWT.ExpireHours = time.Duration(atoiOrDefault(val, int(config.JWT.ExpireHours))) * time.Hour
	}
	if val := os.Getenv("APP_MODE"); val != "" {
		config.App.Mode = val
	}
	if val := os.Getenv("APP_PORT"); val != "" {
		config.App.Port = atoiOrDefault(val, config.App.Port)
	}

	globalConfig = config
	return config, nil
}

// Get 获取全局配置
func Get() *Config {
	if globalConfig == nil {
		panic("配置未初始化，请先调用 Load() 加载配置")
	}
	return globalConfig
}

// MustLoad 加载配置，失败时 panic
func MustLoad(configPath string) *Config {
	config, err := Load(configPath)
	if err != nil {
		panic(err)
	}
	return config
}
