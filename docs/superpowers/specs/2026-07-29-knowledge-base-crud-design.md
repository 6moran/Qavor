# 知识库基础 CRUD 设计

## 目标

实现与 `docs/Restful API.openapi.json` 一致的知识库基础 CRUD 和文件上传/基础 CRUD。接口要求 JWT 登录，但首版不按用户归属过滤数据。

## 范围

实现下列端点：

- `GET /api/v1/knowledge/databases`
- `POST /api/v1/knowledge/databases`
- `GET /api/v1/knowledge/databases/{kb_id}`
- `PUT /api/v1/knowledge/databases/{kb_id}`
- `DELETE /api/v1/knowledge/databases/{kb_id}`
- `POST /api/v1/knowledge/files/upload?kb_id={kb_id}`
- `GET /api/v1/knowledge/databases/{kb_id}/documents`
- `GET /api/v1/knowledge/databases/{kb_id}/documents/{doc_id}`
- `DELETE /api/v1/knowledge/databases/{kb_id}/documents/{doc_id}`

不实现文件夹、解析、分块、索引、检索、统计和外部知识库相关接口。

## 架构

新增 `internal/api/v1/knowledge` Controller、`KnowledgeBaseService`、`KnowledgeFileService`、`KnowledgeBaseRepository` 和 `KnowledgeFileRepository`。Controller 负责 Gin 参数绑定、调用 Service 和统一响应；Service 负责 UUID 型标识生成、DTO/实体转换与 MinIO/数据库的一致性；Repository 封装 GORM 的增删改查。

App 在依赖注入阶段创建 Repository 和 Service，Router 注册 Controller。所有路由放在现有 `/api/v1` 路由组下，并以 `middleware.Auth()` 保护。

## 数据与行为

- 创建请求使用已有 `CreateKnowledgeBaseRequest`；服务端生成 `kb_id`，并将当前登录用户名写入 `CreatedBy`。
- 列表支持 `page`、`page_size`、`keyword`、`kb_type`；未提供分页参数时采用稳定默认值。
- 读取、更新、删除均以 `kb_id` 定位，未找到时返回项目统一的 not-found 业务响应。
- 更新使用已有 `UpdateKnowledgeBaseRequest`，仅更新请求中提供的字段。
- 删除为物理删除，因为当前实体未定义软删除字段；删除知识库时不级联处理关联文件或分块。
- 首版不使用 `CreatedBy` 作为过滤条件，但保留创建记录以支持未来的归属隔离。
- 上传端点要求 `kb_id`，接收 multipart 的 `file` 字段。先验证知识库存在，再复用 `pkg/minio` 的真实 MIME 类型和大小校验上传到 `knowledge/{kb_id}`，成功后创建状态为 `uploaded` 的 `KnowledgeFile`。
- 文件列表仅支持 `page`、`page_size` 和 `status`；不实现目录、递归和路径前缀筛选。
- 文件详情和删除均同时按 `kb_id`、`file_id` 定位，避免跨知识库误操作。
- 删除文件时先删除 MinIO 的 `Path`，再物理删除 `KnowledgeFile`。对象已不存在时仍删除记录；其他对象删除失败则保留记录并返回错误。
- 删除知识库不会自动删除关联文件或对象，避免在未实现级联策略前执行大范围删除。

## 错误与测试

Controller 对路径和 JSON 参数错误返回既有 `BadRequest`；Service/Repository 的查询、更新、删除缺失资源返回既有 `NotFound`；数据库异常通过既有业务错误响应返回。

测试按 TDD 进行：先验证知识库 Service 的创建、分页筛选、详情、更新和删除，再验证文件 Service 的上传成功后的入库、上传失败不入库、列表、详情、MinIO 同步删除和删除失败保留记录。随后覆盖 Controller 的 JWT 保护、请求校验与统一成功/失败响应。最终运行 `go test ./...`。
