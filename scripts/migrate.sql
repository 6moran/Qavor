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

-- ============================================================
-- RAG 评估模块（evaluation）：评估基准 + 评估运行
-- 对应 entity: EvaluationDataset / EvaluationDatasetItem / EvaluationRun / EvaluationRunResult
-- ============================================================

-- 评估基准（评测数据集）
CREATE TABLE IF NOT EXISTS evaluation_datasets (
    id                BIGSERIAL PRIMARY KEY,
    dataset_id        VARCHAR(64)  NOT NULL UNIQUE,
    kb_id             VARCHAR(80)  NOT NULL,
    name              VARCHAR(100) NOT NULL,
    description       TEXT,
    item_count        INTEGER      NOT NULL DEFAULT 0,
    has_gold_chunks   BOOLEAN      NOT NULL DEFAULT FALSE,
    has_gold_answers  BOOLEAN      NOT NULL DEFAULT FALSE,
    build_metadata    JSONB        NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_evaluation_datasets_kb_id ON evaluation_datasets(kb_id);

-- 评估基准问答条目
CREATE TABLE IF NOT EXISTS evaluation_dataset_items (
    id              BIGSERIAL PRIMARY KEY,
    dataset_id      VARCHAR(64) NOT NULL,
    query           TEXT        NOT NULL,
    gold_chunk_ids  JSONB       NOT NULL DEFAULT '[]',
    gold_answer     TEXT,
    sort_order      INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_evaluation_dataset_items_dataset ON evaluation_dataset_items(dataset_id);

-- 评估运行
CREATE TABLE IF NOT EXISTS evaluation_runs (
    id               BIGSERIAL PRIMARY KEY,
    run_id           VARCHAR(64) NOT NULL UNIQUE,
    kb_id            VARCHAR(80) NOT NULL,
    dataset_id       VARCHAR(64) NOT NULL,
    name             VARCHAR(100) NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    total_items      INTEGER     NOT NULL DEFAULT 0,
    completed_items  INTEGER     NOT NULL DEFAULT 0,
    overall_score    DOUBLE PRECISION NOT NULL DEFAULT 0,
    metrics          JSONB       NOT NULL DEFAULT '{}',
    retrieval_config JSONB       NOT NULL DEFAULT '{}',
    progress         DOUBLE PRECISION NOT NULL DEFAULT 0,
    message          TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_kb_id ON evaluation_runs(kb_id);

-- 评估运行单项结果
CREATE TABLE IF NOT EXISTS evaluation_run_results (
    id               BIGSERIAL PRIMARY KEY,
    run_id           VARCHAR(64) NOT NULL,
    query            TEXT        NOT NULL,
    generated_answer TEXT,
    metrics          JSONB       NOT NULL DEFAULT '{}',
    answer_score     DOUBLE PRECISION,
    error_message    TEXT,
    status           VARCHAR(20) NOT NULL DEFAULT 'completed',
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_evaluation_run_results_run ON evaluation_run_results(run_id);

-- ============================================================
-- 全局系统设置（system_settings）：RAG 设置等键值配置
-- 对应 entity: SystemSetting
-- ============================================================
CREATE TABLE IF NOT EXISTS system_settings (
    id         BIGSERIAL PRIMARY KEY,
    key        VARCHAR(128) NOT NULL UNIQUE,
    value      TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
