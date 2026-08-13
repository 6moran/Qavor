<template>
  <div class="chunk-params-config">
    <div class="params-info">
      <p>选个匹配文档结构的分块策略就够了；下方「高级参数」里可以微调段落大小与重叠。</p>
    </div>
    <a-form :model="localParams" name="chunkConfig" autocomplete="off" layout="vertical">
      <a-form-item v-if="showPreset" name="chunk_preset_id">
        <template #label>
          <span class="chunk-preset-label">
            分块策略
            <a-tooltip :title="presetDescription">
              <QuestionCircleOutlined class="chunk-preset-help-icon" />
            </a-tooltip>
          </span>
        </template>
        <a-select
          v-model:value="localParams.chunk_preset_id"
          :options="presetOptions"
          :loading="chunkPresetLoading"
          style="width: 100%"
        />
        <p class="param-description">
          选择适合当前文档结构的分块策略。
          <span v-if="allowPresetFollowDefault">留空时沿用知识库默认策略。</span>
        </p>
      </a-form-item>

      <a-collapse v-if="hasAdvancedFields" ghost class="chunk-advanced-collapse">
        <a-collapse-panel key="advanced" header="高级参数（可选）">
          <div class="chunk-row">
            <a-form-item v-if="showChunkSizeOverlap" name="chunk_token_num">
              <template #label>
                <span class="chunk-preset-label">
                  最大 Token 数
                  <a-tooltip title="每个段落最多包含多少 token。值越大每段越长但检索粒度变粗。推荐 256-1024，留空使用默认值 512。">
                    <QuestionCircleOutlined class="chunk-preset-help-icon" />
                  </a-tooltip>
                </span>
              </template>
              <a-input-number
                v-model:value="parserConfig.chunk_token_num"
                :min="100"
                :max="10000"
                placeholder="默认 512"
                style="width: 100%"
              />
            </a-form-item>
            <a-form-item v-if="showChunkSizeOverlap" name="overlapped_percent">
              <template #label>
                <span class="chunk-preset-label">
                  重叠比例 (%)
                  <a-tooltip title="相邻段落之间重叠部分占总段长的比例。值越大越不容易切断上下文，但索引会变大。推荐 10-20，留空为 0。">
                    <QuestionCircleOutlined class="chunk-preset-help-icon" />
                  </a-tooltip>
                </span>
              </template>
              <a-input-number
                v-model:value="parserConfig.overlapped_percent"
                :min="0"
                :max="99"
                placeholder="默认 0"
                style="width: 100%"
              />
            </a-form-item>
            <a-form-item v-if="showQaSplit" name="delimiter">
              <template #label>
                <span class="chunk-preset-label">
                  分隔符
                  <a-tooltip title="支持 \\n、\\t 等转义字符。留空时使用默认分隔符 \\n">
                    <QuestionCircleOutlined class="chunk-preset-help-icon" />
                  </a-tooltip>
                </span>
              </template>
              <a-input
                v-model:value="parserConfig.delimiter"
                placeholder="默认 \\n，可输入 \\n\\n\\n 或 ---"
                style="width: 100%"
              />
            </a-form-item>
          </div>
          <p class="param-description">一般文档用默认参数即可，留空会让分块器按策略内置值处理。</p>
        </a-collapse-panel>
      </a-collapse>
    </a-form>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { QuestionCircleOutlined } from '@ant-design/icons-vue'
import { useChunkPresetOptions } from '@/composables/useChunkPresetOptions'
import { DEFAULT_CHUNK_PRESET_ID, isPlainObject } from '@/utils/chunkUtils'

const props = defineProps({
  tempChunkParams: {
    type: Object,
    required: true
  },
  showQaSplit: {
    type: Boolean,
    default: true
  },
  showChunkSizeOverlap: {
    type: Boolean,
    default: true
  },
  showPreset: {
    type: Boolean,
    default: true
  },
  allowPresetFollowDefault: {
    type: Boolean,
    default: false
  },
  databasePresetId: {
    type: String,
    default: DEFAULT_CHUNK_PRESET_ID
  }
})

const localParams = computed(() => props.tempChunkParams)
const fallbackParserConfig = ref({})
const {
  chunkPresetSelectOptions,
  chunkPresetLabelMap,
  chunkPresetLoading,
  loadChunkPresetOptions,
  getChunkPresetDescription
} = useChunkPresetOptions()

const parserConfig = computed(() => {
  if (!isPlainObject(props.tempChunkParams.chunk_parser_config)) {
    return fallbackParserConfig.value
  }
  return props.tempChunkParams.chunk_parser_config
})

const presetOptions = computed(() => {
  const options = []
  const defaultPresetLabel =
    chunkPresetLabelMap.value[props.databasePresetId] ||
    props.databasePresetId ||
    DEFAULT_CHUNK_PRESET_ID

  if (props.allowPresetFollowDefault) {
    options.push({
      value: '',
      label: `沿用知识库默认（${defaultPresetLabel}）`
    })
  }

  options.push(...chunkPresetSelectOptions.value)

  return options
})

const effectivePresetId = computed(
  () => props.tempChunkParams.chunk_preset_id || props.databasePresetId || DEFAULT_CHUNK_PRESET_ID
)
const presetDescription = computed(() => getChunkPresetDescription(effectivePresetId.value))

const hasAdvancedFields = computed(
  () => props.showChunkSizeOverlap || props.showQaSplit
)

onMounted(() => {
  loadChunkPresetOptions()
})
</script>

<style scoped>
.chunk-params-config {
  width: 100%;
}

.params-info {
  margin-bottom: 16px;
}

.params-info p {
  margin: 0;
  color: var(--gray-500);
  font-size: 14px;
  line-height: 1.5;
}

.chunk-row {
  display: flex;
  gap: 16px;
  margin-bottom: 8px;
}

.chunk-row > .ant-form-item {
  flex: 1;
  margin-bottom: 0;
}

.chunk-advanced-collapse {
  margin-top: 4px;
}

.chunk-advanced-collapse :deep(.ant-collapse-header) {
  padding: 8px 0;
  font-size: 13px;
  color: var(--gray-500);
}

.chunk-advanced-collapse :deep(.ant-collapse-content-box) {
  padding: 0 0 8px 0;
}

.param-description {
  font-size: 12px;
  color: var(--gray-400);
  margin: 4px 0 0 0;
  line-height: 1.4;
}

.chunk-preset-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.chunk-preset-help-icon {
  color: var(--gray-500);
  cursor: help;
  font-size: 14px;
}
</style>
