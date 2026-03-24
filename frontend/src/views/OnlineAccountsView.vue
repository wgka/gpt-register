<template>
  <div class="online-accounts-page">
    <section class="summary-grid">
      <el-card class="summary-card page-card" shadow="never">
        <span class="summary-card__label">总账号</span>
        <strong class="summary-card__value">{{ files.length }}</strong>
      </el-card>
      <el-card class="summary-card page-card" shadow="never">
        <span class="summary-card__label">有效</span>
        <strong class="summary-card__value">{{ countByStatus('active') }}</strong>
      </el-card>
      <el-card class="summary-card page-card" shadow="never">
        <span class="summary-card__label">禁用</span>
        <strong class="summary-card__value">{{ countByStatus('disabled') }}</strong>
      </el-card>
      <el-card class="summary-card page-card" shadow="never">
        <span class="summary-card__label">限额中</span>
        <strong class="summary-card__value">{{ usageLimitedCount }}</strong>
      </el-card>
      <el-card class="summary-card page-card" shadow="never">
        <span class="summary-card__label">Token 失效</span>
        <strong class="summary-card__value">{{ invalidTokenCount }}</strong>
      </el-card>
    </section>

    <el-card class="page-card scheduler-card" shadow="never">
      <template #header>
        <div class="toolbar">
          <div>
            <h3 class="page-title">定时操作</h3>
            <p class="page-subtitle">服务端后台支持按间隔或固定时间执行，并带失败重试与执行日志。</p>
          </div>
          <div class="toolbar__actions">
            <el-tag :type="schedulerStatusTagType" effect="light">
              {{ schedulerStatusLabel }}
            </el-tag>
          </div>
        </div>
      </template>

      <div class="scheduler-panel">
        <div class="scheduler-form">
          <el-alert
            type="info"
            show-icon
            :closable="false"
            title="定时任务会复用设置页里的 CPA API URL 和 Token，仅处理 token_invalidated / deactivated_workspace 账号。"
          />

          <div class="scheduler-switches">
            <div class="scheduler-toggle">
              <div>
                <strong>开启定时任务</strong>
                <p>开启后由服务端后台自动执行，关闭页面后也会继续跑。</p>
              </div>
              <el-switch v-model="schedulerForm.enabled" inline-prompt active-text="开" inactive-text="关" />
            </div>
            <div class="scheduler-toggle">
              <div>
                <strong>删除失效账号</strong>
                <p>Token 失效账号直接删除；限额账号仍按现有逻辑自动禁用。</p>
              </div>
              <el-switch v-model="schedulerForm.delete_invalid" inline-prompt active-text="开" inactive-text="关" />
            </div>
          </div>

          <div class="scheduler-config-grid">
            <div class="scheduler-config-card">
              <span class="scheduler-config-card__label">调度方式</span>
              <el-radio-group v-model="schedulerForm.mode">
                <el-radio-button label="interval">按间隔</el-radio-button>
                <el-radio-button label="fixed_times">固定时间</el-radio-button>
              </el-radio-group>
            </div>

            <div v-if="schedulerForm.mode === 'interval'" class="scheduler-config-card">
              <span class="scheduler-config-card__label">执行周期</span>
              <div class="scheduler-inline-fields">
                <el-input-number
                  v-model="schedulerForm.interval_minutes"
                  :min="1"
                  :max="1440"
                  controls-position="right"
                />
                <span class="scheduler-inline-fields__suffix">分钟</span>
              </div>
            </div>

            <div v-else class="scheduler-config-card scheduler-config-card--times">
              <span class="scheduler-config-card__label">固定执行时间</span>
              <el-select
                v-model="schedulerForm.fixed_times"
                multiple
                filterable
                allow-create
                default-first-option
                clearable
                collapse-tags
                collapse-tags-tooltip
                placeholder="选择或输入 HH:mm，例如 09:00"
              >
                <el-option
                  v-for="option in schedulerTimeOptions"
                  :key="option"
                  :label="option"
                  :value="option"
                />
              </el-select>
              <p class="scheduler-config-card__hint">可多选，例如 09:00、14:30、23:00。</p>
            </div>

            <div class="scheduler-config-card">
              <span class="scheduler-config-card__label">失败重试</span>
              <div class="scheduler-retry-grid">
                <div class="scheduler-inline-fields">
                  <el-input-number
                    v-model="schedulerForm.retry_count"
                    :min="0"
                    :max="10"
                    controls-position="right"
                  />
                  <span class="scheduler-inline-fields__suffix">次</span>
                </div>
                <div class="scheduler-inline-fields">
                  <el-input-number
                    v-model="schedulerForm.retry_delay_minutes"
                    :min="1"
                    :max="1440"
                    controls-position="right"
                    :disabled="schedulerForm.retry_count === 0"
                  />
                  <span class="scheduler-inline-fields__suffix">分钟后重试</span>
                </div>
              </div>
            </div>
          </div>

          <div class="scheduler-actions-row">
            <p class="scheduler-hint">
              当前配置：{{ schedulerConfiguredActionsText }} · {{ schedulerModeDescription }}
              <span v-if="isSchedulerFormDirty">（有未保存修改）</span>
            </p>
            <div class="scheduler-actions">
              <el-button type="primary" :loading="schedulerSaving" @click="saveScheduler">
                保存配置
              </el-button>
              <el-button type="success" plain :loading="schedulerRunning" @click="runSchedulerNow">
                立即执行
              </el-button>
              <el-button plain :loading="schedulerLoading" @click="loadSchedulerState()">
                刷新状态
              </el-button>
            </div>
          </div>
        </div>

        <div class="scheduler-status">
          <div class="scheduler-status__header">
            <strong>任务状态</strong>
            <el-tag v-if="schedulerState.running" type="warning" effect="plain">执行中</el-tag>
          </div>

          <div class="scheduler-metrics">
            <div class="scheduler-metric">
              <span>下次执行</span>
              <strong>{{ formatDate(schedulerState.next_run_at) }}</strong>
            </div>
            <div class="scheduler-metric">
              <span>触发原因</span>
              <strong>{{ schedulerNextReasonLabel }}</strong>
            </div>
            <div class="scheduler-metric">
              <span>最近执行</span>
              <strong>{{ formatDate(schedulerState.last_run_at) }}</strong>
            </div>
            <div class="scheduler-metric">
              <span>待重试</span>
              <strong>{{ schedulerRetrySummary }}</strong>
            </div>
            <div class="scheduler-metric">
              <span>扫描失效</span>
              <strong>{{ schedulerState.last_result?.invalid_found ?? 0 }}</strong>
            </div>
            <div class="scheduler-metric">
              <span>最近处理</span>
              <strong>{{ schedulerHandledSummary }}</strong>
            </div>
          </div>

          <div class="scheduler-result" v-if="schedulerState.last_result">
            <div class="scheduler-result__actions">
              <span class="scheduler-result__label">上次动作</span>
              <div class="scheduler-result__tags">
                <el-tag
                  v-for="action in schedulerState.last_result.actions"
                  :key="action"
                  size="small"
                  effect="plain"
                >
                  {{ schedulerActionLabel(action) }}
                </el-tag>
              </div>
            </div>
            <p class="scheduler-result__status">
              状态：{{ schedulerRunStatusLabel(schedulerState.last_result.status) }}
              · 尝试 {{ schedulerState.last_result.attempt }}/{{ schedulerState.last_result.max_attempts }}
            </p>
            <div class="scheduler-result__messages" v-if="schedulerResultPreview.length > 0">
              <p v-for="(message, index) in schedulerResultPreview" :key="`${index}-${message}`">
                {{ message }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </el-card>

    <el-card class="page-card" shadow="never">
      <template #header>
        <div class="toolbar">
          <div>
            <h3 class="page-title">执行日志</h3>
            <p class="page-subtitle">记录每次定时/手动/重试执行的结果，便于追踪失败与重试情况。</p>
          </div>
          <div class="toolbar__actions">
            <el-button plain :loading="schedulerLogsLoading" @click="loadSchedulerLogs()">刷新日志</el-button>
          </div>
        </div>
      </template>

      <el-table
        v-loading="schedulerLogsLoading"
        :data="schedulerLogs"
        stripe
        empty-text="暂无执行日志"
      >
        <el-table-column label="执行时间" min-width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at || row.finished_at || row.started_at) }}
          </template>
        </el-table-column>
        <el-table-column label="触发方式" width="120">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ schedulerTriggerLabel(row.trigger_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="schedulerRunStatusTagType(row.status)" size="small" effect="light">
              {{ schedulerRunStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="尝试" width="100">
          <template #default="{ row }">
            {{ row.attempt }}/{{ row.max_attempts }}
          </template>
        </el-table-column>
        <el-table-column label="动作" min-width="180">
          <template #default="{ row }">
            <div class="log-tags">
              <el-tag
                v-for="action in row.actions"
                :key="`${row.id}-${action}`"
                size="small"
                effect="plain"
              >
                {{ schedulerActionLabel(action) }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="结果" min-width="180">
          <template #default="{ row }">
            扫描 {{ row.invalid_found }} / 禁用 {{ row.disabled_count }} / 删除 {{ row.deleted_count }} / 失败 {{ row.failed_count }}
          </template>
        </el-table-column>
        <el-table-column label="说明" min-width="260">
          <template #default="{ row }">
            <span>{{ schedulerLogMessage(row) }}</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination pagination--logs">
        <el-pagination
          :current-page="schedulerLogQuery.page"
          :page-size="schedulerLogQuery.pageSize"
          :page-sizes="[10, 20, 50]"
          :total="schedulerLogTotal"
          background
          layout="total, sizes, prev, pager, next"
          @current-change="handleSchedulerLogPageChange"
          @size-change="handleSchedulerLogPageSizeChange"
        />
      </div>
    </el-card>

    <el-card class="page-card" shadow="never">
      <template #header>
        <div class="toolbar">
          <div>
            <h3 class="page-title">线上账号管理</h3>
            <p class="page-subtitle">从线上服务获取账号列表，自动处理限额账号禁用/恢复，并支持清理 Token 失效账号。</p>
          </div>
          <div class="toolbar__actions">
            <el-button
              type="danger"
              plain
              :disabled="invalidFiles.length === 0"
              :loading="cleaningInvalid"
              @click="cleanAllInvalid"
            >
              清理全部失效 ({{ invalidFiles.length }})
            </el-button>
            <el-button :loading="loading" @click="loadFiles">刷新</el-button>
          </div>
        </div>
      </template>

      <div class="filters">
        <el-input v-model="searchText" clearable placeholder="搜索邮箱 / 账号" />
        <el-select v-model="filterStatus" clearable placeholder="全部状态">
          <el-option label="有效" value="active" />
          <el-option label="禁用" value="disabled" />
          <el-option label="限额中" value="usage_limited" />
          <el-option label="Token 失效" value="token_invalid" />
        </el-select>
      </div>

      <el-table
        v-loading="loading"
        :data="filteredFiles"
        row-key="id"
        stripe
        empty-text="暂无线上账号数据"
      >
        <el-table-column prop="account" label="邮箱" min-width="240" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row)" effect="light">
              {{ statusLabel(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="限额状态" width="120">
          <template #default="{ row }">
            <el-tag v-if="usageLimitState(row) === 'limited'" type="warning" effect="plain">限额中</el-tag>
            <el-tag v-else-if="usageLimitState(row) === 'recoverable'" type="info" effect="plain">待恢复</el-tag>
            <el-tag v-else-if="usageLimitState(row) === 'recovered'" type="success" effect="plain">已恢复</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="Token 状态" width="140">
          <template #default="{ row }">
            <el-tag v-if="isTokenInvalid(row)" type="danger" effect="plain">Token 失效</el-tag>
            <el-tag v-else type="success" effect="plain">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="套餐" width="100">
          <template #default="{ row }">
            {{ row.id_token?.plan_type || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="恢复时间" min-width="180">
          <template #default="{ row }">
            {{ formatRecoveryTime(row) }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="最后刷新" min-width="180">
          <template #default="{ row }">
            {{ formatDate(row.last_refresh) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" :loading="deletingId === row.id" @click="deleteFile(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

function normalizeManagementEndpoint(url?: string): string {
  const raw = (url || '').trim()
  if (!raw) return ''

  try {
    const parsed = new URL(raw)
    const pathname = parsed.pathname.replace(/\/+$/, '')
    if (!pathname) {
      return `${parsed.origin}/v0/management/auth-files`
    }
    return `${parsed.origin}${pathname}`
  } catch {
    return ''
  }
}

function normalizeFixedTimeValue(value: string): string | null {
  const match = value.trim().match(/^(\d{1,2}):(\d{1,2})$/)
  if (!match) return null
  const hour = Number(match[1])
  const minute = Number(match[2])
  if (!Number.isInteger(hour) || !Number.isInteger(minute)) return null
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) return null
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`
}

function normalizeFixedTimes(values: string[]): string[] {
  const unique = new Set<string>()
  for (const value of values) {
    const normalized = normalizeFixedTimeValue(value)
    if (normalized) {
      unique.add(normalized)
    }
  }
  return Array.from(unique).sort()
}

const schedulerTimeOptions = Array.from({ length: 48 }, (_, index) => {
  const hour = Math.floor(index / 2)
  const minute = index % 2 === 0 ? '00' : '30'
  return `${String(hour).padStart(2, '0')}:${minute}`
})

const managementEndpoint = ref(normalizeManagementEndpoint(import.meta.env.VITE_CPA_API_URL as string))
const managementToken = ref(((import.meta.env.VITE_CPA_API_TOKEN as string) || '').trim())

type IdToken = {
  chatgpt_account_id?: string
  plan_type?: string
}

type StatusMessagePayload = {
  error?: {
    code?: string
    type?: string
    resets_at?: number | string
    resets_in_seconds?: number | string
  }
  detail?: {
    code?: string
  }
}

type AuthFile = {
  id: string
  name: string
  account: string
  email: string
  status: string
  status_message: string
  disabled: boolean
  created_at: string
  last_refresh: string
  modtime: string
  id_token?: IdToken
  provider: string
  type: string
}

type SettingsResponse = {
  editable?: {
    cpa?: {
      api_url?: string
      api_token?: string
    }
  }
}

type SchedulerConfig = {
  enabled: boolean
  mode: 'interval' | 'fixed_times'
  interval_minutes: number
  fixed_times: string[]
  disable_invalid: boolean
  delete_invalid: boolean
  retry_count: number
  retry_delay_minutes: number
}

type SchedulerRunResult = {
  started_at: string
  finished_at: string
  trigger_type?: string
  status?: string
  attempt: number
  max_attempts: number
  actions: string[]
  invalid_found: number
  disabled_count: number
  deleted_count: number
  failed_count: number
  messages?: string[]
  error?: string
}

type SchedulerState = {
  config: SchedulerConfig
  running: boolean
  last_run_at?: string
  next_run_at?: string
  next_run_reason?: string
  retry_pending: boolean
  retry_remaining: number
  last_result?: SchedulerRunResult | null
}

type SchedulerResponse = {
  success?: boolean
  message?: string
  error?: string
  state: SchedulerState
}

type SchedulerLog = {
  id: number
  trigger_type: string
  status: string
  attempt: number
  max_attempts: number
  schedule_mode?: string
  actions: string[]
  invalid_found: number
  disabled_count: number
  deleted_count: number
  failed_count: number
  error_message?: string
  messages?: string[]
  started_at?: string
  finished_at?: string
  created_at?: string
}

type SchedulerLogResponse = {
  total: number
  logs: SchedulerLog[]
}

function defaultSchedulerConfig(): SchedulerConfig {
  return {
    enabled: false,
    mode: 'interval',
    interval_minutes: 30,
    fixed_times: ['09:00'],
    disable_invalid: false,
    delete_invalid: true,
    retry_count: 2,
    retry_delay_minutes: 5,
  }
}

function defaultSchedulerState(): SchedulerState {
  return {
    config: defaultSchedulerConfig(),
    running: false,
    retry_pending: false,
    retry_remaining: 0,
    last_result: null,
  }
}

const files = ref<AuthFile[]>([])
const loading = ref(false)
const deletingId = ref<string | null>(null)
const cleaningInvalid = ref(false)
const searchText = ref('')
const filterStatus = ref('')
const tokenInvalidCodes = new Set(['token_invalidated', 'deactivated_workspace'])

const schedulerLoading = ref(false)
const schedulerSaving = ref(false)
const schedulerRunning = ref(false)
const schedulerState = ref<SchedulerState>(defaultSchedulerState())
const schedulerForm = reactive<SchedulerConfig>(defaultSchedulerConfig())
const schedulerLogs = ref<SchedulerLog[]>([])
const schedulerLogsLoading = ref(false)
const schedulerLogTotal = ref(0)
const schedulerLogQuery = reactive({
  page: 1,
  pageSize: 10,
})
let schedulerPollingTimer: ReturnType<typeof setInterval> | null = null

function syncSchedulerForm(config: SchedulerConfig) {
  schedulerForm.enabled = !!config.enabled
  schedulerForm.mode = config.mode || 'interval'
  schedulerForm.interval_minutes = config.interval_minutes || 30
  schedulerForm.fixed_times = normalizeFixedTimes(config.fixed_times || [])
  schedulerForm.disable_invalid = false
  schedulerForm.delete_invalid = !!config.delete_invalid
  schedulerForm.retry_count = Math.max(config.retry_count ?? 2, 0)
  schedulerForm.retry_delay_minutes = config.retry_delay_minutes || 5
}

function applySchedulerState(state: SchedulerState, syncForm = false) {
  const mergedConfig = {
    ...defaultSchedulerConfig(),
    ...(state?.config || {}),
    fixed_times: normalizeFixedTimes(state?.config?.fixed_times || []),
  }

  schedulerState.value = {
    ...defaultSchedulerState(),
    ...state,
    config: mergedConfig,
    last_result: state?.last_result || null,
  }

  if (syncForm) {
    syncSchedulerForm(mergedConfig)
  }
}

function isTokenInvalid(file: AuthFile): boolean {
  if (containsTokenInvalidCode(file.status_message)) {
    return true
  }
  const payload = parseStatusMessage(file)
  if (!payload) return false
  return tokenInvalidCodes.has(payload.error?.code || payload.detail?.code || '')
}

function containsTokenInvalidCode(raw: string | undefined): boolean {
  if (!raw) return false
  return Array.from(tokenInvalidCodes).some((code) => raw.includes(code))
}

function parseStatusMessage(file: AuthFile): StatusMessagePayload | null {
  if (!file.status_message) return null
  let payload: unknown = file.status_message
  for (let i = 0; i < 2; i++) {
    if (payload && typeof payload === 'object') {
      return payload as StatusMessagePayload
    }
    if (typeof payload !== 'string') {
      return null
    }
    try {
      payload = JSON.parse(payload)
    } catch {
      return null
    }
  }
  return payload && typeof payload === 'object' ? (payload as StatusMessagePayload) : null
}

const invalidFiles = computed(() => files.value.filter(isTokenInvalid))

function effectiveStatus(file: AuthFile): string {
  return file.disabled ? 'disabled' : file.status
}

function usageLimitState(file: AuthFile): 'none' | 'limited' | 'recoverable' | 'recovered' {
  if (!isUsageLimited(file)) {
    return 'none'
  }

  const resetAt = usageLimitResetAt(file)
  if (resetAt === null || resetAt > Date.now()) {
    return 'limited'
  }
  if (file.disabled) {
    return 'recoverable'
  }
  return 'recovered'
}

const filteredFiles = computed(() => {
  let result = files.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    result = result.filter((f) => f.account.toLowerCase().includes(q) || f.email.toLowerCase().includes(q))
  }
  if (filterStatus.value === 'token_invalid') {
    result = result.filter(isTokenInvalid)
  } else if (filterStatus.value === 'usage_limited') {
    result = result.filter((f) => usageLimitState(f) === 'limited')
  } else if (filterStatus.value) {
    result = result.filter((f) => effectiveStatus(f) === filterStatus.value)
  }
  return result
})

function countByStatus(status: string): number {
  return files.value.filter((f) => effectiveStatus(f) === status).length
}

const invalidTokenCount = computed(() => invalidFiles.value.length)
const usageLimitedCount = computed(() => files.value.filter((f) => usageLimitState(f) === 'limited').length)

const schedulerStatusLabel = computed(() => {
  if (schedulerState.value.running) return '执行中'
  return schedulerState.value.config.enabled ? '已启用' : '未启用'
})

const schedulerStatusTagType = computed(() => {
  if (schedulerState.value.running) return 'warning'
  return schedulerState.value.config.enabled ? 'success' : 'info'
})

const normalizedFormFixedTimes = computed(() => normalizeFixedTimes(schedulerForm.fixed_times))

const schedulerConfiguredActionsText = computed(() => {
  return schedulerForm.delete_invalid ? '删除失效账号' : '未选择动作'
})

const schedulerModeDescription = computed(() => {
  if (schedulerForm.mode === 'fixed_times') {
    return normalizedFormFixedTimes.value.length > 0
      ? `固定时间 ${normalizedFormFixedTimes.value.join('、')}`
      : '固定时间未配置'
  }
  return `每 ${schedulerForm.interval_minutes} 分钟执行一次`
})

const schedulerHandledSummary = computed(() => {
  const result = schedulerState.value.last_result
  if (!result) return '-'
  return `禁用 ${result.disabled_count} / 删除 ${result.deleted_count} / 失败 ${result.failed_count}`
})

const schedulerRetrySummary = computed(() => {
  if (!schedulerState.value.retry_pending) return '无'
  return `剩余 ${schedulerState.value.retry_remaining} 次`
})

const schedulerNextReasonLabel = computed(() => {
  switch (schedulerState.value.next_run_reason) {
    case 'interval':
      return '按间隔'
    case 'fixed_times':
      return '固定时间'
    case 'retry':
      return '失败重试'
    default:
      return '-'
  }
})

const schedulerResultPreview = computed(() => {
  const result = schedulerState.value.last_result
  if (!result) return []
  const messages = result.messages || []
  const preview = messages.slice(0, 3)
  if (result.error && !preview.includes(result.error)) {
    preview.unshift(result.error)
  }
  return preview.slice(0, 3)
})

const isSchedulerFormDirty = computed(() => {
  const config = schedulerState.value.config
  const configFixedTimes = normalizeFixedTimes(config.fixed_times || [])
  return (
    schedulerForm.enabled !== config.enabled ||
    schedulerForm.mode !== config.mode ||
    schedulerForm.interval_minutes !== config.interval_minutes ||
    schedulerForm.delete_invalid !== config.delete_invalid ||
    schedulerForm.retry_count !== config.retry_count ||
    schedulerForm.retry_delay_minutes !== config.retry_delay_minutes ||
    JSON.stringify(normalizedFormFixedTimes.value) !== JSON.stringify(configFixedTimes)
  )
})

async function readJSONResponse<T>(response: Response): Promise<T> {
  const raw = await response.text()
  try {
    return JSON.parse(raw) as T
  } catch {
    const contentType = response.headers.get('content-type') || 'unknown'
    const snippet = raw.replace(/\s+/g, ' ').trim().slice(0, 120)
    throw new Error(`期望 JSON，实际返回 ${contentType}${snippet ? `: ${snippet}` : ''}`)
  }
}

async function refreshManagementConfig() {
  const response = await fetch('/api/settings')
  if (!response.ok) {
    throw new Error(`加载配置失败: HTTP ${response.status}`)
  }

  const data = await readJSONResponse<SettingsResponse>(response)
  managementEndpoint.value = normalizeManagementEndpoint(data.editable?.cpa?.api_url)
  managementToken.value = (data.editable?.cpa?.api_token || '').trim()
}

async function ensureManagementConfig() {
  try {
    await refreshManagementConfig()
  } catch (e) {
    console.warn('load management config failed', e)
  }

  if (!managementEndpoint.value) {
    throw new Error('CPA API URL 未配置')
  }
  if (!managementToken.value) {
    throw new Error('CPA API Token 未配置')
  }
}

function numberFromUnknown(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

function usageLimitResetAt(file: AuthFile): number | null {
  const payload = parseStatusMessage(file)
  if (payload?.error?.type !== 'usage_limit_reached') {
    return null
  }

  const resetsAt = numberFromUnknown(payload.error.resets_at)
  if (resetsAt !== null && resetsAt > 0) {
    return resetsAt * 1000
  }

  const resetsInSeconds = numberFromUnknown(payload.error.resets_in_seconds)
  if (resetsInSeconds !== null) {
    return Date.now() + Math.max(resetsInSeconds, 0) * 1000
  }

  return null
}

function isUsageLimited(file: AuthFile): boolean {
  return parseStatusMessage(file)?.error?.type === 'usage_limit_reached'
}

function shouldAutoDisableForLimit(file: AuthFile): boolean {
  if (!isUsageLimited(file) || file.disabled) {
    return false
  }
  const resetAt = usageLimitResetAt(file)
  return resetAt === null || resetAt > Date.now()
}

function shouldAutoEnableAfterLimit(file: AuthFile): boolean {
  if (!file.disabled || !isUsageLimited(file)) {
    return false
  }
  const resetAt = usageLimitResetAt(file)
  return resetAt !== null && resetAt <= Date.now()
}

async function updateFileDisabledStatus(file: AuthFile, disabled: boolean) {
  const response = await fetch(`${managementEndpoint.value}/status`, {
    method: 'PATCH',
    headers: {
      Authorization: `Bearer ${managementToken.value}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      name: file.name,
      disabled,
    }),
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
}

async function syncUsageLimitAccountStates(nextFiles: AuthFile[]): Promise<boolean> {
  let changed = false
  const failedAccounts: string[] = []

  for (const file of nextFiles) {
    const shouldDisable = shouldAutoDisableForLimit(file)
    const shouldEnable = !shouldDisable && shouldAutoEnableAfterLimit(file)
    if (!shouldDisable && !shouldEnable) {
      continue
    }

    try {
      await updateFileDisabledStatus(file, shouldDisable)
      changed = true
    } catch (e) {
      console.error('sync usage limit state failed', file.name, e)
      failedAccounts.push(file.account || file.email || file.name)
    }
  }

  if (failedAccounts.length > 0) {
    ElMessage.warning(`部分限额账号状态同步失败: ${failedAccounts.join('、')}`)
  }

  return changed
}

async function loadFiles(syncUsageLimit = true) {
  loading.value = true
  try {
    await ensureManagementConfig()

    const response = await fetch(managementEndpoint.value, {
      headers: { Authorization: `Bearer ${managementToken.value}` },
    })
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    const data = await readJSONResponse<{ files: AuthFile[] }>(response)
    const nextFiles = data.files ?? []

    if (syncUsageLimit) {
      const changed = await syncUsageLimitAccountStates(nextFiles)
      if (changed) {
        await loadFiles(false)
        return
      }
    }

    files.value = nextFiles
  } catch (e) {
    ElMessage.error('加载线上账号失败: ' + (e instanceof Error ? e.message : String(e)))
  } finally {
    loading.value = false
  }
}

async function loadSchedulerState(syncForm = true, silent = false) {
  if (!silent) {
    schedulerLoading.value = true
  }

  try {
    const response = await fetch('/api/online-accounts/scheduler')
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    const data = await readJSONResponse<SchedulerResponse>(response)
    applySchedulerState(data.state, syncForm)
  } catch (e) {
    if (!silent) {
      ElMessage.error('加载定时任务状态失败: ' + (e instanceof Error ? e.message : String(e)))
    }
  } finally {
    if (!silent) {
      schedulerLoading.value = false
    }
  }
}

async function loadSchedulerLogs(silent = false) {
  if (!silent) {
    schedulerLogsLoading.value = true
  }

  try {
    const response = await fetch(
      `/api/online-accounts/scheduler/logs?page=${schedulerLogQuery.page}&page_size=${schedulerLogQuery.pageSize}`,
    )
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    const data = await readJSONResponse<SchedulerLogResponse>(response)
    schedulerLogs.value = data.logs || []
    schedulerLogTotal.value = data.total || 0
  } catch (e) {
    if (!silent) {
      ElMessage.error('加载执行日志失败: ' + (e instanceof Error ? e.message : String(e)))
    }
  } finally {
    if (!silent) {
      schedulerLogsLoading.value = false
    }
  }
}

async function persistScheduler(showSuccess = true): Promise<boolean> {
  const normalizedTimes = normalizedFormFixedTimes.value

  if (schedulerForm.enabled && !schedulerForm.delete_invalid) {
    ElMessage.warning('启用定时任务时至少选择一个动作')
    return false
  }

  if (schedulerForm.mode === 'fixed_times' && normalizedTimes.length === 0) {
    ElMessage.warning('固定时间模式下至少配置一个执行时间，格式如 09:00')
    return false
  }

  schedulerSaving.value = true
  try {
    const response = await fetch('/api/online-accounts/scheduler', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        config: {
          enabled: schedulerForm.enabled,
          mode: schedulerForm.mode,
          interval_minutes: schedulerForm.interval_minutes,
          fixed_times: normalizedTimes,
          disable_invalid: false,
          delete_invalid: schedulerForm.delete_invalid,
          retry_count: schedulerForm.retry_count,
          retry_delay_minutes: schedulerForm.retry_delay_minutes,
        },
      }),
    })
    const data = await readJSONResponse<SchedulerResponse>(response)
    if (!response.ok) {
      throw new Error(data.error || `HTTP ${response.status}`)
    }

    applySchedulerState(data.state, true)
    if (showSuccess) {
      ElMessage.success(data.message || '定时任务配置已保存')
    }
    return true
  } catch (e) {
    ElMessage.error('保存定时任务失败: ' + (e instanceof Error ? e.message : String(e)))
    return false
  } finally {
    schedulerSaving.value = false
  }
}

async function saveScheduler() {
  await persistScheduler(true)
}

async function runSchedulerNow() {
  if (schedulerForm.mode === 'fixed_times' && normalizedFormFixedTimes.value.length === 0) {
    ElMessage.warning('固定时间模式下至少配置一个执行时间后再执行')
    return
  }
  if (!schedulerForm.delete_invalid) {
    ElMessage.warning('至少选择一个动作后再执行')
    return
  }

  if (isSchedulerFormDirty.value) {
    const saved = await persistScheduler(false)
    if (!saved) return
  }

  schedulerRunning.value = true
  try {
    const response = await fetch('/api/online-accounts/scheduler/run', { method: 'POST' })
    const data = await readJSONResponse<SchedulerResponse>(response)
    if (data.state) {
      applySchedulerState(data.state, false)
    }
    if (!response.ok || data.success === false) {
      throw new Error(data.error || `HTTP ${response.status}`)
    }

    ElMessage.success(data.message || '定时任务执行完成')
    await Promise.all([loadFiles(), loadSchedulerState(false, true), loadSchedulerLogs(true)])
  } catch (e) {
    ElMessage.error('执行定时任务失败: ' + (e instanceof Error ? e.message : String(e)))
  } finally {
    schedulerRunning.value = false
  }
}

async function deleteFile(file: AuthFile) {
  try {
    await ElMessageBox.confirm(`确定要删除账号 ${file.account} 吗？此操作不可撤销。`, '删除确认', {
      type: 'warning',
    })
  } catch {
    return
  }

  deletingId.value = file.id
  try {
    await ensureManagementConfig()

    const response = await fetch(`${managementEndpoint.value}?name=${encodeURIComponent(file.name)}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${managementToken.value}` },
    })
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    ElMessage.success(`账号 ${file.account} 已删除`)
    await loadFiles()
  } catch (e) {
    ElMessage.error('删除失败: ' + (e instanceof Error ? e.message : String(e)))
  } finally {
    deletingId.value = null
  }
}

async function cleanAllInvalid() {
  const targets = invalidFiles.value
  if (targets.length === 0) return

  try {
    await ElMessageBox.confirm(
      `确定要删除全部 ${targets.length} 个 Token 失效账号吗？此操作不可撤销。`,
      '批量清理确认',
      { type: 'warning' },
    )
  } catch {
    return
  }

  cleaningInvalid.value = true
  let successCount = 0
  let failCount = 0
  try {
    await ensureManagementConfig()

    for (const file of targets) {
      try {
        const response = await fetch(`${managementEndpoint.value}?name=${encodeURIComponent(file.name)}`, {
          method: 'DELETE',
          headers: { Authorization: `Bearer ${managementToken.value}` },
        })
        if (response.ok) {
          successCount++
        } else {
          failCount++
        }
      } catch {
        failCount++
      }
    }
    ElMessage.success(`清理完成，成功 ${successCount}，失败 ${failCount}`)
    await loadFiles()
  } finally {
    cleaningInvalid.value = false
  }
}

function handleSchedulerLogPageChange(page: number) {
  schedulerLogQuery.page = page
  void loadSchedulerLogs()
}

function handleSchedulerLogPageSizeChange(pageSize: number) {
  schedulerLogQuery.pageSize = pageSize
  schedulerLogQuery.page = 1
  void loadSchedulerLogs()
}

function formatDate(value?: string) {
  if (!value) return '-'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatRecoveryTime(file: AuthFile) {
  const resetAt = usageLimitResetAt(file)
  if (resetAt === null) return '-'
  return formatDate(new Date(resetAt).toISOString())
}

function statusTagType(file: AuthFile) {
  if (usageLimitState(file) === 'limited') {
    return 'warning'
  }

  switch (effectiveStatus(file)) {
    case 'active':
      return 'success'
    case 'disabled':
      return 'info'
    default:
      return 'warning'
  }
}

function statusLabel(file: AuthFile) {
  if (usageLimitState(file) === 'limited' && file.disabled) {
    return '限额禁用'
  }
  if (usageLimitState(file) === 'limited') {
    return '限额中'
  }
  return effectiveStatus(file)
}

function schedulerActionLabel(action: string) {
  switch (action) {
    case 'disable_invalid':
      return '禁用失效账号'
    case 'delete_invalid':
      return '删除失效账号'
    default:
      return action || '-'
  }
}

function schedulerTriggerLabel(triggerType: string) {
  switch (triggerType) {
    case 'manual':
      return '手动执行'
    case 'retry':
      return '失败重试'
    case 'scheduled':
      return '定时执行'
    default:
      return triggerType || '-'
  }
}

function schedulerRunStatusLabel(status?: string) {
  switch (status) {
    case 'success':
      return '成功'
    case 'partial_failed':
      return '部分失败'
    case 'failed':
      return '失败'
    default:
      return status || '-'
  }
}

function schedulerRunStatusTagType(status?: string) {
  switch (status) {
    case 'success':
      return 'success'
    case 'partial_failed':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'info'
  }
}

function schedulerLogMessage(log: SchedulerLog) {
  if (log.error_message) return log.error_message
  if (log.messages && log.messages.length > 0) return log.messages[0]
  return '-'
}

onMounted(() => {
  void Promise.all([loadFiles(), loadSchedulerState(true), loadSchedulerLogs()])
  schedulerPollingTimer = setInterval(() => {
    void Promise.all([loadSchedulerState(false, true), loadSchedulerLogs(true)])
  }, 30000)
})

onBeforeUnmount(() => {
  if (schedulerPollingTimer) {
    clearInterval(schedulerPollingTimer)
    schedulerPollingTimer = null
  }
})
</script>

<style scoped>
.online-accounts-page {
  display: grid;
  gap: 20px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 20px;
}

.summary-card {
  display: grid;
  gap: 8px;
}

.summary-card__label {
  color: #52606d;
  font-size: 14px;
}

.summary-card__value {
  font-size: 30px;
  line-height: 1;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filters {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(180px, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.page-title {
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 600;
}

.page-subtitle {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.scheduler-panel {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(320px, 0.95fr);
  gap: 20px;
}

.scheduler-form {
  display: grid;
  gap: 16px;
}

.scheduler-switches {
  display: grid;
  gap: 12px;
}

.scheduler-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(241, 245, 249, 0.86));
}

.scheduler-toggle strong {
  display: block;
  font-size: 14px;
  color: #0f172a;
}

.scheduler-toggle p {
  margin: 6px 0 0;
  font-size: 12px;
  color: #64748b;
}

.scheduler-config-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.scheduler-config-card {
  display: grid;
  gap: 12px;
  padding: 16px 18px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(248, 250, 252, 0.92);
}

.scheduler-config-card--times {
  grid-column: span 2;
}

.scheduler-config-card__label {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.scheduler-config-card__hint {
  margin: 0;
  font-size: 12px;
  color: #64748b;
}

.scheduler-inline-fields {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.scheduler-inline-fields__suffix {
  font-size: 13px;
  color: #475569;
}

.scheduler-retry-grid {
  display: grid;
  gap: 12px;
}

.scheduler-actions-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.scheduler-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.scheduler-hint {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.scheduler-status {
  display: grid;
  gap: 16px;
  padding: 20px;
  border-radius: 22px;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.12), transparent 34%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.92));
  color: #e2e8f0;
}

.scheduler-status__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.scheduler-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.scheduler-metric {
  display: grid;
  gap: 6px;
  padding: 14px;
  border-radius: 16px;
  background: rgba(15, 23, 42, 0.28);
  border: 1px solid rgba(148, 163, 184, 0.18);
}

.scheduler-metric span {
  font-size: 12px;
  color: rgba(226, 232, 240, 0.72);
}

.scheduler-metric strong {
  font-size: 15px;
  line-height: 1.5;
}

.scheduler-result {
  display: grid;
  gap: 12px;
}

.scheduler-result__actions {
  display: grid;
  gap: 10px;
}

.scheduler-result__label,
.scheduler-result__status {
  font-size: 12px;
  color: rgba(226, 232, 240, 0.76);
  margin: 0;
}

.scheduler-result__tags,
.log-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.scheduler-result__messages {
  display: grid;
  gap: 8px;
}

.scheduler-result__messages p {
  margin: 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.32);
  font-size: 12px;
  line-height: 1.6;
  color: rgba(226, 232, 240, 0.88);
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.pagination--logs {
  margin-top: 18px;
}

@media (max-width: 1200px) {
  .summary-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .scheduler-panel {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .filters,
  .scheduler-config-grid,
  .scheduler-metrics {
    grid-template-columns: 1fr;
  }

  .scheduler-config-card--times {
    grid-column: auto;
  }
}

@media (max-width: 640px) {
  .summary-grid {
    grid-template-columns: 1fr;
  }

  .scheduler-toggle {
    align-items: flex-start;
    flex-direction: column;
  }

  .scheduler-actions-row,
  .scheduler-actions {
    width: 100%;
  }
}
</style>
