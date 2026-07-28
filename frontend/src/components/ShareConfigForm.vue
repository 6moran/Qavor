<template>
  <div class="share-config-form" :class="{ disabled }">
    <div
      class="share-mode-cards"
      :class="`active-${config.access_level}`"
      role="radiogroup"
      aria-label="共享设置"
    >
      <div
        v-for="option in shareModeOptions"
        :key="option.value"
        role="radio"
        class="share-mode-card"
        :class="{ active: config.access_level === option.value }"
        :aria-checked="config.access_level === option.value"
        :tabindex="!disabled && config.access_level === option.value ? 0 : -1"
        @click="setAccessLevel(option.value)"
        @keydown.enter.prevent="setAccessLevel(option.value)"
        @keydown.space.prevent="setAccessLevel(option.value)"
      >
        <div class="card-main">
          <div class="card-header">
            <div class="card-icon-wrapper" aria-hidden="true">
              <component :is="option.icon" class="card-icon" :size="20" />
            </div>
            <div class="card-title">{{ option.title }}</div>
            <div
              v-if="config.access_level === option.value && option.value !== 'global'"
              class="card-action"
              @click.stop
            >
              <a-dropdown
                :trigger="['click']"
                placement="bottomRight"
                overlay-class-name="share-selection-popover"
              >
                <a-button
                  size="small"
                  class="select-action lucide-icon-btn"
                  :aria-label="option.value === 'department' ? '选择部门' : '选择用户'"
                  :disabled="disabled"
                >
                  <UserPlus class="select-action-icon" :size="14" />
                  <span class="access-count">{{ getAccessCount(option.value) }}</span>
                </a-button>
                <template #overlay>
                  <div class="selection-dropdown" @mousedown.stop @click.stop>
                    <div class="selection-dropdown-header">
                      <div class="selection-dropdown-title">
                        {{ option.value === 'department' ? '可访问部门' : '可访问用户' }}
                      </div>
                      <div class="selection-dropdown-subtitle">
                        {{ getAccessSummary(option.value) }}
                      </div>
                    </div>
                    <a-input
                      v-model:value="selectionSearch[option.value]"
                      size="small"
                      allow-clear
                      class="selection-search"
                      :placeholder="option.value === 'department' ? '搜索部门' : '搜索用户'"
                      @mousedown.stop
                      @click.stop
                    />
                    <div v-if="getSelectionOptions(option.value).length" class="selection-list">
                      <div
                        v-for="item in getSelectionOptions(option.value)"
                        :key="item.value"
                        role="checkbox"
                        :aria-checked="isSelected(option.value, item.value)"
                        :tabindex="item.disabled ? -1 : 0"
                        class="selection-item"
                        :class="{
                          selected: isSelected(option.value, item.value),
                          locked: item.disabled
                        }"
                        @mousedown.stop
                        @click.stop="
                          !item.disabled &&
                          toggleSelection(
                            option.value,
                            item.value,
                            !isSelected(option.value, item.value)
                          )
                        "
                        @keydown.enter.prevent="
                          !item.disabled &&
                          toggleSelection(
                            option.value,
                            item.value,
                            !isSelected(option.value, item.value)
                          )
                        "
                        @keydown.space.prevent="
                          !item.disabled &&
                          toggleSelection(
                            option.value,
                            item.value,
                            !isSelected(option.value, item.value)
                          )
                        "
                      >
                        <span class="selection-item-content">
                          <a-checkbox
                            :checked="isSelected(option.value, item.value)"
                            :disabled="item.disabled"
                            @click.stop
                            @change="
                              toggleSelection(option.value, item.value, $event.target.checked)
                            "
                          />
                          <span class="selection-label">{{ item.label }}</span>
                        </span>
                        <span v-if="item.disabled" class="selection-required">必选</span>
                      </div>
                    </div>
                    <div v-else class="selection-empty">暂无可选项</div>
                  </div>
                </template>
              </a-dropdown>
            </div>
          </div>
          <div class="card-description">{{ option.description }}</div>
        </div>
      </div>
    </div>
    <a-alert
      v-if="disabled && disabledReason"
      type="info"
      show-icon
      class="share-disabled-alert"
      :message="disabledReason"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, nextTick } from 'vue'
import { Globe, Building2, Users, UserPlus } from 'lucide-vue-next'
import { useUserStore } from '@/stores/user'
import { departmentApi } from '@/apis/department_api'
import { authApi } from '@/apis/auth_api'

const userStore = useUserStore()
const departments = ref([])
const users = ref([])
const syncingFromProps = ref(false)

const baseShareModeOptions = [
  {
    value: 'global',
    title: '全局共享',
    description: '所有用户都可以访问',
    icon: Globe
  },
  {
    value: 'department',
    title: '部门共享',
    description: '选中的部门成员可以访问',
    icon: Building2
  },
  {
    value: 'user',
    title: '指定人',
    description: '选中的用户可以访问',
    icon: Users
  }
]

const props = defineProps({
  modelValue: {
    type: Object,
    required: true,
    default: () => ({
      access_level: 'global',
      department_ids: [],
      user_uids: []
    })
  },
  autoSelectUserDept: {
    type: Boolean,
    default: false
  },
  disabled: {
    type: Boolean,
    default: false
  },
  disabledReason: {
    type: String,
    default: ''
  },
  allowedAccessLevels: {
    type: Array,
    default: () => ['global', 'department', 'user']
  }
})

const emit = defineEmits(['update:modelValue'])

const config = reactive({
  access_level: 'global',
  department_ids: [],
  user_uids: []
})

const selectionSearch = reactive({
  department: '',
  user: ''
})

const currentDepartmentId = computed(() => {
  if (!userStore.departmentId) return null
  return Number(userStore.departmentId)
})

const currentUserUid = computed(() => userStore.uid || '')
const normalizedAllowedAccessLevels = computed(() => {
  const allowed = props.allowedAccessLevels.filter((level) =>
    ['global', 'department', 'user'].includes(level)
  )
  return allowed.length ? allowed : ['global']
})
const shareModeOptions = computed(() =>
  baseShareModeOptions.filter((option) =>
    normalizedAllowedAccessLevels.value.includes(option.value)
  )
)

const departmentOptions = computed(() =>
  departments.value.map((dept) => {
    const value = Number(dept.id)
    return {
      label: dept.name,
      value,
      disabled: value === currentDepartmentId.value
    }
  })
)

const userOptions = computed(() =>
  users.value.map((user) => ({
    label: user.department_name ? `${user.username}（${user.department_name}）` : user.username,
    value: user.uid,
    disabled: user.uid === currentUserUid.value
  }))
)

const normalizeDepartmentIds = (ids) =>
  Array.from(new Set((ids || []).map((id) => Number(id)).filter((id) => Number.isFinite(id))))

const normalizeUserUids = (uids) =>
  Array.from(new Set((uids || []).map((uid) => String(uid).trim()).filter(Boolean)))

const ensureCurrentDepartment = () => {
  if (!props.autoSelectUserDept || !currentDepartmentId.value) return
  if (!config.department_ids.includes(currentDepartmentId.value)) {
    config.department_ids = [currentDepartmentId.value, ...config.department_ids]
  }
}

const ensureCurrentUser = () => {
  if (!currentUserUid.value) return
  if (!config.user_uids.includes(currentUserUid.value)) {
    config.user_uids = [currentUserUid.value, ...config.user_uids]
  }
}

const normalizeActiveConfig = () => {
  if (config.access_level === 'global') {
    config.department_ids = []
    config.user_uids = []
    return
  }

  if (config.access_level === 'department') {
    config.department_ids = normalizeDepartmentIds(config.department_ids)
    config.user_uids = []
    ensureCurrentDepartment()
    return
  }

  config.department_ids = []
  config.user_uids = normalizeUserUids(config.user_uids)
  ensureCurrentUser()
}

const initConfig = () => {
  syncingFromProps.value = true
  const requestedAccessLevel = ['global', 'department', 'user'].includes(
    props.modelValue?.access_level
  )
    ? props.modelValue.access_level
    : 'global'
  config.access_level = normalizedAllowedAccessLevels.value.includes(requestedAccessLevel)
    ? requestedAccessLevel
    : normalizedAllowedAccessLevels.value[0]
  config.department_ids = normalizeDepartmentIds(props.modelValue?.department_ids)
  config.user_uids = normalizeUserUids(props.modelValue?.user_uids)
  normalizeActiveConfig()
  nextTick(() => {
    syncingFromProps.value = false
  })
}

const emitConfig = () => {
  emit('update:modelValue', {
    access_level: config.access_level,
    department_ids:
      config.access_level === 'department' ? normalizeDepartmentIds(config.department_ids) : [],
    user_uids: config.access_level === 'user' ? normalizeUserUids(config.user_uids) : []
  })
}

const setAccessLevel = (accessLevel) => {
  if (props.disabled || !normalizedAllowedAccessLevels.value.includes(accessLevel)) return
  if (config.access_level === accessLevel) return
  config.access_level = accessLevel
  normalizeActiveConfig()
}

const getAccessSummary = (accessLevel) => {
  if (accessLevel === 'global') return '所有用户可访问'
  if (accessLevel === 'department') return `${config.department_ids.length} 个部门可访问`
  if (accessLevel === 'user' && config.user_uids.length === 1) return '仅自己可访问'
  return `${config.user_uids.length} 个用户可访问`
}

const getAccessCount = (accessLevel) => {
  if (accessLevel === 'department') return config.department_ids.length
  if (accessLevel === 'user') return config.user_uids.length
  return ''
}

const getSelectionOptions = (accessLevel) => {
  const options = accessLevel === 'department' ? departmentOptions.value : userOptions.value
  const query = selectionSearch[accessLevel]?.trim().toLowerCase()
  if (!query) return options
  return options.filter((item) => item.label.toLowerCase().includes(query))
}

const isSelected = (accessLevel, value) => {
  if (accessLevel === 'department') return config.department_ids.includes(Number(value))
  if (accessLevel === 'user') return config.user_uids.includes(String(value))
  return false
}

const toggleSelection = (accessLevel, value, checked) => {
  if (props.disabled) return
  if (accessLevel === 'department') {
    const departmentId = Number(value)
    const selected = checked
      ? [...config.department_ids, departmentId]
      : config.department_ids.filter((id) => id !== departmentId)
    config.department_ids = normalizeDepartmentIds(selected)
    ensureCurrentDepartment()
    return
  }

  const uid = String(value)
  const selected = checked
    ? [...config.user_uids, uid]
    : config.user_uids.filter((item) => item !== uid)
  config.user_uids = normalizeUserUids(selected)
  ensureCurrentUser()
}

const loadDepartments = async () => {
  try {
    const res = await departmentApi.getDepartments()
    departments.value = res.departments || res || []
    if (config.access_level === 'department') ensureCurrentDepartment()
  } catch (e) {
    console.error('加载部门列表失败:', e)
    departments.value = []
  }
}

const loadUsers = async () => {
  try {
    users.value = await authApi.getUserAccessOptions()
    if (config.access_level === 'user') ensureCurrentUser()
  } catch (e) {
    console.error('加载用户列表失败:', e)
    users.value = []
  }
}

watch(
  () => props.modelValue,
  () => initConfig(),
  { deep: true }
)

watch(normalizedAllowedAccessLevels, () => initConfig())

watch(
  config,
  () => {
    if (!syncingFromProps.value) emitConfig()
  },
  { deep: true }
)

watch(currentDepartmentId, () => {
  if (config.access_level === 'department') ensureCurrentDepartment()
})

watch(currentUserUid, () => {
  if (config.access_level === 'user') ensureCurrentUser()
})

const validate = () => {
  normalizeActiveConfig()

  if (config.access_level === 'global') {
    return { valid: true, message: '' }
  }

  if (config.access_level === 'department') {
    if (!currentDepartmentId.value) {
      return { valid: false, message: '您不属于任何部门，无法使用部门共享模式' }
    }
    if (!config.department_ids.includes(currentDepartmentId.value)) {
      return { valid: false, message: '您所在的部门必须在可访问部门范围内' }
    }
    return { valid: true, message: '' }
  }

  if (!currentUserUid.value) {
    return { valid: false, message: '无法获取当前用户，无法使用指定人可访问模式' }
  }
  if (!config.user_uids.includes(currentUserUid.value)) {
    return { valid: false, message: '当前用户必须在可访问用户范围内' }
  }
  return { valid: true, message: '' }
}

onMounted(() => {
  initConfig()
  loadDepartments()
  loadUsers()
})

defineExpose({
  config,
  validate
})
</script>

<style lang="less" scoped>
.share-config-form {
  .share-mode-cards {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
    align-items: stretch;

    @media (max-width: 768px) {
      grid-template-columns: 1fr;
    }
  }

  .share-disabled-alert {
    margin-top: 10px;
  }

  &.disabled .share-mode-card {
    cursor: not-allowed;
    opacity: 0.78;
  }

  .share-mode-card {
    position: relative;
    display: flex;
    min-width: 0;
    min-height: 82px;
    flex-direction: column;
    gap: 10px;
    padding: 14px;
    border: 1px solid var(--gray-200);
    border-radius: 10px;
    background: var(--gray-0);
    cursor: pointer;
    transition:
      border-color 180ms ease,
      background-color 180ms ease,
      box-shadow 180ms ease,
      transform 180ms ease;

    &:hover,
    &:focus-visible {
      border-color: var(--main-color);
      background: var(--main-10);
    }

    &:focus-visible {
      outline: none;
      box-shadow: 0 0 0 3px var(--main-20);
    }

    &.active {
      border-color: var(--main-color);
      background: var(--main-10);
      box-shadow: 0 0 0 1px var(--main-10);
    }
  }

  .card-main {
    display: flex;
    min-width: 0;
    flex: 1;
    flex-direction: column;
    gap: 8px;
  }

  .card-header {
    display: flex;
    align-items: center;
    min-width: 0;
    gap: 10px;
  }

  .card-action {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-left: auto;
  }

  .card-icon-wrapper {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border-radius: 9px;
    background: var(--gray-25);
    transition:
      background-color 180ms ease,
      box-shadow 180ms ease;
  }

  .card-icon {
    color: var(--gray-500);
    transition: color 180ms ease;
  }

  .share-mode-card.active .card-icon-wrapper {
    background: var(--main-0);
    box-shadow: inset 0 0 0 1px var(--main-100);
  }

  .share-mode-card.active .card-icon {
    color: var(--main-color);
  }

  .card-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--gray-800);
    line-height: 1.35;
    white-space: nowrap;
  }

  .card-description {
    font-size: 12px;
    line-height: 1.45;
    color: var(--gray-600);
  }

  .access-count {
    color: var(--main-color);
    font-size: 12px;
    font-weight: 500;
    line-height: 1;
  }

  .select-action {
    width: 44px;
    min-width: 24px;
    height: 24px;
    padding: 0;

    color: var(--main-color);
  }

  .select-action-icon {
    flex-shrink: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .share-mode-card,
    .card-icon-wrapper,
    .card-icon {
      transition: none;
    }
  }
}
</style>

<style lang="less">
.share-selection-popover {
  .selection-dropdown {
    width: 280px;
    max-height: 340px;
    padding: 8px;
    overflow: hidden auto;
    border: 1px solid var(--gray-200);
    border-radius: 14px;
    background: var(--gray-0);
    box-shadow:
      0 14px 36px rgb(0 0 0 / 12%),
      0 2px 8px rgb(0 0 0 / 6%);
  }

  .selection-dropdown-header {
    padding: 8px 10px 10px;
    margin-bottom: 4px;
    border-bottom: 1px solid var(--gray-100);
  }

  .selection-dropdown-title {
    font-size: 13px;
    font-weight: 700;
    color: var(--gray-900);
    line-height: 1.4;
  }

  .selection-dropdown-subtitle {
    margin-top: 2px;
    font-size: 12px;
    color: var(--gray-500);
    line-height: 1.4;
  }

  .selection-search {
    margin: 8px;
    width: calc(100% - 16px);
    height: 30px;
  }

  .selection-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .selection-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 38px;
    gap: 8px;
    padding: 8px 10px;
    border-radius: 9px;
    color: var(--gray-800);
    cursor: pointer;
    transition:
      background-color 160ms ease,
      color 160ms ease;

    &:hover {
      background: var(--gray-50);
    }

    &.selected {
      background: var(--main-10);
      color: var(--gray-900);
    }

    &.locked {
      cursor: not-allowed;
    }
  }

  .selection-item-content {
    display: flex;
    align-items: center;
    min-width: 0;
    gap: 8px;
  }

  .selection-label {
    min-width: 0;
    overflow: hidden;
    font-size: 13px;
    line-height: 18px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .selection-required {
    flex-shrink: 0;
    padding: 1px 6px;
    border-radius: 999px;
    background: var(--gray-100);
    color: var(--gray-500);
    font-size: 11px;
    line-height: 16px;
  }

  .selection-empty {
    display: block;
    padding: 14px 0;
    font-size: 13px;
    color: var(--gray-600);
    text-align: center;
  }
}
</style>
