# Git 提交规范

> 本文档定义了 Qavor 项目的 Git 提交信息规范。
> 最后更新：2026-08-25

---

## 1. 提交信息格式

采用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/) 规范：

```
<type>(<scope>): <subject>

[optional body]


[optional footer(s)]
```

### 示例

```bash
# 简单提交
git commit -m "feat(rag): 增加混合检索 RRF 融合"

# 带范围的提交
git commit -m "fix(auth): 修复登录 token 过期未刷新问题"

# 带描述的提交
git commit -m "feat(agent): 增加 Agent 执行取消功能

- 支持用户取消正在执行的 Agent 任务
- 增加任务状态枚举 CANCELLED
- 修改前端按钮交互逻辑

Closes #123"
```

---

## 2. 类型说明

| 类型 | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat(rag): 增加混合检索` |
| `fix` | Bug 修复 | `fix(auth): 修复登录失败` |
| `docs` | 文档更新 | `docs: 更新部署流程文档` |
| `style` | 代码格式（不影响功能） | `style: 格式化 Go 代码` |
| `refactor` | 重构（非新功能、非修复） | `refactor(agent): 重构工具调用逻辑` |
| `perf` | 性能优化 | `perf(rag): 优化向量检索性能` |
| `test` | 测试相关 | `test: 增加 auth 模块单元测试` |
| `build` | 构建系统或外部依赖 | `build: 升级 Go 版本到 1.25` |
| `ci` | CI 配置 | `ci: 添加前端测试流水线` |
| `chore` | 其他杂项 | `chore: 清理无用代码` |
| `revert` | 回滚 | `revert: 回滚 feat(rag)` |

---

## 3. 范围（Scope）

范围是可选的，用于说明提交影响的模块：

| 范围 | 说明 |
|------|------|
| `auth` | 用户认证模块 |
| `agent` | Agent 执行模块 |
| `rag` | RAG 知识库模块 |
| `chat` | 对话模块 |
| `tool` | 工具/MCP 模块 |
| `trace` | 链路追踪模块 |
| `skill` | Skill 系统 |
| `memory` | 记忆系统 |
| `frontend` | 前端通用 |
| `docker` | 容器化相关 |

---

## 4. 主题（Subject）

主题是提交的简短描述：

- 使用中文
- 不超过 50 个字符
- 不加句号
- 使用祈使句（"增加" 而非 "增加了"）

**好的示例：**
```bash
feat(rag): 增加混合检索 RRF 融合
fix(auth): 修复登录 token 过期问题
docs: 更新部署流程文档
```

**不好的示例：**
```bash
feat(rag): 增加了混合检索 RRF 融合功能。  # 太长，有句号
fix bug  # 太模糊
update code  # 太模糊
```

---

## 5. 提交范围约定

### 5.1 代码文件

**只提交代码文件：**
- ✅ `.go` 文件
- ✅ `.vue`、`.js`、`.ts` 文件
- ✅ `.yaml`、`.yml` 配置文件
- ✅ `go.mod`、`go.sum`
- ✅ `package.json`、`pnpm-lock.yaml`

**不提交的文件：**
- ❌ `docs/` 文件夹
- ❌ `frontend/test/` 测试文件
- ❌ `*_test.go` 测试文件
- ❌ 测试数据文件
- ❌ `.env`、`configs/config.yaml`（本地配置）
- ❌ `logs/` 日志文件
- ❌ `node_modules/`

### 5.2 分次提交

一个功能应该分多次提交，每次提交只做一件事：

```bash
# 1. 先提交实体定义
git add internal/model/entity/knowledge_base.go
git commit -m "feat(knowledge): 定义知识库实体结构"

# 2. 再提交 Repository
git add internal/repository/knowledge_base_repository.go
git commit -m "feat(knowledge): 实现知识库数据访问层"

# 3. 再提交 Service
git add internal/service/knowledge_base_service.go
git commit -m "feat(knowledge): 实现知识库业务逻辑"

# 4. 最后提交 Controller 和路由
git add internal/api/v1/knowledge_base/
git commit -m "feat(knowledge): 增加知识库 API 接口"
```

---

## 6. 特殊提交

### 6.1 合并冲突解决

```bash
git commit -m "merge: 解决与 develop 的合并冲突"
```

### 6.2 版本发布

```bash
git commit -m "chore(release): v1.0.0"
```

### 6.3 紧急修复

```bash
git commit -m "hotfix(auth): 紧急修复登录验证漏洞"
```

---

## 7. 检查工具

### 7.1 本地检查

```bash
# 检查最近一次提交
git log -1 --pretty=%B

# 检查最近 5 次提交
git log -5 --pretty="%h %s"
```

### 7.2 自动格式化

项目可以配置 `commitlint` 来自动检查提交信息：

```bash
# 安装（如果项目配置了）
npm install -D @commitlint/cli @commitlint/config-conventional

# 检查提交信息
npx commitlint --from HEAD~1 --to HEAD
```

---

## 8. 常见问题

### Q: 什么时候用 `feat`，什么时候用 `fix`？

- **feat**：新增功能（之前不存在的）
- **fix**：修复问题（之前存在但不正确的）

### Q: scope 必须写吗？

不是必须的，但建议写。scope 能让提交信息更清晰。

### Q: 提交信息写英文还是中文？

建议使用**中文**，与项目保持一致。

### Q: 一个功能太大，怎么拆分提交？

按模块拆分：
```bash
# 实体层
feat(xxx): 定义 xxx 实体结构

# 数据层
feat(xxx): 实现 xxx 数据访问层

# 业务层
feat(xxx): 实现 xxx 业务逻辑

# 接口层
feat(xxx): 增加 xxx API 接口
```

---

## 9. 示例提交历史

```bash
$ git log --oneline -15

a1b2c3d feat(knowledge): 增加知识库文档管理接口
b2c3d4e feat(knowledge): 实现知识库业务逻辑
c3d4e5f feat(knowledge): 实现知识库数据访问层
d4e5f6g feat(knowledge): 定义知识库实体结构
e5f6g7h fix(rag): 修复向量检索结果排序问题
f6g7h8i feat(rag): 增加混合检索 RRF 融合
g7h8i9j refactor(agent): 重构工具调用逻辑
h8i9j0k fix(auth): 修复登录 token 过期未刷新
i9j0k1l feat(auth): 增加用户登出功能
j0k1l2m docs: 更新 API 文档
k1l2m3n test: 增加 auth 模块单元测试
l2m3n4o build: 升级 Go 版本到 1.25
m3n4o5p ci: 添加前端测试流水线
n4o5p6q chore: 清理无用代码
o5p6q7r feat(agent): 增加 Agent 执行取消功能
```

---

## 相关文档

- [PR流程规范](PR流程规范.md)
- [部署流程](部署流程.md)
- [开发指南](DEVELOPMENT.md)