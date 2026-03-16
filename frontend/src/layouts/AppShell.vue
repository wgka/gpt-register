<template>
  <el-container class="shell">
    <el-aside class="shell__aside" width="240px">
      <div class="brand">
        <h1 class="brand__title">Codex Register</h1>
      </div>

      <el-menu :default-active="activePath" class="menu" router>
        <el-menu-item index="/">
          <span>执行台</span>
        </el-menu-item>
        <el-menu-item index="/accounts">
          <span>账号管理</span>
        </el-menu-item>
        <el-menu-item index="/settings">
          <span>设置页</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="shell__header">
        <div>
          <h2 class="shell__heading">{{ heading.title }}</h2>
        </div>
      </el-header>

      <el-main class="shell__main">
        <div class="shell__content">
          <RouterView />
        </div>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

const heading = computed(() => {
  switch (route.name) {
    case 'accounts':
      return {
        title: '账号管理',
      }
    case 'settings':
      return {
        title: '系统设置页',
      }
    default:
      return {
        title: '注册执行台',
      }
  }
})

const activePath = computed(() => route.path)
</script>

<style scoped>
.shell {
  min-height: 100vh;
  background: transparent;
}

.shell__aside {
  padding: 32px 18px;
  border-right: 1px solid rgba(148, 163, 184, 0.18);
  background: linear-gradient(180deg, #182033 0%, #20293f 100%);
}

.brand {
  padding: 8px 10px 28px;
  color: #e2e8f0;
}

.brand__title {
  margin: 0;
  font-size: 22px;
  line-height: 1.05;
  letter-spacing: -0.03em;
}

.menu {
  border-right: none;
  background: transparent;
}

.menu :deep(.el-menu-item) {
  height: 52px;
  margin-bottom: 12px;
  border-radius: 16px;
  color: #c7d2e5;
  font-weight: 600;
}

.menu :deep(.el-menu-item.is-active) {
  color: #113043;
  background: linear-gradient(135deg, #7dd3fc, #a7f3d0);
  box-shadow: 0 14px 30px rgba(125, 211, 252, 0.24);
}

.shell__header {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 92px;
  padding: 26px 36px 18px;
}

.shell__heading {
  margin: 0;
  font-size: 34px;
  letter-spacing: -0.04em;
}

.shell__main {
  padding: 0 36px 36px;
  display: flex;
  justify-content: center;
}

.shell__content {
  width: min(1180px, 100%);
}

@media (max-width: 900px) {
  .shell {
    flex-direction: column;
  }

  .shell__aside {
    width: 100%;
  }

  .shell__header {
    justify-content: flex-start;
    padding: 24px 20px 16px;
  }

  .shell__heading {
    font-size: 28px;
  }

  .shell__main {
    padding: 0 20px 24px;
  }
}
</style>
