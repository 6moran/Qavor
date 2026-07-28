<template>
  <Teleport to="body">
    <div v-if="open" class="conversation-search-overlay" @mousedown.self="close">
      <section
        class="conversation-search-modal"
        role="dialog"
        aria-modal="true"
        aria-label="搜索对话"
        @keydown.down.prevent="moveSelection(1)"
        @keydown.up.prevent="moveSelection(-1)"
        @keydown.enter.prevent="confirmSelection"
        @keydown.esc.prevent="close"
      >
        <div class="conversation-search-input-row">
          <input
            ref="searchInputRef"
            v-model="searchText"
            class="conversation-search-input"
            type="text"
            placeholder="搜索对话..."
            autocomplete="off"
            aria-label="搜索对话"
          />
          <button type="button" class="conversation-search-close" aria-label="关闭" @click="close">
            <X :size="20" />
          </button>
        </div>

        <div ref="resultListRef" class="conversation-search-body" @scroll="handleResultScroll">
          <template v-if="isSearchMode">
            <div v-if="isSearching && results.length === 0" class="conversation-search-skeleton">
              <div v-for="index in 5" :key="index" class="skeleton-row">
                <span class="skeleton-dot"></span>
                <span class="skeleton-lines">
                  <i></i>
                  <i></i>
                </span>
              </div>
            </div>

            <div v-else-if="results.length > 0" class="conversation-search-results">
              <button
                v-for="(item, index) in results"
                :key="item.id"
                type="button"
                class="conversation-search-result"
                :class="{ selected: selectedIndex === index }"
                @mouseenter="selectedIndex = index"
                @click="selectSearchResult(item)"
              >
                <MessageCircle :size="18" class="result-icon" />
                <span class="result-main">
                  <span class="result-title">{{ item.title || '新的对话' }}</span>
                  <span class="result-snippet">
                    <template v-for="(part, partIndex) in splitSnippet(item)" :key="partIndex">
                      <mark v-if="part.match">{{ part.text }}</mark>
                      <span v-else>{{ part.text }}</span>
                    </template>
                  </span>
                </span>
                <span class="result-date">{{
                  formatResultDate(item.latest_match_at || item.updated_at)
                }}</span>
              </button>
              <div v-if="isLoadingMore" class="conversation-search-loading-more">加载中...</div>
            </div>

            <div v-else class="conversation-search-empty">未找到相关对话</div>
          </template>

          <template v-else>
            <button
              type="button"
              class="conversation-search-default-item"
              :class="{ selected: selectedIndex === 0 }"
              @mouseenter="selectedIndex = 0"
              @click="createThread"
            >
              <MessageCirclePlus :size="18" class="default-icon" />
              <span>新对话</span>
            </button>

            <template v-for="row in recentRows" :key="row.key">
              <div v-if="row.type === 'label'" class="conversation-search-group-label">
                {{ row.label }}
              </div>
              <button
                v-else
                type="button"
                class="conversation-search-default-item"
                :class="{ selected: selectedIndex === row.actionIndex }"
                @mouseenter="selectedIndex = row.actionIndex"
                @click="selectRecentThread(row.thread)"
              >
                <MessageCircle :size="18" class="default-icon" />
                <span>{{ row.thread.title || '新的对话' }}</span>
              </button>
            </template>

            <div v-if="recentRows.length === 0" class="conversation-search-empty default-empty">
              暂无对话历史
            </div>
          </template>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { MessageCircle, MessageCirclePlus, X } from 'lucide-vue-next'
import { threadApi } from '@/apis'
import dayjs, { parseToShanghai } from '@/utils/time'

const SEARCH_LIMIT = 20
const RECENT_LIMIT = 30

const props = defineProps({
  open: {
    type: Boolean,
    default: false
  },
  recentThreads: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['update:open', 'select-thread', 'create-thread', 'thread-found'])

const searchInputRef = ref(null)
const resultListRef = ref(null)
const searchText = ref('')
const results = ref([])
const selectedIndex = ref(0)
const hasMore = ref(false)
const isSearching = ref(false)
const isLoadingMore = ref(false)
let searchTimer = null
let searchRequestId = 0

const trimmedSearchText = computed(() => searchText.value.trim())
const isSearchMode = computed(() => Boolean(trimmedSearchText.value))

const sortedRecentThreads = computed(() => {
  return [...props.recentThreads]
    .sort((a, b) => {
      const first = parseToShanghai(a.updated_at || a.created_at)
      const second = parseToShanghai(b.updated_at || b.created_at)
      if (!first && !second) return 0
      if (!first) return 1
      if (!second) return -1
      return second.valueOf() - first.valueOf()
    })
    .slice(0, RECENT_LIMIT)
})

const recentRows = computed(() => {
  const rows = []
  let lastGroup = ''
  let actionIndex = 1
  sortedRecentThreads.value.forEach((thread) => {
    const group = getRecentGroupLabel(thread)
    if (group !== lastGroup) {
      rows.push({ type: 'label', key: `label-${group}`, label: group })
      lastGroup = group
    }
    rows.push({
      type: 'thread',
      key: thread.id,
      thread,
      actionIndex
    })
    actionIndex += 1
  })
  return rows
})

const actionCount = computed(() => {
  if (isSearchMode.value) return results.value.length
  return 1 + sortedRecentThreads.value.length
})

const resetState = () => {
  searchText.value = ''
  results.value = []
  hasMore.value = false
  isSearching.value = false
  isLoadingMore.value = false
  selectedIndex.value = 0
}

const close = () => {
  emit('update:open', false)
}

const createThread = () => {
  emit('create-thread')
  close()
}

const selectRecentThread = (thread) => {
  if (!thread?.id) return
  emit('select-thread', thread)
  close()
}

const selectSearchResult = (item) => {
  if (!item?.id) return
  emit('thread-found', normalizeSearchThread(item))
  emit('select-thread', normalizeSearchThread(item))
  close()
}

const normalizeSearchThread = (item) => ({
  id: item.id || item.thread_id,
  uid: item.uid,
  agent_id: item.agent_id,
  title: item.title,
  is_pinned: Boolean(item.is_pinned),
  created_at: item.created_at,
  updated_at: item.updated_at,
  metadata: item.metadata || {}
})

const moveSelection = (delta) => {
  if (actionCount.value <= 0) return
  selectedIndex.value = (selectedIndex.value + delta + actionCount.value) % actionCount.value
  scrollSelectedIntoView()
}

const confirmSelection = () => {
  if (isSearchMode.value) {
    const item = results.value[selectedIndex.value]
    if (item) selectSearchResult(item)
    return
  }

  if (selectedIndex.value === 0) {
    createThread()
    return
  }
  const thread = sortedRecentThreads.value[selectedIndex.value - 1]
  if (thread) selectRecentThread(thread)
}

const scrollSelectedIntoView = () => {
  nextTick(() => {
    const selected = resultListRef.value?.querySelector('.selected')
    selected?.scrollIntoView({ block: 'nearest' })
  })
}

const searchThreads = async ({ reset = false } = {}) => {
  const query = trimmedSearchText.value
  if (!query) {
    results.value = []
    hasMore.value = false
    selectedIndex.value = 0
    return
  }

  const requestId = ++searchRequestId
  const offset = reset ? 0 : results.value.length
  if (reset) {
    isSearching.value = true
    hasMore.value = false
  } else {
    isLoadingMore.value = true
  }

  try {
    const response = await threadApi.searchThreads(query, {
      limit: SEARCH_LIMIT,
      offset
    })
    if (requestId !== searchRequestId) return
    const items = response?.items || []
    results.value = reset ? items : [...results.value, ...items]
    hasMore.value = Boolean(response?.has_more)
    selectedIndex.value =
      results.value.length > 0 ? Math.min(selectedIndex.value, results.value.length - 1) : 0
  } catch (error) {
    if (requestId === searchRequestId) {
      console.warn('搜索对话失败:', error)
      results.value = reset ? [] : results.value
      hasMore.value = false
    }
  } finally {
    if (requestId === searchRequestId) {
      isSearching.value = false
      isLoadingMore.value = false
    }
  }
}

const handleResultScroll = () => {
  if (!isSearchMode.value || !hasMore.value || isSearching.value || isLoadingMore.value) return
  const el = resultListRef.value
  if (!el) return
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) {
    searchThreads({ reset: false })
  }
}

const getRecentGroupLabel = (thread) => {
  const parsed = parseToShanghai(thread.updated_at || thread.created_at)
  if (!parsed) return '更早'
  const diffDays = dayjs().startOf('day').diff(parsed.startOf('day'), 'day')
  if (diffDays <= 7) return '前 7 天'
  if (diffDays <= 30) return '前 30 天'
  return '更早'
}

const formatResultDate = (value) => {
  const parsed = parseToShanghai(value)
  if (!parsed) return ''
  if (parsed.year() === dayjs().year()) return parsed.format('M月D日')
  return parsed.format('YYYY-MM-DD')
}

const splitSnippet = (item) => {
  const content = item?.snippets?.[0]?.content || ''
  const query = trimmedSearchText.value
  if (!content || !query) return [{ text: content, match: false }]

  const lowerContent = content.toLowerCase()
  const lowerQuery = query.toLowerCase()
  const parts = []
  let cursor = 0
  let index = lowerContent.indexOf(lowerQuery)

  while (index >= 0) {
    if (index > cursor) {
      parts.push({ text: content.slice(cursor, index), match: false })
    }
    parts.push({ text: content.slice(index, index + query.length), match: true })
    cursor = index + query.length
    index = lowerContent.indexOf(lowerQuery, cursor)
  }
  if (cursor < content.length) {
    parts.push({ text: content.slice(cursor), match: false })
  }
  return parts
}

watch(
  () => props.open,
  (nextOpen) => {
    if (!nextOpen) return
    resetState()
    nextTick(() => {
      searchInputRef.value?.focus()
    })
  }
)

watch(trimmedSearchText, (query) => {
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
  selectedIndex.value = 0
  results.value = []
  hasMore.value = false
  if (!query) {
    searchRequestId += 1
    isSearching.value = false
    return
  }
  isSearching.value = true
  searchTimer = setTimeout(() => {
    searchThreads({ reset: true })
  }, 240)
})

onUnmounted(() => {
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
  searchRequestId += 1
})
</script>

<style lang="less" scoped>
.conversation-search-overlay {
  position: fixed;
  inset: 0;
  z-index: 1200;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 18vh 16px 24px;
  background: color-mix(in srgb, var(--gray-0) 72%, transparent);
  backdrop-filter: blur(2px);
}

.conversation-search-modal {
  width: min(680px, calc(100vw - 32px));
  max-height: min(620px, 72vh);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--gray-150);
  border-radius: 12px;
  background: var(--gray-0);
  box-shadow:
    0 24px 60px var(--shadow-1),
    0 2px 12px var(--shadow-0);
}

.conversation-search-input-row {
  display: flex;
  align-items: center;
  min-height: 62px;
  border-bottom: 1px solid var(--gray-100);
}

.conversation-search-input {
  flex: 1 1 auto;
  min-width: 0;
  height: 62px;
  padding: 0 18px;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--gray-1000);
  font-size: 18px;
  line-height: 24px;

  &::placeholder {
    color: var(--gray-400);
  }
}

.conversation-search-close {
  flex: 0 0 40px;
  width: 40px;
  height: 40px;
  margin-right: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--gray-500);
  cursor: pointer;
  transition:
    background-color 0.2s ease,
    color 0.2s ease;

  &:hover,
  &:focus-visible {
    background: var(--gray-50);
    color: var(--gray-900);
    outline: none;
  }
}

.conversation-search-body {
  min-height: 280px;
  max-height: calc(72vh - 63px);
  overflow-y: auto;
  padding: 8px;
  scrollbar-width: thin;
}

.conversation-search-default-item,
.conversation-search-result {
  width: 100%;
  min-height: 44px;
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--gray-900);
  cursor: pointer;
  text-align: left;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease;

  &:hover,
  &.selected,
  &:focus-visible {
    background: var(--gray-50);
    outline: none;
  }
}

.conversation-search-default-item {
  height: 44px;
  padding: 0 14px;
  font-size: 15px;
}

.default-icon,
.result-icon {
  flex: 0 0 18px;
  color: var(--gray-700);
}

.conversation-search-group-label {
  padding: 14px 14px 8px;
  color: var(--gray-500);
  font-size: 13px;
  line-height: 18px;
}

.conversation-search-result {
  min-height: 60px;
  padding: 9px 12px;
}

.result-main {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.result-title {
  overflow: hidden;
  color: var(--gray-1000);
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-snippet {
  overflow: hidden;
  color: var(--gray-600);
  font-size: 13px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;

  mark {
    padding: 0;
    background: color-mix(in srgb, var(--main-color) 14%, transparent);
    color: var(--main-700);
  }
}

.result-date {
  flex: 0 0 auto;
  align-self: center;
  color: var(--gray-500);
  font-size: 13px;
  line-height: 18px;
}

.conversation-search-skeleton {
  padding: 8px 14px;
}

.skeleton-row {
  height: 58px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.skeleton-dot {
  flex: 0 0 16px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--gray-100);
}

.skeleton-lines {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 10px;

  i {
    height: 8px;
    border-radius: 999px;
    background: var(--gray-100);

    &:first-child {
      width: 190px;
    }

    &:last-child {
      width: min(390px, 72%);
    }
  }
}

.conversation-search-empty {
  padding: 48px 16px;
  color: var(--gray-500);
  font-size: 14px;
  text-align: center;
}

.default-empty {
  padding-top: 32px;
}

.conversation-search-loading-more {
  padding: 10px 0 6px;
  color: var(--gray-500);
  font-size: 13px;
  text-align: center;
}

@media (max-width: 640px) {
  .conversation-search-overlay {
    padding-top: 12vh;
  }

  .conversation-search-input {
    font-size: 16px;
  }
}
</style>
