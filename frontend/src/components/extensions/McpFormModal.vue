<template>
  <a-modal
    v-model:open="visible"
    title="添加 MCP"
    @ok="handleFormSubmit"
    :confirmLoading="formLoading"
    @cancel="visible = false"
    :maskClosable="false"
    width="640px"
    class="server-modal"
  >
    <!-- JSON 输入区域 -->
    <div class="json-input-section">
      <div class="json-input-header">
        <span class="json-input-label">粘贴 JSON 配置</span>
        <a-tooltip title="支持 Claude Desktop、Cursor 等工具的 MCP 配置格式">
          <HelpCircle :size="14" class="json-input-help" />
        </a-tooltip>
      </div>
      <a-textarea
        v-model:value="jsonInput"
        placeholder='粘贴 JSON 配置，如：
{
  "mcpServers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"]
    }
  }
}'
        :rows="5"
        class="json-textarea"
      />
      <div class="json-input-actions">
        <a-button type="primary" ghost @click="parseJson" :disabled="!jsonInput.trim()">
          <Code :size="14" />
          解析 JSON
        </a-button>
        <a-button v-if="jsonParsed" @click="clearJsonInput">
          清空重填
        </a-button>
        <span v-if="jsonParsed" class="parse-success">
          <CheckCircle :size="14" />
          已解析
        </span>
      </div>
    </div>

    <a-form v-if="jsonParsed" layout="vertical" class="extension-form">
      <a-form-item label="MCP 名称" required class="form-item">
        <a-input v-model:value="form.name" placeholder="请输入 MCP 名称，作为唯一标识" />
      </a-form-item>
      <a-form-item label="描述" class="form-item">
        <a-input v-model:value="form.description" placeholder="请输入 MCP 描述" />
      </a-form-item>
      <a-form-item label="传输类型" required class="form-item">
        <a-select v-model:value="form.transport">
          <a-select-option value="streamable_http">streamable_http</a-select-option>
          <a-select-option value="sse">sse</a-select-option>
          <a-select-option value="stdio">stdio</a-select-option>
        </a-select>
      </a-form-item>
      <template v-if="form.transport === 'streamable_http' || form.transport === 'sse'">
        <a-form-item label="MCP URL" required class="form-item">
          <a-input v-model:value="form.url" placeholder="https://example.com/mcp" />
        </a-form-item>
        <a-form-item label="HTTP 请求头" class="form-item">
          <a-textarea
            v-model:value="form.headersText"
            placeholder='JSON 格式，如：{"Authorization": "Bearer xxx"}'
            :rows="3"
          />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="HTTP 超时（秒）" class="form-item">
              <a-input-number
                v-model:value="form.timeout"
                :min="1"
                :max="300"
                style="width: 100%"
              />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="SSE 读取超时（秒）" class="form-item">
              <a-input-number
                v-model:value="form.sse_read_timeout"
                :min="1"
                :max="300"
                style="width: 100%"
              />
            </a-form-item>
          </a-col>
        </a-row>
      </template>
      <template v-if="isStdioTransport">
        <a-form-item label="命令" required class="form-item">
          <a-input v-model:value="form.command" placeholder="例如：npx 或 /path/to/server" />
        </a-form-item>
        <a-form-item label="参数" class="form-item">
          <a-select
            v-model:value="form.args"
            mode="tags"
            placeholder="输入参数后回车添加，如：-m"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item label="环境变量" class="form-item">
          <McpEnvEditor v-model="form.env" />
        </a-form-item>
      </template>
    </a-form>

    <template #footer>
      <div class="modal-footer">
        <a-button
          :loading="testLoading"
          @click="handleTestConnection"
          :disabled="!canTestConnection"
        >
          测试连接
        </a-button>
        <div class="modal-footer-actions">
          <a-button @click="visible = false">取消</a-button>
          <a-button type="primary" @click="handleFormSubmit" :loading="formLoading">确定</a-button>
        </div>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { Code, CheckCircle, HelpCircle } from 'lucide-vue-next'
import { mcpApi } from '@/apis/mcp_api'
import McpEnvEditor from '@/components/McpEnvEditor.vue'

const props = defineProps({
  open: { type: Boolean, default: false }
})

const emit = defineEmits(['update:open', 'submitted'])

const visible = computed({
  get: () => props.open,
  set: (val) => emit('update:open', val)
})

const formLoading = ref(false)
const testLoading = ref(false)
const jsonInput = ref('')
const jsonParsed = ref(false)

const form = reactive({
  name: '',
  description: '',
  transport: 'streamable_http',
  url: '',
  command: '',
  args: [],
  env: null,
  headersText: '',
  timeout: null,
  sse_read_timeout: null
})

const isStdioTransport = computed(
  () =>
    String(form.transport || '')
      .trim()
      .toLowerCase() === 'stdio'
)

watch(
  () => props.open,
  (val) => {
    if (val) {
      Object.assign(form, {
        name: '',
        description: '',
        transport: 'streamable_http',
        url: '',
        command: '',
        args: [],
        env: null,
        headersText: '',
        timeout: null,
        sse_read_timeout: null
      })
      jsonInput.value = ''
      jsonParsed.value = false
    }
  },
  { immediate: true }
)

// 智能解析 JSON 配置
const parseJson = () => {
  try {
    const parsed = JSON.parse(jsonInput.value)
    let config = null
    let detectedName = ''

    // 格式1: 带 mcpServers 包装（Claude Desktop / Cursor 格式）
    if (parsed.mcpServers && typeof parsed.mcpServers === 'object') {
      const keys = Object.keys(parsed.mcpServers)
      if (keys.length > 0) {
        detectedName = keys[0]
        config = parsed.mcpServers[detectedName]
      }
    }
    // 格式2: 直接配置对象
    else if (parsed.command || parsed.url) {
      config = parsed
    }

    if (!config) {
      message.error('无法识别的 JSON 格式，请检查配置')
      return
    }

    // 自动填充表单
    fillFormFromConfig(config, detectedName)
    jsonParsed.value = true
    message.success('JSON 解析成功，请确认配置')
  } catch (e) {
    message.error('JSON 格式错误：' + e.message)
  }
}

// 从配置对象填充表单
const fillFormFromConfig = (config, detectedName = '') => {
  // 填充 name（仅在未设置时）
  if (detectedName && !form.name) {
    form.name = detectedName
  }

  // 智能判断 transport 类型
  if (config.url) {
    // 根据 URL 或 headers 判断是 SSE 还是 streamable_http
    const hasSseIndicator = config.url.toLowerCase().includes('sse') ||
      (config.headers && JSON.stringify(config.headers).toLowerCase().includes('sse'))
    form.transport = hasSseIndicator ? 'sse' : 'streamable_http'
    form.url = config.url
  } else if (config.command) {
    form.transport = 'stdio'
    form.command = config.command
  }

  // 填充其他字段
  if (config.args && Array.isArray(config.args)) {
    form.args = config.args
  }
  if (config.env && typeof config.env === 'object') {
    form.env = config.env
  }
  if (config.headers && typeof config.headers === 'object') {
    form.headersText = JSON.stringify(config.headers, null, 2)
  }
  if (config.description) {
    form.description = config.description
  }
}

// 清空 JSON 输入并重置表单
const clearJsonInput = () => {
  jsonInput.value = ''
  jsonParsed.value = false
  // 重置表单
  Object.assign(form, {
    name: '',
    description: '',
    transport: 'streamable_http',
    url: '',
    command: '',
    args: [],
    env: null,
    headersText: '',
    timeout: null,
    sse_read_timeout: null
  })
}

// 构建提交/测试共用的配置数据；校验失败返回 null
const buildFormData = () => {
  let headers = null
  if (form.headersText.trim()) {
    try {
      headers = JSON.parse(form.headersText)
    } catch {
      message.error('请求头 JSON 格式错误')
      return null
    }
  }
  const data = {
    name: form.name,
    description: form.description || null,
    transport: form.transport,
    url: form.url || null,
    command: form.command || null,
    args: form.args.length > 0 ? form.args : null,
    env: form.env,
    headers,
    timeout: form.timeout || null,
    sse_read_timeout: form.sse_read_timeout || null
  }
  if (!data.name?.trim()) {
    message.error('MCP 名称不能为空')
    return null
  }
  if (!data.transport) {
    message.error('请选择传输类型')
    return null
  }
  if (['sse', 'streamable_http'].includes(data.transport)) {
    if (!data.url?.trim()) {
      message.error('HTTP 类型必须填写 MCP URL')
      return null
    }
  }
  if (data.transport === 'stdio') {
    if (!data.command?.trim()) {
      message.error('StdIO 类型必须填写命令')
      return null
    }
  }
  return data
}

const handleFormSubmit = async () => {
  try {
    const data = buildFormData()
    if (!data) return
    formLoading.value = true
    const result = await mcpApi.createMcpServer(data)
    if (result.success) {
      message.success('MCP 创建成功')
    } else {
      message.error(result.message || '创建失败')
      return
    }
    visible.value = false
    emit('submitted')
  } catch (err) {
    message.error(err.message || '操作失败')
  } finally {
    formLoading.value = false
  }
}

const canTestConnection = computed(() => {
  if (!form.transport) return false
  if (isStdioTransport.value) return !!form.command?.trim()
  return !!form.url?.trim()
})

const handleTestConnection = async () => {
  const data = buildFormData()
  if (!data) return
  try {
    testLoading.value = true
    const result = await mcpApi.testMcpServerConfig(data)
    if (result.success) {
      const info = result.data
      const suffix = info?.server_name
        ? ` · ${info.server_name}${info.server_version ? ' ' + info.server_version : ''}`
        : ''
      message.success('连接成功' + suffix)
    } else {
      message.error(result.message || '连接失败')
    }
  } catch (err) {
    message.error(err.message || '连接失败')
  } finally {
    testLoading.value = false
  }
}
</script>

<style lang="less" scoped>
@import '@/assets/css/extensions.less';

.json-input-section {
  margin-bottom: 16px;
}

.json-input-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}

.json-input-label {
  font-weight: 600;
  color: var(--gray-700);
  font-size: 14px;
}

.json-input-help {
  color: var(--gray-400);
  cursor: help;
}

.json-textarea {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.5;
}

.json-input-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.parse-success {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--color-success-600);
  font-size: 13px;
}

.modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-footer-actions {
  display: flex;
  gap: 8px;
}
</style>
