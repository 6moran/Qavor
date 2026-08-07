<template>
  <section class="conversation-nav-section" :class="{ collapsed }">
    <div v-if="showHistory && !collapsed" class="history-panel">
      <div class="history-label" @click="listCollapsed = !listCollapsed">
        <span>最近</span>
        <ChevronDown :size="14" class="collapse-icon" :class="{ collapsed: listCollapsed }" />
      </div>
      <div
        v-show="!listCollapsed"
        ref="listRef"
        class="conversation-list"
        @scroll="handleListScroll"
      >
        <template v-if="sortedChats.length > 0">
          <div
            v-for="chat in sortedChats"
            :key="chat.id"
            type="button"
            class="conversation-item"
            :class="{ active: currentChatId === chat.id }"
            @click="$emit('select-chat', chat.id)"
            @dblclick.stop="renameChat(chat.id)"
            @click.middle="$emit('delete-chat', chat.id)"
          >
            <span class="conversation-title">{{ chat.title || '新的对话' }}</span>
            <span class="actions-mask"></span>
            <span class="conversation-actions" @click.stop @dblclick.stop>
              <a-dropdown :trigger="['click']">
                <template #overlay>
                  <a-menu>
                    <a-menu-item
                      key="pin"
                      :icon="h(chat.is_pinned ? PinOff : Pin, { size: 14 })"
                      @click.stop="$emit('toggle-pin', chat.id)"
                    >
                      {{ chat.is_pinned ? '取消置顶' : '置顶' }}
                    </a-menu-item>
                    <a-menu-item
                      key="rename"
                      :icon="h(SquarePen, { size: 14 })"
                      @click.stop="renameChat(chat.id)"
                    >
                      重命名
                    </a-menu-item>
                    <a-menu-item
                      key="delete"
                      :icon="h(Trash2, { size: 14 })"
                      @click.stop="$emit('delete-chat', chat.id)"
                    >
                      删除
                    </a-menu-item>
                  </a-menu>
                </template>
                <span class="action-btn-wrapper">
                  <a-button type="text" class="more-btn">
                    <MoreVertical :size="16" />
                  </a-button>
                  <Pin v-if="chat.is_pinned" :size="14" class="pinned-indicator" />
                </span>
              </a-dropdown>
            </span>
          </div>
        </template>
        <div v-else-if="!collapsed" class="empty-list">暂无对话历史</div>
        <div v-if="isLoadingMore" class="load-more-hint">加载中...</div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, h, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { ChevronDown, MoreVertical, Pin, PinOff, SquarePen, Trash2 } from 'lucide-vue-next'
import { parseToShanghai } from '@/utils/time'

const props = defineProps({
  currentChatId: {
    type: String,
    default: null
  },
  chatsList: {
    type: Array,
    default: () => []
  },
  hasMoreChats: {
    type: Boolean,
    default: false
  },
  isLoadingMore: {
    type: Boolean,
    default: false
  },
  collapsed: {
    type: Boolean,
    default: false
  },
  showHistory: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['select-chat', 'delete-chat', 'rename-chat', 'toggle-pin', 'load-more-chats'])

const SCROLL_THRESHOLD = 48

const listRef = ref(null)
const listCollapsed = ref(false)

const sortedChats = computed(() => {
  return [...props.chatsList].sort((a, b) => {
    if (a.is_pinned !== b.is_pinned) {
      return a.is_pinned ? -1 : 1
    }
    const dateA = parseToShanghai(b.created_at)
    const dateB = parseToShanghai(a.created_at)
    if (!dateA || !dateB) return 0
    return dateA.diff(dateB)
  })
})

const handleListScroll = () => {
  const el = listRef.value
  if (!el || props.isLoadingMore || !props.hasMoreChats) return
  const remaining = el.scrollHeight - el.scrollTop - el.clientHeight
  if (remaining > SCROLL_THRESHOLD) return
  emit('load-more-chats')
}

const renameChat = async (chatId) => {
  const chat = props.chatsList.find((item) => String(item.id) === String(chatId))
  if (!chat) return

  let newTitle = chat.title || ''
  Modal.confirm({
    title: '重命名对话',
    icon: null,
    closable: false,
    maskClosable: true,
    centered: true,
    width: 400,
    class: 'rename-conversation-modal',
    content: h('div', [
      h('p', { class: 'rename-conversation-description' }, '保持简短且易于识别'),
      h('input', {
        value: newTitle,
        class: 'rename-conversation-input',
        onInput: (event) => {
          newTitle = event.target.value
        }
      })
    ]),
    okText: '保存',
    cancelText: '取消',
    onOk: () => {
      if (!newTitle.trim()) {
        message.warning('标题不能为空')
        return Promise.reject()
      }
      emit('rename-chat', { chatId, title: newTitle })
    }
  })
}
</script>

<style lang="less">
.rename-conversation-modal {
  .ant-modal-content {
    padding: 22px 24px 20px;
    border-radius: 12px;
  }

  .ant-modal-confirm-title {
    color: var(--gray-900);
    font-size: 18px;
    font-weight: 600;
    line-height: 1.4;
  }

  .ant-modal-confirm-body .ant-modal-confirm-content {
    width: 100%;
    max-width: none !important;
    margin-top: 4px;
  }

  .ant-modal-confirm-btns {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 18px;

    .ant-btn {
      min-width: 72px;
      height: 36px;
      margin-inline-start: 0;
      border-radius: 8px;
      font-size: 15px;
    }
  }
}

.rename-conversation-description {
  margin: 0 0 14px;
  color: var(--gray-500);
  font-size: 15px;
}

.rename-conversation-input {
  width: 100%;
  height: 40px;
  padding: 0 12px;
  color: var(--gray-900);
  background: var(--gray-0);
  border: 1px solid var(--gray-150);
  border-radius: 8px;
  outline: none;
  font-size: 15px;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;

  &:focus {
    border-color: var(--main-400);
    box-shadow: 0 0 0 2px var(--main-50);
  }
}
</style>

<style lang="less" scoped>
.conversation-nav-section {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
  overflow: hidden;
}

.conversation-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-panel {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
}

.history-label {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  padding: 4px 8px;
  color: var(--gray-800);
  cursor: pointer;
  font-size: 15px;
  font-weight: 600;
  border-radius: 6px;
  gap: 4px;

  span {
    font-weight: 500;
  }
}

.collapse-icon {
  transition: transform 0.2s ease;

  &.collapsed {
    transform: rotate(-90deg);
  }
}

.conversation-list {
  min-height: 0;
  max-height: calc(8 * 40px + 7 * 2px);
  overflow-y: auto;
  padding-right: 2px;
  scrollbar-width: thin;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.conversation-item {
  position: relative;
  display: flex;
  align-items: center;
  flex-shrink: 0;
  width: 100%;
  height: 40px;
  padding: 0 8px;
  overflow: hidden;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--gray-700);
  cursor: pointer;
  font-size: 14px;
  text-align: left;
  transition:
    background-color 0.2s ease,
    color 0.2s ease;

  &:hover {
    background: rgba(100, 116, 139, 0.1);
    color: #2563b0;

    .actions-mask,
    .conversation-actions {
      opacity: 1;
    }

    .more-btn {
      display: inline-flex;
    }

    .pinned-indicator {
      display: none;
    }
  }

  &.active {
    background-color: rgba(37, 99, 176, 0.14);
    color: #2563b0;

    .conversation-title {
      font-weight: 600;
    }

    &:hover {
      background-color: rgba(37, 99, 176, 0.14);
    }
  }

  &:has(.pinned-indicator) {
    .actions-mask,
    .conversation-actions {
      opacity: 1;
    }
  }
}

.actions-mask {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 56px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease;
}

.conversation-actions {
  position: absolute;
  top: 50%;
  right: 4px;
  display: flex;
  align-items: center;
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.2s ease;
}

.action-btn-wrapper {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
}

.more-btn {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: none;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  color: var(--gray-600);
}

.pinned-indicator {
  color: var(--gray-400);
}

.empty-list {
  margin-top: 16px;
  color: var(--gray-500);
  font-size: 14px;
  text-align: center;
  padding: 8px;
}

.load-more-hint {
  padding: 8px;
  color: var(--gray-400);
  font-size: 13px;
  text-align: center;
}
</style>
