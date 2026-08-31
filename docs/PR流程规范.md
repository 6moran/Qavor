# Pull Request 流程规范

> 本文档定义了 Qavor 项目的 PR（Pull Request）提交、审查和合并流程。
> 最后更新：2026-08-25

---

## 1. 分支策略

```
main          ← 生产分支，只接受 develop 的合并或紧急修复
  ↑
develop       ← 开发主分支，功能集成在此
  ↑
feature/*     ← 功能分支，从 develop 创建
fix/*         ← 修复分支，从 develop 或 main 创建
```

| 分支类型 | 命名规范 | 说明 |
|---------|---------|------|
| `feature/*` | `feature/模块名-功能描述` | 新功能开发 |
| `fix/*` | `fix/模块名-问题描述` | Bug 修复 |
| `hotfix/*` | `hotfix/紧急问题描述` | 生产环境紧急修复 |
| `refactor/*` | `refactor/模块名-重构描述` | 代码重构 |

---

## 2. PR 提交流程

### 2.1 创建功能分支

```bash
# 从 develop 拉取最新代码
git checkout develop
git pull origin develop

# 创建功能分支
git checkout -b feature/模块名-功能描述

# 开发完成后推送
git push origin feature/模块名-功能描述
```

### 2.2 提交前检查清单

- [ ] 代码遵循项目代码规范（见 `docs/DEVELOPMENT.md`）
- [ ] 后端测试全部通过：`go test ./...`
- [ ] 前端测试全部通过：`cd frontend && pnpm test:unit`
- [ ] 代码已格式化：`go fmt ./...`
- [ ] 静态检查通过：`go vet ./...`
- [ ] 只提交代码文件，不提交 `docs/`、测试文件
- [ ] 提交信息符合规范（见 [Git提交规范](Git提交规范.md)）

### 2.3 创建 PR

1. 登录 GitHub，进入仓库页面
2. 点击 "Compare & pull request" 按钮
3. 填写 PR 信息：

**标题格式：**
```
feat(模块): 功能描述
```

**描述模板：**
```markdown
## 变更说明
<!-- 简要描述本次变更的内容 -->

## 关联 Issue
<!-- 如果有，填写关联的 Issue 编号 -->
Closes #

## 测试情况
- [ ] 后端测试通过
- [ ] 前端测试通过
- [ ] 本地功能验证

## 截图/录屏
<!-- 如果涉及 UI 变更，提供截图 -->
```

### 2.4 PR 目标分支

| PR 类型 | 目标分支 |
|---------|---------|
| 新功能 | `develop` |
| Bug 修复 | `develop` |
| 紧急修复 | `main` |

---

## 3. PR 审查流程

### 3.1 审查要求

- **至少 1 人审查通过** 才能合并
- 审查重点：
  - 代码逻辑是否正确
  - 是否遵循项目规范
  - 是否有潜在的 Bug 或安全问题
  - 测试是否充分

### 3.2 审查操作

```bash
# 本地审查 PR 代码
git fetch origin
git checkout origin/feature/模块名-功能描述
```

### 3.3 审查反馈

- **通过**：在 PR 页面点击 "Approve"
- **需要修改**：在 PR 页面点击 "Request changes" 并说明原因
- **评论**：在 PR 页面添加评论讨论

### 3.4 常见审查问题

| 问题类型 | 说明 |
|---------|------|
| 代码规范 | 命名、注释、格式不符合规范 |
| 逻辑错误 | 代码逻辑有问题 |
| 测试不足 | 缺少必要的单元测试 |
| 文档缺失 | 新功能未更新相关文档 |
| 提交信息 | 不符合 Conventional Commits 规范 |

---

## 4. 合并策略

### 4.1 合并方式

项目使用 **Squash and merge** 策略，保持主分支历史清晰。

### 4.2 合并操作

1. PR 审查通过后，点击 "Squash and merge"
2. 确认提交信息（会自动合并为一条）
3. 删除功能分支（勾选 "Delete branch"）

### 4.3 合并后清理

```bash
# 本地删除已合并的分支
git checkout develop
git pull origin develop
git branch -d feature/模块名-功能描述
```

---

## 5. 解决冲突

### 5.1 检测冲突

```bash
# 更新 develop 分支
git fetch origin develop

# 在功能分支上合并 develop
git checkout feature/模块名-功能描述
git merge origin/develop
```

### 5.2 解决冲突

```bash
# 查看冲突文件
git status

# 手动解决冲突后
git add <冲突文件>
git commit -m "merge: 解决与 develop 的合并冲突"
git push origin feature/模块名-功能描述
```

---

## 6. 特殊情况处理

### 6.1 WIP（Work in Progress）PR

- 创建 PR 时在标题前加 `[WIP]`：`[WIP] feat(模块): 功能描述`
- 表示 PR 尚未完成，不需要审查

### 6.2 紧急修复（Hotfix）

```bash
# 从 main 创建 hotfix 分支
git checkout main
git pull origin main
git checkout -b hotfix/紧急问题描述

# 修复后推送
git push origin hotfix/紧急问题描述
```

- PR 目标分支选择 `main`
- 合并后需要同步到 `develop`：

```bash
git checkout develop
git merge main
git push origin develop
```

---

## 7. 最佳实践

1. **小批量提交**：PR 变更尽量控制在 200 行以内，便于审查
2. **单一职责**：一个 PR 只做一件事
3. **及时响应**：收到审查反馈后尽快修改
4. **保持更新**：定期 rebase develop 分支，避免冲突积累
5. **写好描述**：让审查者快速理解变更内容

---

## 相关文档

- [Git提交规范](Git提交规范.md)
- [部署流程](部署流程.md)
- [开发指南](DEVELOPMENT.md)