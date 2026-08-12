<template>
  <div class="file-tree-wrapper" :class="treeClass">
    <a-tree
      :selected-keys="selectedKeys"
      :expanded-keys="expandedKeys"
      :tree-data="treeData"
      :show-icon="showIcon"
      :block-node="blockNode"
      :show-line="showLine"
      class="custom-file-tree"
      @update:selected-keys="handleSelectedUpdate"
      @update:expanded-keys="handleExpandedUpdate"
      @select="handleSelect"
    >
      <!-- Custom Icon Slot -->
      <template #icon="{ data, expanded }">
        <slot name="icon" :node="data" :expanded="expanded">
          <template v-if="data.isLeaf">
            <FileTypeIcon v-if="useFileIcons" :name="data.key" :size="16" />
            <FileText v-else :size="16" class="file-icon" />
          </template>
          <template v-else>
            <span
              v-if="isNodeLoading(data.key)"
              class="folder-loading-icon"
              aria-label="正在加载"
            ></span>
            <FileTypeIcon v-else is-dir :size="18" />
          </template>
        </slot>
      </template>

      <!-- Custom Title Slot -->
      <template #title="{ data }">
        <div class="tree-node-wrapper" @click="handleNodeClick(data)">
          <div class="tree-node-content">
            <slot name="title" :node="data">
              <span class="node-title-text" :title="data.title">{{ data.title }}</span>
            </slot>
          </div>
          <div class="node-actions" @click.stop v-if="$slots.actions">
            <slot name="actions" :node="data"></slot>
          </div>
        </div>
      </template>
    </a-tree>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { FileText } from 'lucide-vue-next'
import FileTypeIcon from '@/components/common/FileTypeIcon.vue'

const props = defineProps({
  treeData: {
    type: Array,
    required: true,
    default: () => []
  },
  loadData: {
    type: Function,
    default: undefined
  },
  selectedKeys: {
    type: Array,
    default: () => []
  },
  expandedKeys: {
    type: Array,
    default: () => []
  },
  showIcon: {
    type: Boolean,
    default: true
  },
  blockNode: {
    type: Boolean,
    default: true
  },
  showLine: {
    type: Boolean,
    default: false
  },
  treeClass: {
    type: String,
    default: ''
  },
  useFileIcons: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits([
  'update:selectedKeys',
  'update:expandedKeys',
  'select'
])

const loadingKeys = ref(new Set())

const setNodeLoading = (key, isLoading) => {
  const nextLoadingKeys = new Set(loadingKeys.value)
  if (isLoading) {
    nextLoadingKeys.add(key)
  } else {
    nextLoadingKeys.delete(key)
  }
  loadingKeys.value = nextLoadingKeys
}

const isNodeLoading = (key) => loadingKeys.value.has(key)

// 手动触发子目录加载（本项目隐藏了 switcher，只能靠点击标题展开，
// 因此不走 a-tree 原生的 :load-data 机制，改由这里显式调用并驱动 loading 图标）。
const triggerFolderLoad = async (key, data) => {
  if (!props.loadData) return
  setNodeLoading(key, true)
  try {
    await props.loadData({ key, ...data })
  } finally {
    setNodeLoading(key, false)
  }
}

const handleSelectedUpdate = (keys) => {
  emit('update:selectedKeys', keys)
}

const handleExpandedUpdate = (keys) => {
  emit('update:expandedKeys', keys)
}

const handleSelect = (selectedKeys, info) => {
  emit('select', selectedKeys, info)
}

const handleNodeClick = (data) => {
  const isFolder = data.isLeaf === false || (data.children && Array.isArray(data.children))
  if (!isFolder) return

  const key = data.key
  const newExpandedKeys = [...props.expandedKeys]
  const index = newExpandedKeys.indexOf(key)

  if (index > -1) {
    newExpandedKeys.splice(index, 1)
  } else {
    newExpandedKeys.push(key)
  }

  emit('update:expandedKeys', newExpandedKeys)

  // 展开且子节点尚未加载时，主动拉取子目录。手动切换 expandedKeys 走受控 watch
  // 路径、不会触发 a-tree 内部 onNodeLoad，必须显式调用；已加载过的文件夹复用
  // 缓存，不再重复请求。
  if (index === -1) {
    const children = data.children
    const notLoaded = !children || (Array.isArray(children) && children.length === 0)
    if (props.loadData && notLoaded) {
      triggerFolderLoad(key, data)
    }
  }
}
</script>

<style scoped lang="less">
.file-tree-wrapper {
  width: 100%;

  /* 统一节点容器 */
  :deep(.ant-tree-treenode) {
    display: flex;
    align-items: center;
    width: 100%;
    padding: 0;
    height: 32px;

    /* 隐藏切换器 */
    .ant-tree-switcher {
      display: none;
    }

    /* 缩进单元 */
    .ant-tree-indent {
      display: flex;
      align-items: center;
      align-self: stretch;
      &-unit {
        width: 14px;
      }
    }

    /* 内容区域容器 */
    .ant-tree-node-content-wrapper {
      display: flex;
      align-items: center;
      flex: 1;
      min-width: 0;
      height: 32px;
      line-height: 32px;
      padding: 0 4px;
      border-radius: 4px;
      transition: background-color 0.2s;

      /* 图标容器 */
      .ant-tree-iconEle {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 24px;
        height: 100%;
        margin-right: 4px;
        flex-shrink: 0;

        .anticon {
          display: flex;
          align-items: center;
        }
      }

      /* 标题文字容器 */
      .ant-tree-title {
        display: flex;
        align-items: center;
        flex: 1;
        min-width: 0;
        height: 100%;
      }

      &:hover {
        background-color: var(--gray-50);
      }

      &.ant-tree-node-selected {
        background-color: var(--gray-100);
      }
    }
  }
}

.tree-node-wrapper {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  min-width: 0;
}

.tree-node-content {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}

.node-title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: var(--gray-800);
}

.folder-icon {
  color: var(--main-500);
  fill: var(--main-500);
  fill-opacity: 0.2;
}

.folder-loading-icon {
  width: 15px;
  height: 15px;
  border: 2px solid var(--gray-200);
  border-top-color: var(--main-500);
  border-radius: 50%;
  animation: file-tree-folder-loading 0.8s linear infinite;
}

@keyframes file-tree-folder-loading {
  to {
    transform: rotate(360deg);
  }
}

.node-actions {
  display: none;
  align-items: center;
  padding-left: 8px;
  flex-shrink: 0;
}

.ant-tree-node-content-wrapper:hover .node-actions {
  display: flex;
}
</style>
