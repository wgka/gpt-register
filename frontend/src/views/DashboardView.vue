<template>
  <div class="dashboard">
    <section class="hero page-card">
      <div class="hero__copy">
        <span class="hero__eyebrow">Registration</span>
        <h1 class="hero__title">开始注册</h1>
      </div>
      <el-button class="hero__refresh" :loading="loading" type="primary" @click="refreshData">刷新</el-button>
    </section>

    <section class="console-grid">
      <el-card class="page-card launch-card" shadow="never">
        <template #header>
          <div class="section-header">
            <h3 class="page-title">开始注册</h3>
            <el-tag class="section-tag" type="info">今日 {{ registrationStats.today_count ?? 0 }}</el-tag>
          </div>
        </template>

        <el-form class="launch-form" label-position="top" @submit.prevent>
          <div class="launch-fields">
            <el-form-item label="数量">
              <el-input-number v-model="form.count" :min="1" :max="100" controls-position="right" />
            </el-form-item>
            <el-form-item label="并发数">
              <el-input-number
                v-model="form.concurrency"
                :min="1"
                :max="Math.min(20, form.count)"
                controls-position="right"
              />
            </el-form-item>
            <el-form-item label="最小间隔（秒）">
              <el-input-number v-model="form.interval_min" :min="0" :max="3600" controls-position="right" />
            </el-form-item>
            <el-form-item label="最大间隔（秒）">
              <el-input-number v-model="form.interval_max" :min="0" :max="3600" controls-position="right" />
            </el-form-item>
          </div>
          <div class="form-actions">
            <el-button class="action-btn" type="primary" :loading="starting" @click="startRegistration">开始</el-button>
            <el-button
              class="action-btn"
              type="danger"
              plain
              :disabled="!socket || !canCancel"
              @click="cancelRegistration"
            >
              停止
            </el-button>
          </div>
        </el-form>

        <el-divider content-position="left">最近成功账号</el-divider>
        <div v-if="recentResults.length === 0" class="result-empty">
          注册成功后会在这里显示绑卡链接，支持直接复制。
        </div>
        <div v-else class="result-list">
          <article v-for="item in recentResults" :key="item.task_uuid" class="result-card">
            <div class="result-card__header">
              <div>
                <strong>{{ item.email }}</strong>
                <p class="result-card__meta">
                  {{ item.source === 'login' ? '已存在账号登录' : '新注册账号' }}
                </p>
              </div>
              <el-button
                link
                type="primary"
                :disabled="!item.bind_card_url"
                @click="copyValue(item.bind_card_url, '绑卡链接')"
              >
                复制链接
              </el-button>
            </div>
            <div class="result-row">
              <span class="result-row__label">绑卡链接</span>
              <code>{{ item.bind_card_url_summary || '-' }}</code>
            </div>
            <div class="result-row result-row--meta">
              <span>账号 ID {{ item.account_id || '-' }}</span>
              <span>工作区 {{ item.workspace_id || '-' }}</span>
            </div>
          </article>
        </div>
      </el-card>

      <el-card class="page-card log-card" shadow="never">
        <template #header>
          <div class="section-header">
            <h3 class="page-title">实时日志</h3>
            <div class="status-bar">
              <el-tag :type="statusTagType(current.status)">{{ current.status || '-' }}</el-tag>
              <el-tag :type="websocketTagType">{{ websocketState }}</el-tag>
            </div>
          </div>
        </template>

        <div class="summary-grid">
          <div class="summary-item">
            <span class="summary-item__label">总数</span>
            <strong>{{ current.total ?? 0 }}</strong>
          </div>
          <div class="summary-item">
            <span class="summary-item__label">完成</span>
            <strong>{{ current.completed ?? 0 }}</strong>
          </div>
          <div class="summary-item">
            <span class="summary-item__label">成功</span>
            <strong>{{ current.success ?? 0 }}</strong>
          </div>
          <div class="summary-item">
            <span class="summary-item__label">失败</span>
            <strong>{{ current.failed ?? 0 }}</strong>
          </div>
        </div>

        <div class="log-panel">
          <div v-if="logs.length === 0" class="log-empty">暂无日志</div>
          <div v-else class="log-lines">
            <p v-for="(line, index) in logs" :key="`log-${index}`">{{ line }}</p>
          </div>
        </div>
      </el-card>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'

type RegistrationStats = {
  by_status: Record<string, number>
  today_count: number
}

type BatchResponse = {
  batch_id: string
}

type TaskEvent = {
  type: 'status' | 'log' | 'result'
  batch_id?: string
  task_uuid?: string
  status?: string
  message?: string
  extra?: Record<string, unknown>
}

type RecentRegistrationResult = {
  task_uuid: string
  email: string
  account_id: string
  workspace_id: string
  source: string
  bind_card_url: string
  bind_card_url_summary: string
}

const loading = ref(false)
const starting = ref(false)
const socket = ref<WebSocket | null>(null)
const websocketState = ref('未连接')
const logs = ref<string[]>([])
const recentResults = ref<RecentRegistrationResult[]>([])
const activeBatchID = ref('')
const registrationStats = reactive<RegistrationStats>({
  by_status: {},
  today_count: 0,
})
const form = reactive({
  count: 1,
  concurrency: 1,
  interval_min: 5,
  interval_max: 15,
})
const current = reactive({
  status: '',
  total: 0,
  completed: 0,
  success: 0,
  failed: 0,
})

async function refreshData() {
  loading.value = true
  try {
    await loadStats()
  } catch {
    ElMessage.error('刷新失败')
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  const response = await fetch('/api/registration/stats')
  if (!response.ok) {
    throw new Error('load stats failed')
  }
  const payload = (await response.json()) as RegistrationStats
  registrationStats.by_status = payload.by_status ?? {}
  registrationStats.today_count = payload.today_count ?? 0
}

async function startRegistration() {
  if (form.interval_max < form.interval_min) {
    ElMessage.warning('最大间隔不能小于最小间隔')
    return
  }

  starting.value = true
  try {
    resetCurrent()
    logs.value = []
    recentResults.value = []

    const response = await fetch('/api/registration/batch', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        count: form.count,
        concurrency: form.concurrency,
        interval_min: form.interval_min,
        interval_max: form.interval_max,
        email_service_type: 'meteormail',
      }),
    })
    if (!response.ok) {
      throw new Error('start failed')
    }

    const payload = (await response.json()) as BatchResponse
    activeBatchID.value = payload.batch_id
    current.status = 'running'
    current.total = form.count
    connectSocket(payload.batch_id)
    ElMessage.success('已开始')
  } catch {
    ElMessage.error('启动失败')
  } finally {
    starting.value = false
  }
}

function connectSocket(batchID: string) {
  closeSocket()
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${protocol}//${window.location.host}/ws/batch/${batchID}`)
  socket.value = ws
  websocketState.value = '连接中'

  ws.onopen = () => {
    websocketState.value = '已连接'
  }

  ws.onmessage = (event) => {
    const payload = JSON.parse(event.data) as TaskEvent
    applyEvent(payload)
  }

  ws.onerror = () => {
    websocketState.value = '连接异常'
  }

  ws.onclose = () => {
    if (socket.value === ws) {
      socket.value = null
    }
    websocketState.value = activeBatchID.value ? '已断开' : '未连接'
  }
}

function applyEvent(payload: TaskEvent) {
  const extra = payload.extra ?? {}

  if (payload.type === 'status') {
    current.status = payload.status || current.status
    current.total = asNumber(extra.total, current.total)
    current.completed = asNumber(extra.completed, current.completed)
    current.success = asNumber(extra.success, current.success)
    current.failed = asNumber(extra.failed, current.failed)

    if (payload.message) {
      appendLog(payload.message)
    }

    if (isTerminalStatus(current.status)) {
      void loadStats()
    }
    return
  }

  if (payload.type === 'log' && payload.message) {
    appendLog(payload.message)
    return
  }

  if (payload.type === 'result') {
    const result = normalizeRecentResult(payload)
    if (!result) {
      return
    }

    recentResults.value = [result, ...recentResults.value.filter((item) => item.task_uuid !== result.task_uuid)].slice(0, 8)
  }
}

function appendLog(message: string) {
  logs.value = [...logs.value, message].slice(-400)
}

function normalizeRecentResult(payload: TaskEvent): RecentRegistrationResult | null {
  const extra = payload.extra ?? {}
  const taskUUID = asString(extra.task_uuid) || payload.task_uuid || `${Date.now()}`
  const email = asString(extra.email)
  if (!email) {
    return null
  }

  return {
    task_uuid: taskUUID,
    email,
    account_id: asString(extra.account_id),
    workspace_id: asString(extra.workspace_id),
    source: asString(extra.source) || 'register',
    bind_card_url: asString(extra.bind_card_url),
    bind_card_url_summary: asString(extra.bind_card_url_summary),
  }
}

function cancelRegistration() {
  if (!socket.value) {
    return
  }
  socket.value.send(JSON.stringify({ type: 'cancel' }))
}

function closeSocket() {
  if (socket.value) {
    socket.value.close()
    socket.value = null
  }
  websocketState.value = '未连接'
}

function resetCurrent() {
  activeBatchID.value = ''
  current.status = ''
  current.total = 0
  current.completed = 0
  current.success = 0
  current.failed = 0
}

function statusTagType(status: string) {
  switch (status) {
    case 'completed':
      return 'success'
    case 'running':
    case 'cancelling':
      return 'warning'
    case 'failed':
      return 'danger'
    case 'cancelled':
      return 'info'
    default:
      return 'info'
  }
}

function isTerminalStatus(status: string) {
  return status === 'completed' || status === 'failed' || status === 'cancelled'
}

function asNumber(value: unknown, fallback: number) {
  return typeof value === 'number' ? value : fallback
}

function asString(value: unknown) {
  return typeof value === 'string' ? value : ''
}

async function copyValue(value: string, label: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    ElMessage.warning(`${label} 不可复制`)
    return
  }

  try {
    await writeClipboard(trimmed)
    ElMessage.success(`${label} 已复制`)
  } catch {
    ElMessage.error(`${label} 复制失败`)
  }
}

async function writeClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

const canCancel = computed(() => current.status === 'running' || current.status === 'cancelling')

const websocketTagType = computed<'info' | 'success' | 'warning'>(() => {
  switch (websocketState.value) {
    case '已连接':
      return 'success'
    case '连接中':
      return 'warning'
    default:
      return 'info'
  }
})

onMounted(() => {
  refreshData().catch(() => {
    ElMessage.error('初始化失败')
  })
})

onBeforeUnmount(() => {
  closeSocket()
})
</script>

<style scoped>
.dashboard {
  display: grid;
  gap: 24px;
}

.hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 28px 32px;
}

.hero__copy {
  display: grid;
  gap: 8px;
}

.hero__eyebrow {
  color: #5b6b88;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.hero__title {
  margin: 0;
  font-size: 36px;
  line-height: 1;
  letter-spacing: -0.05em;
}

.hero__refresh {
  min-width: 112px;
}

.console-grid {
  display: grid;
  grid-template-columns: minmax(320px, 380px) minmax(0, 1fr);
  gap: 24px;
}

.section-header,
.status-bar,
.form-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.launch-form {
  display: grid;
  gap: 18px;
}

.result-empty {
  padding: 18px 16px;
  border-radius: 16px;
  background: #f8fafc;
  color: #64748b;
  font-size: 13px;
}

.result-list {
  display: grid;
  gap: 12px;
}

.result-card {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96) 0%, rgba(241, 245, 249, 0.96) 100%);
}

.result-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.result-card__meta {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 12px;
}

.result-row {
  display: grid;
  gap: 6px;
}

.result-row__label {
  color: #52606d;
  font-size: 12px;
  font-weight: 700;
}

.result-row code {
  font-size: 12px;
  word-break: break-all;
  white-space: pre-wrap;
}

.result-row--meta {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  color: #64748b;
  font-size: 12px;
}

.launch-card :deep(.el-card__header),
.log-card :deep(.el-card__header) {
  padding: 24px 24px 18px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.14);
}

.launch-card :deep(.el-card__body),
.log-card :deep(.el-card__body) {
  padding: 24px;
}

.launch-fields {
  display: grid;
  gap: 16px;
}

.launch-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.launch-form :deep(.el-form-item__label) {
  margin-bottom: 10px;
  color: #516076;
  font-weight: 700;
}

.launch-form :deep(.el-input-number) {
  width: 100%;
}

.launch-form :deep(.el-input-number .el-input__wrapper) {
  border-radius: 14px;
}

.action-btn {
  min-width: 120px;
  height: 44px;
  border-radius: 14px;
  font-weight: 700;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.summary-item {
  display: grid;
  gap: 6px;
  padding: 16px 18px;
  border-radius: 18px;
  background: linear-gradient(180deg, #f8fbff 0%, #f2f6fb 100%);
  border: 1px solid rgba(148, 163, 184, 0.12);
}

.summary-item__label {
  color: #64748b;
  font-size: 12px;
  font-weight: 600;
}

.summary-item strong {
  font-size: 24px;
  letter-spacing: -0.04em;
}

.log-panel {
  min-height: 520px;
  max-height: 640px;
  overflow: auto;
  padding: 18px 20px;
  border-radius: 22px;
  background:
    radial-gradient(circle at top right, rgba(34, 211, 238, 0.12), transparent 22%),
    linear-gradient(180deg, #11182b 0%, #121a2f 100%);
  color: #e2e8f0;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.log-empty {
  color: #75839d;
}

.log-lines {
  display: grid;
  gap: 8px;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 12px;
  line-height: 1.6;
}

.log-lines p {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 1100px) {
  .console-grid,
  .summary-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .hero,
  .section-header,
  .status-bar,
  .form-actions,
  .result-card__header {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero {
    padding: 24px 20px;
  }

  .hero__title {
    font-size: 28px;
  }

  .result-row--meta {
    grid-template-columns: 1fr;
  }
}
</style>
