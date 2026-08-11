-- Qavor PostgreSQL 迁移脚本
-- 注意：第一版不创建 HNSW / IVFFlat 索引，仅启用 pgvector 扩展并补齐字段。

-- 启用 pgvector 扩展（需先在 PostgreSQL 实例安装 pgvector）
CREATE EXTENSION IF NOT EXISTS vector;

-- 启用中文友好的字符级关键词检索扩展
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 知识库必须绑定 Embedding 与 Chat 模型；模型配置存放在 models 表。
ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS embedding_model_id bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS chat_model_id bigint NOT NULL DEFAULT 0;

-- 知识库类型固定为 pgvector，不再保存类型字段。
ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS kb_type;

-- knowledge_chunks 增加 RAG 所需字段
ALTER TABLE knowledge_chunks
    ADD COLUMN IF NOT EXISTS token_count integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS embedding vector;

-- 文件 + 分块序号 唯一约束，保证同一文件不会重复生成 Chunk
CREATE UNIQUE INDEX IF NOT EXISTS uk_knowledge_chunks_file_index
    ON knowledge_chunks(file_id, chunk_index);

-- 知识库 / 文件联合查询索引
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_kb_file
    ON knowledge_chunks(kb_id, file_id);

-- 为关键词 TopK 近邻排序创建 trigram GiST 索引
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_content_trgm
ON knowledge_chunks
USING gist (content gist_trgm_ops(siglen=64));
