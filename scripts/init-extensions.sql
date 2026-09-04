-- Qavor PostgreSQL Docker 初始化脚本
-- 放在 /docker-entrypoint-initdb.d/ 下，容器首次启动时自动执行
-- 只创建扩展，不建表 —— 表由应用 GORM AutoMigrate 创建

-- pgvector: 向量类型 + 余弦相似度检索
CREATE EXTENSION IF NOT EXISTS vector;

-- pg_trgm: 中文友好的字符级关键词检索（trigram 相似度）
CREATE EXTENSION IF NOT EXISTS pg_trgm;
