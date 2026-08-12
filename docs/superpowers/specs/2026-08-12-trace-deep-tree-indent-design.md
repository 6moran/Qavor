# Trace 深树缩进显示不全修复设计

## 背景

`/traces/:trace_id` 详情页（TraceDetailPanel.vue）在 Span 数量多、嵌套深的 Trace 上出现两个问题：

1. **默认全部展开**：加载数据后把所有 Span 节点加入展开集合，深树整体展开；
2. **缩进无上限且双重叠加**：调用树行 `paddingLeft = depth * 18 + 8`，子容器 `.tree-children { margin-left: 18px }` 嵌套递进，每层实际 36px；时间轴 `rowDepth * 14`。30 层深度时缩进约 1080px，节点内容被推出面板可视区，出现显示不全。

与 2026-08-09-trace-tree-frontend-design.md 第 5 节「默认展开根节点和前两层 + 异常祖先路径」的要求不符：正确实现 `buildDefaultExpandedSpanIds` 已存在且有单测，但组件未接线。

## 方案

### 1. 默认浅展开

`TraceDetailPanel.vue` 的 `loadDetail()` 中：

```js
// 改前
collectTreeSpanIds(buildTraceTree(spans.value)).forEach(id => expandedBranches.add(id))
// 改后
buildDefaultExpandedSpanIds(treeRoots.value).forEach(id => expandedBranches.add(id))
```

- 默认展开深度 0、1 的有子节点节点，以及 error/timeout/cancelled/interrupted Span 的完整祖先路径；
- `全部展开` / `全部收起` 按钮行为不变，用户仍可手动全展开；
- `collectTreeSpanIds` 仍在 `expandAll` 中使用，import 保留。

### 2. 缩进压缩

`traceViewModel.js` 新增纯函数：

```js
/**
 * 计算指定深度的压缩缩进像素值。
 * 前 compactAfter 层每层 step px，之后每层 compactStep px，
 * 避免深层 Span 把内容推出可视区。
 */
export function compressedIndent(depth, step, compactStep, compactAfter) {
  const full = Math.min(Math.max(depth, 0), compactAfter) * step
  const extra = Math.max(depth - compactAfter, 0) * compactStep
  return full + extra
}
```

| 位置 | 公式 | 30 层 | 50 层 |
|---|---|---|---|
| 调用树行 | `compressedIndent(depth, 18, 6, 8) + 8` | 276px | 396px |
| 时间轴 | `compressedIndent(rowDepth(row), 14, 6, 10)` | 260px | 380px |

配套修改：

- `.tree-children` 移除 `margin-left: 18px`（层级缩进统一由行 padding 承担，消除双重建 36px/层），保留 `border-left` 竖线与背景；
- 调用树行 `paddingLeft` 改用压缩公式；
- 时间轴 `paddingLeft` 改用压缩公式。

### 3. 横向滚动兜底

`.tree-node-row` 增加 `min-width: max-content`：

- 常规宽度下行宽 ≤ 容器宽，行为与现在一致（名字 ellipsis）；
- 缩进或名字超宽时行自然撑开，触发 `.tree-body` 既有 `overflow: auto` 出现横向滚动条；
- 时间轴保持现有 grid 布局，不强制横向滚动。

### 4. 测试

- `traceViewModel.test.js`：新增 `compressedIndent` 用例（depth=0/8/9/30，负值、step/compactStep/compactAfter 参数）；
- `traceListEntry.test.js`：
  - 默认浅展开：组件不再使用 `collectTreeSpanIds(buildTraceTree(spans.value))` 全量展开，且使用 `buildDefaultExpandedSpanIds`；
  - 压缩缩进：调用树行与时间轴均使用 `compressedIndent`，树行有 `min-width:max-content` 滚动兜底。

### 5. 不改的部分

时间轴 grid 布局、`.tree-children` 竖线与背景、`expandAll`/`collapseAll`、树递归渲染结构、时间轴名字 ellipsis。

## 验收标准

1. 深 Trace 打开后默认只展开前两层与异常路径，行数大幅下降；
2. 任意深度下调用树与时间轴缩进有界（≤ 约 400px），内容不被推出可视区；
3. 超宽内容出现横向滚动条，可滚动查看；
4. 现有单元测试全部通过，新增用例通过。
