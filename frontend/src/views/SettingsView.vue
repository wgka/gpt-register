<template>
  <div class="settings-page">
    <section class="hero page-card">
      <div>
        <span class="hero__eyebrow">Settings</span>
        <h1 class="page-title">运行配置</h1>
        <p class="page-subtitle">代理、CPA 自动上传和 Telegram Bot 都可以在这里维护。</p>
      </div>
      <div class="hero__actions">
        <el-button plain :loading="loading" @click="loadSettings">刷新</el-button>
        <el-button type="primary" :loading="saving" @click="saveSettings">保存设置</el-button>
      </div>
    </section>

    <section class="settings-grid">
      <el-card class="page-card" shadow="never">
        <template #header>
          <div class="section-header">
            <h3 class="section-title">代理设置</h3>
            <el-tag type="info">Registration</el-tag>
          </div>
        </template>

        <el-form label-position="top" class="config-form">
          <el-form-item label="固定代理 URL">
            <el-input
              v-model="form.proxy.url"
              clearable
              placeholder="http://127.0.0.1:7897 或 socks5://127.0.0.1:7890"
            />
          </el-form-item>
          <el-form-item label="动态代理 API URL">
            <el-input
              v-model="form.proxy.api_url"
              clearable
              placeholder="返回代理池 JSON 的接口地址"
            />
          </el-form-item>

          <div class="inline-grid">
            <el-form-item label="最大尝试次数">
              <el-input-number v-model="form.proxy.attempts" :min="1" :max="20" controls-position="right" />
            </el-form-item>
            <el-form-item label="预检超时（秒）">
              <el-input-number
                v-model="form.proxy.preflight_timeout"
                :min="3"
                :max="60"
                controls-position="right"
              />
            </el-form-item>
          </div>

          <el-alert
            type="info"
            show-icon
            :closable="false"
            title="如果同时填写动态代理 API 和固定代理，系统会优先尝试动态代理，失败后回退到固定代理。"
          />
        </el-form>
      </el-card>

      <el-card class="page-card" shadow="never">
        <template #header>
          <div class="section-header">
            <h3 class="section-title">CPA 控制</h3>
            <el-tag :type="form.cpa.enabled ? 'success' : 'info'">
              {{ form.cpa.enabled ? '已启用' : '未启用' }}
            </el-tag>
          </div>
        </template>

        <el-form label-position="top" class="config-form">
          <el-form-item>
            <template #label>
              <span>自动上传开关</span>
            </template>
            <el-switch v-model="form.cpa.enabled" inline-prompt active-text="开" inactive-text="关" />
          </el-form-item>
          <el-form-item label="CPA API URL">
            <el-input v-model="form.cpa.api_url" clearable placeholder="http://host:port/v0/management/auth-files" />
          </el-form-item>
          <el-form-item label="CPA API Token">
            <el-input v-model="form.cpa.api_token" type="password" show-password clearable placeholder="输入 CPA Token" />
          </el-form-item>
          <el-form-item label="CPA 专用代理（可选）">
            <el-input
              v-model="form.cpa.proxy_url"
              clearable
              placeholder="留空则直连；可填写 http/socks5 代理"
            />
          </el-form-item>
        </el-form>
      </el-card>

      <el-card class="page-card" shadow="never">
        <template #header>
          <div class="section-header">
            <h3 class="section-title">Telegram Bot</h3>
            <el-tag type="warning">需重启生效</el-tag>
          </div>
        </template>

        <el-form label-position="top" class="config-form">
          <el-form-item label="TELEGRAM_BOT_TOKEN">
            <el-input
              v-model="form.telegram.bot_token"
              type="password"
              show-password
              clearable
              placeholder="不填则不会启动 Telegram Bot"
            />
          </el-form-item>
          <el-form-item label="允许的 chat_id 白名单">
            <el-input
              v-model="form.telegram.allowed_chat_ids"
              clearable
              placeholder="多个 chat_id 用英文逗号分隔"
            />
          </el-form-item>
          <el-form-item>
            <template #label>
              <span>调试日志</span>
            </template>
            <el-switch v-model="form.telegram.debug" inline-prompt active-text="开" inactive-text="关" />
          </el-form-item>

          <el-alert
            type="warning"
            show-icon
            :closable="false"
            :title="state.editable.telegram.restart_hint || 'Telegram Bot Token 变更后请重启应用'"
          />
        </el-form>
      </el-card>
    </section>

    <el-card class="page-card" shadow="never">
      <template #header>
        <h3 class="section-title">当前运行信息</h3>
      </template>

      <el-descriptions :column="2" border class="runtime-grid">
        <el-descriptions-item label="应用名称">
          {{ state.app?.name ?? '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="版本">
          {{ state.app?.version ?? '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="调试模式">
          <el-tag :type="state.app?.debug ? 'warning' : 'success'">
            {{ state.app?.debug ? '开启' : '关闭' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="监听地址">
          {{ state.runtime?.addr ?? '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="数据库">
          {{ state.runtime?.database_url ?? '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="日志文件">
          {{ state.runtime?.log_file ?? '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <div class="sections-grid">
      <el-card
        v-for="section in state.sections"
        :key="section.category"
        class="page-card"
        shadow="never"
      >
        <template #header>
          <div class="section-header">
            <h3 class="section-title">{{ section.title }}</h3>
            <el-tag type="info">{{ section.category }}</el-tag>
          </div>
        </template>

        <el-table :data="section.items" stripe>
          <el-table-column prop="name" label="字段" min-width="180" />
          <el-table-column prop="db_key" label="数据库键" min-width="180" />
          <el-table-column prop="description" label="说明" min-width="180" />
          <el-table-column prop="type" label="类型" width="100" />
          <el-table-column label="当前值" min-width="220">
            <template #default="{ row }">
              <el-tag v-if="row.secret" type="warning" effect="plain">secret</el-tag>
              <span class="value-text">{{ row.value }}</span>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'

type SettingsItem = {
  name: string
  db_key: string
  description: string
  type: string
  secret: boolean
  value: string | number | boolean
}

type SettingsSection = {
  category: string
  title: string
  items: SettingsItem[]
}

type EditableSettings = {
  proxy: {
    url: string
    api_url: string
    attempts: number
    preflight_timeout: number
  }
  cpa: {
    enabled: boolean
    api_url: string
    api_token: string
    proxy_url: string
  }
  telegram: {
    bot_token: string
    allowed_chat_ids: string
    debug: boolean
    restart_hint: string
  }
}

type SettingsResponse = {
  app: {
    name: string
    version: string
    debug: boolean
  }
  runtime: {
    addr: string
    database_url: string
    database_driver: string
    log_file: string
  }
  sections: SettingsSection[]
  editable: EditableSettings
}

const loading = ref(false)
const saving = ref(false)

const state = reactive<SettingsResponse>({
  app: {
    name: '',
    version: '',
    debug: false,
  },
  runtime: {
    addr: '',
    database_url: '',
    database_driver: '',
    log_file: '',
  },
  sections: [],
  editable: {
    proxy: {
      url: '',
      api_url: '',
      attempts: 4,
      preflight_timeout: 12,
    },
    cpa: {
      enabled: false,
      api_url: '',
      api_token: '',
      proxy_url: '',
    },
    telegram: {
      bot_token: '',
      allowed_chat_ids: '',
      debug: false,
      restart_hint: '',
    },
  },
})

const form = reactive<EditableSettings>({
  proxy: {
    url: '',
    api_url: '',
    attempts: 4,
    preflight_timeout: 12,
  },
  cpa: {
    enabled: false,
    api_url: '',
    api_token: '',
    proxy_url: '',
  },
  telegram: {
    bot_token: '',
    allowed_chat_ids: '',
    debug: false,
    restart_hint: '',
  },
})

function syncEditable(editable: EditableSettings) {
  state.editable = editable
  form.proxy.url = editable.proxy.url
  form.proxy.api_url = editable.proxy.api_url
  form.proxy.attempts = editable.proxy.attempts
  form.proxy.preflight_timeout = editable.proxy.preflight_timeout

  form.cpa.enabled = editable.cpa.enabled
  form.cpa.api_url = editable.cpa.api_url
  form.cpa.api_token = editable.cpa.api_token
  form.cpa.proxy_url = editable.cpa.proxy_url

  form.telegram.bot_token = editable.telegram.bot_token
  form.telegram.allowed_chat_ids = editable.telegram.allowed_chat_ids
  form.telegram.debug = editable.telegram.debug
  form.telegram.restart_hint = editable.telegram.restart_hint
}

async function loadSettings() {
  loading.value = true
  try {
    const response = await fetch('/api/settings')
    if (!response.ok) {
      throw new Error('load settings failed')
    }

    const payload = (await response.json()) as SettingsResponse
    state.app = payload.app
    state.runtime = payload.runtime
    state.sections = payload.sections
    syncEditable(payload.editable)
  } catch {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    const response = await fetch('/api/settings', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        editable: form,
      }),
    })
    if (!response.ok) {
      throw new Error('save settings failed')
    }

    const payload = (await response.json()) as {
      editable: EditableSettings
      message?: string
    }
    syncEditable(payload.editable)
    ElMessage.success(payload.message || '保存成功')
  } catch {
    ElMessage.error('保存配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadSettings()
})
</script>

<style scoped>
.settings-page {
  display: grid;
  gap: 20px;
}

.hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding: 28px 32px;
}

.hero__eyebrow {
  display: inline-flex;
  margin-bottom: 16px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: #6366f1;
}

.page-title {
  margin: 0;
  font-size: 26px;
  font-weight: 700;
  line-height: 1.2;
}

.page-subtitle {
  margin: 6px 0 0;
  color: #64748b;
  max-width: 460px;
  line-height: 1.6;
}

.hero__actions {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
  align-self: flex-end;
}

.hero__actions :deep(.el-button) {
  min-width: 96px;
  border-radius: 10px;
  font-weight: 500;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 20px;
}

.config-form {
  display: grid;
  gap: 4px;
}

.inline-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.runtime-grid {
  width: 100%;
}

.sections-grid {
  display: grid;
  gap: 20px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.section-title {
  margin: 0;
  font-size: 20px;
}

.value-text {
  margin-left: 8px;
  word-break: break-all;
}

@media (max-width: 900px) {
  .hero {
    flex-direction: column;
    align-items: stretch;
    padding: 24px;
  }

  .hero__actions {
    width: 100%;
  }

  .hero__actions :deep(.el-button) {
    flex: 1;
  }

  .page-subtitle {
    max-width: none;
  }

  .inline-grid {
    grid-template-columns: 1fr;
  }
}
</style>
