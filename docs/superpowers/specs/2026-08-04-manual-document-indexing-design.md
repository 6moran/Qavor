# 知识库文件手动向量入库设计

## 1. 背景

当前文件上传接口虽然接收 `auto_index` 参数，但上传成功后无论该参数为何值，都会立即创建 `DocumentProcessingJob` 并投递到 Redis Stream。`DocumentWorker` 又在同一个任务中依次完成文档解析、Markdown 保存、分块和向量化，因此用户无法在上传后决定何时生成向量。

前端已经存在“等待解析”“待入库”“入库中”“已入库”等展示和操作，但后端尚未提供手动入库接口，前后端状态语义没有形成闭环。

本设计采用单一文件状态字段的分阶段状态机：上传后自动解析，解析成功后暂停；只有用户手动点击“入库”才执行分块和向量化。

## 2. 目标

1. 文件上传成功后自动解析并生成可预览的 Markdown。
2. 解析完成后不自动调用 Embedding，不写入向量分块。
3. 用户手动点击“入库”后，系统异步执行分块和向量化。
4. 解析失败与向量入库失败可以分别识别和重试。
5. 支持单文件和批量入库，并防止同一文件重复并发处理。
6. 保持现有 `KnowledgeFile.Status` 单字段模型，控制本阶段改造范围。

## 3. 非目标与约束

1. 本阶段不允许用户修改知识库已经绑定的 Embedding 模型。
2. 一个知识库内的所有文件始终使用该知识库统一绑定的 Embedding 模型。
3. 本阶段不实现多套向量版本并行、无损热切换或历史索引版本回滚。
4. 本阶段不拆分为 `parse_status` 和 `index_status` 两个文件字段。
5. 不在文件上传请求中同步完成解析或向量化。
6. `auto_index` 不再作为普通上传流程的有效开关；前后端应移除或忽略该参数，避免产生两套行为。

知识库的 Embedding 模型在创建后视为不可变。后端必须拒绝通过更新知识库接口修改 `embedding_model_id`。将来如果开放模型迁移，需要单独设计全库重建流程，而不是直接修改字段。

## 4. 状态模型

### 4.1 文件状态

`KnowledgeFile.Status` 使用以下规范状态：

| 状态 | 含义 | 用户可执行操作 |
| --- | --- | --- |
| `uploaded` | 原文件已保存，尚未进入解析队列 | 等待系统投递；异常滞留时可重试解析 |
| `parse_queued` | 解析任务已创建，等待 Worker | 查看、下载、删除 |
| `parsing` | 正在解析并生成 Markdown | 查看原文件、下载 |
| `parsed` | 解析成功，尚未向量入库 | 预览 Markdown、配置分块参数、入库 |
| `parse_failed` | 解析失败 | 查看错误、重试解析、删除 |
| `index_queued` | 入库任务已创建，等待 Worker | 查看、下载 |
| `indexing` | 正在分块并生成向量 | 查看、下载 |
| `indexed` | 向量入库成功，可以参与检索 | 查看分块、重新入库、删除 |
| `index_failed` | 入库失败，解析结果仍然有效 | 查看 Markdown、查看错误、重试入库、删除 |

文件夹记录不参与该状态机，继续使用 `completed`，并由 `IsFolder` 区分。

### 4.2 合法状态转移

```text
uploaded -> parse_queued -> parsing -> parsed
                              \-> parse_failed

parse_failed -> parse_queued

parsed -> index_queued -> indexing -> indexed
                               \-> index_failed

index_failed -> index_queued
indexed -> index_queued
```

服务层必须校验状态转移，Repository 通过带当前状态条件的原子更新完成抢占。例如，手动入库只允许：

```sql
UPDATE knowledge_files
SET status = 'index_queued'
WHERE kb_id = ?
  AND file_id = ?
  AND status IN ('parsed', 'index_failed', 'indexed');
```

受影响行数为零时返回“文件状态已变化或正在处理中”，不得再次创建任务。

### 4.3 任务状态

`DocumentProcessingJob` 增加 `job_type`：

- `parse`：读取原文件、解析、生成 Markdown。
- `index`：读取 Markdown、分块、调用知识库绑定的 Embedding 模型、写入向量。

任务状态继续使用：

```text
pending -> running -> succeeded
                   \-> failed
                   \-> pending   （租约恢复）
pending/running -> cancelled
```

文件状态描述当前业务结果，任务状态描述一次异步执行的生命周期。

## 5. 业务流程

### 5.1 上传与自动解析

1. 校验知识库、父目录和文件。
2. 将原文件上传至 MinIO。
3. 创建 `KnowledgeFile`，初始状态设为 `uploaded`。
4. 在数据库事务中将文件更新为 `parse_queued`，同时创建 `job_type=parse` 的任务。
5. 事务提交后向 Redis Stream 投递任务消息。
6. Worker 领取任务后，将文件从 `parse_queued` 原子更新为 `parsing`。
7. Worker 解析文件并保存 `normalized.md`。
8. 成功时更新 `markdown_file`、清空错误信息，并将状态改为 `parsed`。
9. 失败时保留原文件，将状态改为 `parse_failed` 并记录可展示的错误信息。

如果 Redis 投递失败，任务标记为 `failed`，文件标记为 `parse_failed`。不删除已经上传的文件，让用户可以重试，避免一次队列故障造成上传结果丢失。

### 5.2 用户手动入库

1. 用户在 `parsed`、`index_failed` 或 `indexed` 状态点击“入库/重新入库”。
2. 请求携带分块参数；Embedding 模型不由请求指定。
3. 服务端从知识库读取固定的 `embedding_model_id`，并验证模型可用。
4. 服务层原子地把文件更新为 `index_queued`，同时创建 `job_type=index` 的任务。
5. Worker 将文件改为 `indexing`，读取已生成的 Markdown 并执行分块和向量化。
6. 首次入库成功后将文件改为 `indexed`，同时更新 `chunk_count` 和 `token_count`。
7. 失败时将文件改为 `index_failed`，保留 Markdown，并记录错误信息。

重新入库时，Worker 应先生成完整的新分块结果，再在数据库事务中替换该文件的旧分块，避免处理中途留下新旧数据混合。由于本阶段不实现向量版本热切换，替换事务应尽量短，并在失败时回滚。

### 5.3 重试

- `parse_failed`：创建新的 `parse` 任务，不复用已经终止的任务记录。
- `index_failed`：创建新的 `index` 任务，复用现有 Markdown，不重新解析。
- `indexed`：允许重新入库，用于调整分块参数；仍使用知识库固定的 Embedding 模型。

每次重试创建新的 `job_id`，以保留任务历史和错误记录。

### 5.4 删除

- `parsing`、`indexing` 状态默认禁止删除，避免 Worker 与资源删除竞态。
- 排队状态可先将任务标记为 `cancelled`，再删除文件。
- 删除已入库文件时，必须同时删除该文件的 chunks、向量数据、Markdown 派生文件和原文件。
- 删除操作应具备幂等性；对象存储中的文件已经不存在时不视为失败。

## 6. API 设计

### 6.1 上传

```http
POST /api/v1/knowledge/files/upload?kb_id={kb_id}&parent_id={parent_id}
Content-Type: multipart/form-data
```

响应返回文件和解析任务：

```json
{
  "file_id": "file-id",
  "status": "parse_queued",
  "processing_job_id": "parse-job-id"
}
```

### 6.2 单文件入库

```http
POST /api/v1/knowledge/databases/{kb_id}/documents/{file_id}/index
Content-Type: application/json
```

```json
{
  "chunk_size": 500,
  "chunk_overlap": 50
}
```

响应：

```json
{
  "file_id": "file-id",
  "status": "index_queued",
  "processing_job_id": "index-job-id"
}
```

### 6.3 批量入库

```http
POST /api/v1/knowledge/databases/{kb_id}/documents/index
Content-Type: application/json
```

```json
{
  "file_ids": ["file-1", "file-2"],
  "chunk_size": 500,
  "chunk_overlap": 50
}
```

批量接口逐个原子抢占文件。响应必须分别列出已创建任务和失败项，单个文件失败不回滚其他文件。

### 6.4 重试解析

```http
POST /api/v1/knowledge/databases/{kb_id}/documents/{file_id}/parse
```

仅允许 `parse_failed` 状态调用。

### 6.5 任务查询

保留现有任务查询和轮询接口。任务响应增加：

```json
{
  "job_id": "job-id",
  "job_type": "parse",
  "status": "running"
}
```

## 7. Worker 拆分

保留一套 Redis Stream 和一个任务表，但 Worker 根据 `job_type` 分派：

```text
DocumentWorker
├── processParseJob
└── processIndexJob
```

`processParseJob` 只负责：

```text
读取原文件 -> Parser -> 上传 normalized.md -> parsed
```

`processIndexJob` 只负责：

```text
读取 normalized.md -> Chunker -> Embedding -> 持久化 chunks -> indexed
```

任何一个处理器都不得顺带执行另一个阶段。这样可以确保上传和解析不会触发 Embedding 消耗。

## 8. 前端展示与操作

前端统一展示以下文案：

| 后端状态 | 展示文案 | 主操作 |
| --- | --- | --- |
| `uploaded`、`parse_queued` | 等待解析 | 无 |
| `parsing` | 解析中 | 无 |
| `parsed` | 待入库 | 入库 |
| `parse_failed` | 解析失败 | 重试解析 |
| `index_queued` | 等待入库 | 无 |
| `indexing` | 入库中 | 无 |
| `indexed` | 已入库 | 重新入库 |
| `index_failed` | 入库失败 | 重试入库 |

用户点击入库后，前端使用返回的 `processing_job_id` 轮询任务状态，同时定期刷新文件记录。文件状态是按钮权限的最终依据，任务状态只用于展示本次执行进度。

前端移除普通上传流程中的“自动入库”开关，避免用户误以为上传会产生向量。

## 9. 并发、幂等与恢复

1. 数据库增加部分唯一索引或等价约束，保证同一文件、同一 `job_type` 同时最多存在一个 `pending/running` 任务。
2. 文件状态抢占成功后才能创建任务；重复点击返回现有活动任务或明确的冲突响应。
3. Redis 消息至少一次投递，Worker 依靠任务状态和 `worker_id` 租约保证重复消息不会重复执行。
4. Worker 崩溃后沿用现有租约回收机制，将超时的 `running` 任务重新变为可领取状态。
5. 对 `index` 任务，向量替换必须以文件为单位保证事务性，禁止部分新分块覆盖旧结果。
6. 文件状态与任务终态更新失败时不得 ACK Redis 消息，以便恢复流程重新处理。

## 10. Embedding 模型约束

1. 创建知识库时必须绑定有效的 Embedding 模型。
2. 入库请求不得接收 `embedding_model_id`、模型名称或模型连接参数。
3. Index Worker 根据 `job.KBID` 查询知识库，并使用其绑定模型。
4. 更新知识库接口忽略或拒绝修改 `embedding_model_id`；推荐返回明确的业务错误。
5. 文件的处理参数中可以记录实际使用的模型 ID 和模型标识作为审计快照，但该值由服务端写入，用户不可修改。

## 11. 错误处理

错误信息分为稳定错误码和用户可读信息：

- `PARSER_FAILED`
- `PARSED_CONTENT_NOT_FOUND`
- `INVALID_CHUNK_PARAMS`
- `EMBEDDING_MODEL_UNAVAILABLE`
- `EMBEDDING_REQUEST_FAILED`
- `VECTOR_WRITE_FAILED`
- `INVALID_FILE_STATE`
- `ACTIVE_JOB_EXISTS`
- `QUEUE_ENQUEUE_FAILED`

外部服务返回的敏感连接信息不得写入 `error_message` 或返回前端；完整错误记录在服务端日志中。

## 12. 测试范围

### 服务层

- 上传成功后只创建 `parse` 任务，不创建 `index` 任务。
- `parsed` 文件可以创建入库任务。
- 非法状态不能创建入库任务。
- 连续点击入库只产生一个活动任务。
- 入库请求无法覆盖知识库绑定的 Embedding 模型。
- 更新知识库时不能修改 Embedding 模型。

### Worker

- Parse Worker 成功后停在 `parsed`，且不调用 Indexer。
- Parse Worker 失败后进入 `parse_failed`。
- Index Worker 只读取 Markdown，不重新解析原文件。
- Index Worker 成功后进入 `indexed` 并更新统计字段。
- Index Worker 失败后进入 `index_failed`，Markdown 保持可用。
- 重复消费同一消息不会重复写入 chunks。

### API

- 单文件入库、批量入库和重试解析的成功、冲突与非法状态响应。
- 任务响应包含正确的 `job_type`。
- 跨知识库的文件 ID 不可操作。

### 前端

- 每个规范状态映射到正确文案和按钮。
- 排队和运行状态禁止重复触发。
- 解析失败只显示“重试解析”，入库失败只显示“重试入库”。
- 普通上传界面不再提供自动入库开关。

## 13. 迁移与兼容

现有状态按以下规则迁移：

| 旧状态 | 新状态 |
| --- | --- |
| `uploaded` | `parse_queued` 或保留 `uploaded` 后由补偿任务投递 |
| `processing`、`parsing` | 根据活动任务修正为 `parsing` |
| `ready`、`parsed` | `parsed` |
| `done`、`indexed` | `indexed` |
| `failed`、`error_parsing` | `parse_failed` |
| `index_failed`、`error_indexing` | `index_failed` |
| `indexing` | 根据活动任务保留 `indexing`，无活动任务则改为 `index_failed` |

上线顺序应保证后端先兼容新旧状态，随后迁移数据，最后简化前端旧状态别名。迁移完成并观察稳定后再删除兼容映射。

## 14. 验收标准

1. 上传一个文件后，Embedding 服务调用次数保持为零。
2. 文件自动解析成功并显示“待入库”，Markdown 可以预览。
3. 只有点击“入库”后才调用 Embedding 并生成 chunks。
4. 入库成功后文件显示“已入库”并可被 RAG 检索。
5. 解析和入库失败分别展示正确错误与重试入口。
6. 同一文件连续点击不会产生并发重复任务或重复 chunks。
7. 用户无法通过任何现有知识库更新接口修改 Embedding 模型。
