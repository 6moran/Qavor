# Qavor README 完善设计

## 背景

当前根目录 `README.md` 仍将 Qavor 描述为 Go/Gin 后端基础框架，且包含已过时的 Go 版本、依赖版本、用户注册接口和目录结构。实际仓库已经演进为包含 Vue 前端、Agent 编排、RAG 知识库、MCP/Skill、异步任务和链路追踪的完整 AI Agent 平台。

## 目标读者与目标

- 主要读者：首次接触仓库、需要理解并在本地运行项目的开发者。
- 文档语言：中文，技术名词和命令保留英文原名。
- 上手目标：开发者只阅读根 README，即可了解产品边界、准备依赖、完成配置、启动前后端并验证服务。
- 命令环境：主体命令保持跨平台；文件复制等存在差异的操作同时提供 PowerShell 与 Bash 写法。

## 范围

本次只改写根目录 `README.md`。不修改应用代码、配置样例、Makefile、已有专项文档和当前工作区中的其他未提交文件；不新增 Docker、部署脚本、许可证、贡献规范、线上演示地址或未经仓库验证的功能声明。

## README 信息架构

1. 项目标题、现有 Logo、一句话定位和主要技术栈。
2. 项目能力：Agent 对话与管理、模型管理、RAG 知识库、MCP/Skill、工作区、异步运行队列、SSE、链路追踪和数据总览。
3. 系统架构：使用一张 Mermaid 图说明 Vue、Gin、业务模块与 PostgreSQL/pgvector、Redis、MinIO、Python 解析器之间的关系。
4. 快速开始：列出前置环境、克隆仓库、创建 PostgreSQL 数据库及启用 pgvector、复制并修改配置、安装可选 Python 解析依赖、启动后端、启动前端、访问页面并验证健康检查。
5. 配置说明：区分必须配置项、可选依赖及功能降级；说明 YAML 与 `.env` 的职责，不复制完整配置样例。
6. 项目结构：仅展示当前核心目录和职责，避免逐文件列举。
7. 常用开发命令：Go 测试/构建、前端依赖安装/开发/单测/构建、可选 Python 依赖安装。
8. 文档索引：链接架构、API/OpenAPI、开发、模型接入及关键设计文档。
9. 注意事项：说明首次建库与迁移顺序、共享 Redis 消费组的风险，以及 Redis/MinIO/Python 不可用时受影响的功能。

## 事实来源与边界

- Go 版本、依赖与模块名以 `go.mod` 为准。
- 前端包管理器、脚本和技术栈以 `frontend/package.json`、`frontend/vite.config.js` 为准。
- 配置项与覆盖规则以 `configs/config.yaml.example`、`.env.example`、`pkg/config` 为准。
- 启动依赖和降级行为以 `internal/app/app.go` 为准。
- 数据库扩展及迁移说明以实体定义和 `scripts/migrate.sql` 为准。
- 功能与页面入口以实际路由、Service、Agent/RAG/MCP/Skill/Trace 模块和前端路由为准。
- README 不罗列完整 API，避免与 OpenAPI 文档重复并降低漂移风险。

## 快速开始约束

- PostgreSQL 是后端启动的必需依赖；知识库向量字段要求先安装并启用 pgvector 扩展。
- `configs/config.yaml` 和根目录 `.env` 都要从样例创建；当前加载逻辑要求两者存在。
- Redis 与 MinIO 初始化失败时后端仍可启动，但运行队列、异步文档处理、文件上传等能力会受限，因此完整体验仍将它们列为前置依赖。
- Python 依赖仅用于 Office、PDF 和图片解析；纯文本路径及不使用文档解析时可不安装。
- 前端开发服务器默认通过 Vite 将 `/api` 代理到 `http://127.0.0.1:8080`，默认页面地址为 `http://localhost:5173`。
- 登录使用配置中的单实例管理员账号，不再展示已经不存在的注册流程。

## 验收标准

- README 不再出现 MySQL、Go 1.21、Gin 1.9、GORM 1.25、用户注册等过时描述。
- 所有命令、路径、功能和依赖均能在当前仓库中找到依据。
- 快速开始覆盖 PowerShell 与 Bash 的差异操作，且不要求读者自行猜测配置文件、启动顺序或访问地址。
- Markdown 链接和 Mermaid 语法可正常解析，仓库内相对链接全部指向现有文件。
- 运行 Markdown 静态检查或等价的结构检查，并核对 `git diff --check`；若环境允许，再执行与文档命令相关的轻量验证。
