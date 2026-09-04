# Qavor 文档解析管道增强设计

## 1. 背景

Qavor 当前已经具备从知识文件上传、异步解析、保存统一 Markdown、分块到索引的基本链路：TXT/Markdown 由 Go 直接归一化，DOCX/PPTX/XLSX 交给 Docling，PDF 与图片交给 RapidOCR 或通用 OCR API。

当前实现仍有四个需要集中解决的问题：

1. 缺少覆盖真实格式的可重复测试文件和端到端验收；
2. 数字版 PDF 也被逐页渲染为图片后 OCR，丢失原生结构并产生额外成本；
3. 解析结果只有较浅的文本归一化，缺少统一但安全的格式清理；
4. 每次二进制文档解析都会启动一个新的 Python 进程，Docling 和 OCR 初始化无法跨任务复用。

本设计在现有 Redis Stream、DocumentWorker、DocumentParser 和 `normalized.md` 链路上增量演进，不拆分独立解析服务。

## 2. 目标

本次实现包含以下四项：

1. 增加真实、无隐私、可提交到 Git 的测试文件，并建立单元、进程池、Python 集成和 Go 全链路测试；
2. PDF 优先通过 Docling 提取数字版内容，Docling 失败或没有有效正文时整份降级到现有 OCR 路径；
3. 增加确定性、保守的 DocumentCleaner，只做格式归一化和明确的解析残片清理；
4. 由 Go 管理固定大小的常驻 Python 子进程池，并让 DocumentWorker 以受控并发消费文档任务。

## 3. 非目标

本次明确不做：

- 单文档解析超时、超时错误码、超时强制终止或超时自动重试；
- 基于 LLM 的内容改写、摘要或语义清洗；
- 自动删除 PDF 页眉、页脚、页码或模糊重复段落；
- 一份 PDF 内按页混合 Docling 与 OCR；
- 独立 HTTP/gRPC Python 解析服务；
- MinerU、PP-StructureV3 等新解析后端；
- 上传后自动索引策略调整；
- RAG 检索、Rerank 或回答生成策略调整。

不设置任务超时意味着：如果底层 Docling/OCR 永久阻塞，对应 Python Worker 会一直被占用，需通过重启 Qavor 恢复。该限制作为后续演进项保留。

## 4. 架构选择

采用“Go 管理常驻 Python 子进程池”方案。

未采用独立 HTTP 服务，因为它会引入额外端口、部署单元和服务治理成本；未采用 Python 主进程再管理 multiprocessing 子进程，因为 Windows 下子进程取消、故障隔离和模型内存控制更复杂。

目标结构：

```text
Redis Stream
    ↓
DocumentWorker 并发消费者（固定数量）
    ↓
ingestion.Parser
    ├─ TXT / Markdown：Go 直接解析
    └─ Office / PDF / 图片：PythonWorkerPool
                              ├─ PythonWorker 1
                              └─ PythonWorker 2
    ↓
DocumentCleaner
    ↓
normalized.md
    ↓
现有分块与索引任务
```

进程池默认大小为 2，可通过配置调整。单个 Python Worker 同一时间只处理一份文档。

## 5. Go 侧组件

### 5.1 PythonWorker

`PythonWorker` 代表一个常驻 Python 子进程，职责包括：

- 以 `--serve-stdio` 模式启动 Python 解析运行时；
- 建立 stdin、stdout 和 stderr 管道；
- 等待一条结构化 ready 握手，确认协议版本和后端能力；
- 一次发送一个请求并读取对应响应；
- 校验协议版本、请求 ID 和必需字段；
- 单独持续读取 stderr 并写入 Qavor 日志，防止子进程因 stderr 缓冲区写满而阻塞；
- 记录已完成任务数，达到 `max_tasks_per_worker` 后在空闲时轮换；
- 在应用关闭、进程崩溃或协议损坏时退出并释放管道。

Windows 下使用平台专用进程属性隐藏控制台窗口；Linux/macOS 使用普通后台子进程。用户使用 Qavor 时不会看到或操作命令行窗口。

### 5.2 PythonWorkerPool

`PythonWorkerPool` 继续满足现有 `DocumentParser` 接口，使 Worker 和业务服务不感知进程协议。

职责包括：

- 按配置启动固定数量的 PythonWorker；
- 为每次二进制文档解析借出一个空闲 Worker；
- 正常完成后归还 Worker；
- Worker 崩溃或协议损坏时丢弃并补充新 Worker；
- 进程池关闭后拒绝新请求并终止所有后台 Python 进程；
- 暴露当前 Worker 数量和解析后端能力，供健康接口使用。

TXT 和 Markdown 继续由 Go 直接处理，不占用 PythonWorker。

### 5.3 DocumentWorker 并发

DocumentWorker 从单个串行消费循环调整为固定数量的正常消费循环，每个循环拥有唯一 Redis consumer ID。默认消费者数量与 Python 进程池大小一致。

Pending 恢复仍只保留一个协调循环，避免多个恢复器重复扫描和抢占消息。所有正常任务和恢复任务共享同一个 PythonWorkerPool，由池容量形成统一背压。

TXT/Markdown 虽不占用 Python 进程，但仍受 DocumentWorker 固定并发约束，避免多个大文件同时读入内存。

## 6. Python 常驻运行时与协议

Python 入口同时保留两种模式：

- `--input <path>`：单文件 CLI 模式，方便本地诊断；
- `--serve-stdio`：常驻 JSONL 模式，供 Go 进程池调用。

JSONL 表示 stdin/stdout 中一行对应一个完整 JSON 对象。stdout 只允许输出协议消息，普通日志、警告和异常详情必须写到 stderr。

### 6.1 握手

Python 启动后先检查各后端能否导入，但不提前加载 OCR 模型。不同后端独立检查，Docling 不可用不能阻断图片 OCR 能力。

示例：

```json
{"type":"ready","version":1,"capabilities":{"docling":true,"rapidocr":true,"api_ocr":true}}
```

### 6.2 解析请求

```json
{"type":"parse","version":1,"request_id":"uuid","input_path":"C:/temp/a.pdf","filename":"a.pdf","ocr_engine":"rapidocr"}
```

临时文件仍由 Go 为单次 Parse 调用创建并在响应处理、图片上传回填完成后清理。Python 只读取本地路径，不接触 MinIO 凭证。

### 6.3 成功响应

```json
{"type":"result","version":1,"request_id":"uuid","ok":true,"result":{"markdown":"...","picture_paths":[],"pages":[],"metadata":{}}}
```

### 6.4 失败响应

```json
{"type":"result","version":1,"request_id":"uuid","ok":false,"error":{"code":"PARSER_FAILED","message":"文档解析失败"}}
```

协议不向前端暴露本地路径、API Key、完整原文或底层堆栈。详细诊断只进入 stderr 日志。

## 7. PDF 解析策略

所有 PDF 固定先尝试 Docling：

```text
PDF
 ↓
Docling
 ├─ 成功且存在有效可见正文 → 使用 Docling Markdown
 └─ 转换失败或正文为空     → 整份 PDF 降级 OCR
```

“有效正文”采用保守的确定性判定：移除 Markdown 语法和空白后，至少包含一个字母、汉字或数字。此判定只用于识别完全空结果，不对内容质量打分。

本次使用整份 PDF 降级，避免逐页混合导致顺序错乱和内容重复。

元数据至少包含：

```json
{"parser":"docling","fallback":false}
```

或：

```json
{"parser":"rapidocr","fallback":true,"fallback_reason":"docling_empty"}
```

若配置通用 OCR API，降级解析器可以是 `api_ocr`。Docling 和 OCR 都失败时才返回解析失败。

## 8. 确定性内容清洗

DocumentCleaner 位于解析之后、`normalized.md` 保存之前，对所有格式统一执行。

清洗原则是“宁可保留重复内容，也不删除可能有意义的原始知识”。本次允许的操作只有：

- 统一 CRLF/CR 为 LF；
- 删除除换行和制表符以外的异常控制字符；
- 清理行尾空白、文档首尾空白和过多连续空行；
- 删除明确为空的 Markdown 标题；
- 清理明确为空或无源地址的解析器图片占位；
- 清理 Docling/OCR 已知且可精确识别的格式残片；
- 保持输出为可分块的稳定 Markdown。

必须保留：

- 页眉、页脚、页码正文；
- `<!-- page:N -->` 页面边界标记；
- 标题层级、代码块、表格、列表；
- 图片 OCR 描述和对象存储 URL；
- 脚注、引用、版本号、保密说明和免责声明；
- 语义相同但表达不同的段落。

本次不做基于位置的页眉页脚删除，也不做模糊重复检测。

每条清洗规则应独立、确定且可测试。单条规则无法处理输入时跳过该规则并保留原文；解析 Markdown 非空时，清洗器不得使整份文档失败或变为空。清洗器发生降级时在元数据记录 `cleaner_degraded=true`。

## 9. 故障处理

### 9.1 普通解析失败

格式损坏、Docling 与 OCR 都失败或解析结果为空时，返回稳定的 ParserError，沿用现有文件状态机进入 `parse_failed`。普通解析错误不自动重试。

### 9.2 Worker 崩溃

Python 进程退出、管道断开或返回 EOF 时：

1. 当前 Worker 从池中移除；
2. 进程池启动替代 Worker；
3. 当前解析请求在新的 Worker 上内部重试一次；
4. 第二次仍失败则返回 `PARSER_WORKER_CRASHED`。

解析结果写入 `normalized.md` 前没有持久化副作用，因此该一次重试不会产生重复数据。

### 9.3 协议损坏

非法 JSON、协议版本不匹配、请求 ID 不一致或缺少必需字段统一视为 `PARSER_PROTOCOL_ERROR`。对应进程状态不可信，必须销毁并重建；当前请求最多内部重试一次。

### 9.4 清洗降级

清洗规则异常时保留解析器原始 Markdown，不让整份文档失败。只有解析器原始输出本身没有有效正文时才返回 `PARSER_EMPTY_CONTENT`。

### 9.5 状态与写入

保持现有状态机：

```text
parse_queued → parsing → parsed / parse_failed
```

只有解析、图片回填、清洗和 Markdown 上传全部成功后才转为 `parsed`。重新解析不能直接覆盖当前对象键：新 Markdown 先上传到包含本次任务 ID 的独立对象键，上传成功后再更新文件记录；旧 Markdown 和旧索引在切换完成前保持可用，切换后的旧对象进入可恢复的延迟清理流程。

## 10. 健康状态与关闭

解析健康接口不再静态宣称本地 OCR 一定可用，而是基于进程池握手汇总：

- `healthy`：可用 Worker 数达到配置值；
- `degraded`：至少一个 Worker 可用，但数量不足或部分解析能力缺失；
- `unavailable`：没有可用 Worker。

健康结果分别标明 Docling、RapidOCR 和通用 OCR API 能力，不泄露路径或凭证。

Qavor 关闭时停止领取新解析任务，取消消费上下文，关闭池中的 stdin，并确保所有后台 Python 进程退出。Windows 下不得留下可见控制台窗口或孤儿 Python 进程。

## 11. 配置

新增配置：

```yaml
document_parser:
  python_path: python
  pool_size: 2
  max_tasks_per_worker: 100
```

- `pool_size` 必须大于 0；
- `max_tasks_per_worker` 用于在任务之间轮换进程，控制长期内存增长；
- 本次不增加任务超时配置。

## 12. 测试文件

在文档解析模块的 `testdata` 下提交小型、无隐私、来源可说明的样本：

- `plain.txt`：换行、控制字符和连续空行；
- `structured.md`：标题、表格、代码块和图片链接；
- `digital.pdf`：原生正文、标题和表格；
- `scanned.pdf`：纯图片扫描页；
- `with-image.docx`：正文和带可识别文字的内嵌图片；
- `slides.pptx`：标题、正文和图片；
- `table.xlsx`：多行多列表格；
- `text-image.png`：中英文、数字；
- `corrupted.pdf`：损坏输入。

二进制样本应尽量小，并在 README 中记录生成方式和预期文本，禁止使用个人文档或来源不明的文件。

## 13. 分层测试

### 13.1 Go 单元测试

- DocumentCleaner 的输入/输出 golden 测试；
- 保证页眉、页脚、页码、代码块、表格和图片 URL 被保留；
- PDF Docling 优先与 OCR fallback 的路由测试；
- JSONL 请求、响应、错误映射和协议校验；
- 进程池借出、归还、关闭和容量控制。

### 13.2 进程池集成测试

- 启动两个轻量测试 Worker；
- 连续解析多个请求时 PID 保持不变，证明进程复用；
- 两个请求可同时占用两个 Worker；
- 子进程崩溃和协议损坏后能够补充新进程并仅重试一次；
- stderr 日志不会污染 stdout；
- 池关闭后不存在残留测试进程；
- Windows 启动和轮换期间不显示控制台窗口。

### 13.3 真实 Python 集成测试

- 提供可复现的依赖约束文件，在全新隔离 Python 环境中安装解析依赖，并确保依赖检查通过；
- 数字 PDF 返回 `parser=docling` 且保留预期结构；
- 扫描 PDF 返回 OCR parser 与 fallback 元数据；
- DOCX/PPTX/XLSX 输出包含预期文本；
- Office 内嵌图片 OCR 描述保留在 Markdown；
- 连续处理多个文件时 Python PID 不变，后端初始化得到复用；
- 损坏文件返回稳定错误协议。

### 13.4 Go 全链路验收

验证：

```text
上传样本 → Redis 任务 → 常驻 Python Worker → 解析/降级 → 清洗
→ normalized.md → 分块 → indexed → 检索命中预期文本
```

检查文件状态、Markdown 内容、解析元数据、图片 URL、分块数量和检索结果。测试必须区分“代码/单元测试通过”和“真实依赖与服务运行通过”。

## 14. 验收标准

本次实现完成需要同时满足：

1. 所有新增和现有 Go 单元测试通过；
2. Python 协议和真实样本集成测试可从空白隔离环境按依赖约束重复安装并通过；
3. 数字 PDF 使用 Docling，扫描 PDF 能自动降级 OCR；
4. 清洗不删除测试样本中的页眉、页脚、页码、代码块、表格和图片 URL；
5. 连续多个二进制文档复用同一批 Python PID；
6. 并发请求数受池容量约束；
7. 崩溃或协议错误的 Worker 被替换，当前请求最多重试一次；
8. Windows 运行期间不弹出 Python 控制台窗口；
9. Qavor 关闭后没有残留由本应用启动的 Python Worker；
10. 真实上传到检索的端到端样本至少覆盖数字 PDF、扫描 PDF、Office 内嵌图片和图片 OCR。

## 15. 实施顺序

建议按以下顺序实施：

1. 添加轻量协议测试 Worker、真实样本和失败基线；
2. 重构 Python 入口为单文件模式与常驻 JSONL 模式；
3. 实现 PythonWorker 和 PythonWorkerPool；
4. 接入 Windows 无窗口进程配置与池生命周期；
5. 增加 DocumentWorker 受控并发；
6. 实现 PDF Docling 优先与整份 OCR fallback；
7. 实现保守的 DocumentCleaner；
8. 完成真实 Python 集成和 Go 全链路验收；
9. 更新架构、配置与开发文档。
