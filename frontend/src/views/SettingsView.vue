<template>
  <div class="settings-page">
    <el-card class="page-card" shadow="never">
      <template #header>
        <h3 class="page-title">运行配置</h3>
      </template>

      <el-descriptions :column="1" border>
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
import { onMounted, reactive } from 'vue'
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
}

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
})

async function loadSettings() {
  const response = await fetch('/api/settings')
  if (!response.ok) {
    throw new Error('load settings failed')
  }

  const payload = (await response.json()) as SettingsResponse
  state.app = payload.app
  state.runtime = payload.runtime
  state.sections = payload.sections
}

onMounted(() => {
  loadSettings().catch(() => {
    ElMessage.error('加载配置失败')
  })
})
</script>

<style scoped>
.settings-page {
  display: grid;
  gap: 20px;
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
</style>
