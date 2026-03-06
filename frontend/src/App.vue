<template>
  <div class="flex h-screen w-screen overflow-hidden bg-bg-dark text-text-primary font-sans">
    <!-- 侧边栏 -->
    <aside
      :class="[
        'flex flex-col transition-all duration-300 ease-in-out border-r border-border bg-bg-panel flex-shrink-0',
        collapsed ? 'w-16' : 'w-56'
      ]"
    >
      <!-- Logo 头部 -->
      <div class="flex items-center gap-3 px-4 py-5 border-b border-border h-16 overflow-hidden">
        <div class="flex-shrink-0 w-8 h-8 rounded-lg bg-accent flex items-center justify-center shadow-glow">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M2 12h3M19 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12"/>
          </svg>
        </div>
        <div v-if="!collapsed" class="overflow-hidden animate-fade-in">
          <div class="text-sm font-semibold text-text-primary whitespace-nowrap">NetWeaverGo</div>
          <div class="text-xs text-text-muted whitespace-nowrap">Control Center</div>
        </div>
      </div>

      <!-- 导航菜单 -->
      <nav class="flex-1 py-4 px-2 space-y-1">
        <button
          v-for="item in menuItems"
          :key="item.key"
          @click="handleNav(item.key)"
          :class="[
            'w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 group',
            activeKey === item.key
              ? 'bg-accent/20 text-accent border border-accent/30 shadow-glow'
              : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary border border-transparent'
          ]"
        >
          <span class="flex-shrink-0 w-5 h-5" v-html="item.icon"></span>
          <span v-if="!collapsed" class="animate-fade-in whitespace-nowrap overflow-hidden">{{ item.label }}</span>
        </button>
      </nav>

      <!-- 折叠按钮 -->
      <div class="p-3 border-t border-border">
        <button
          @click="collapsed = !collapsed"
          class="w-full flex items-center justify-center p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-all duration-200"
          :title="collapsed ? '展开侧边栏' : '折叠侧边栏'"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4 transition-transform duration-300"
            :class="collapsed ? 'rotate-180' : ''"
            viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
          >
            <path d="M11 17l-5-5 5-5M18 17l-5-5 5-5"/>
          </svg>
        </button>
      </div>
    </aside>

    <!-- 主内容区 -->
    <div class="flex flex-col flex-1 min-w-0">
      <!-- 顶部导航栏 -->
      <header class="flex items-center justify-between px-6 h-16 border-b border-border bg-bg-panel flex-shrink-0">
        <div class="flex items-center gap-3">
          <h1 class="text-base font-semibold text-text-primary">{{ currentTitle }}</h1>
          <span class="hidden sm:flex items-center gap-1.5 text-xs text-text-muted bg-bg-card px-2.5 py-1 rounded-full border border-border">
            <span class="w-1.5 h-1.5 rounded-full bg-success animate-pulse-slow"></span>
            在线
          </span>
        </div>
        <div class="text-xs text-text-muted font-mono">v1.0</div>
      </header>

      <!-- 内容主体 -->
      <main class="flex-1 overflow-auto scrollbar-custom bg-bg-dark p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

const collapsed = ref(false)
const activeKey = ref('Dashboard')

const menuItems = [
  {
    key: 'Dashboard',
    label: '概览仪表盘',
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
      <rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/>
    </svg>`,
  },
  {
    key: 'Devices',
    label: '设备资产',
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/>
      <line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
    </svg>`,
  },
  {
    key: 'Tasks',
    label: '任务执行',
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
    </svg>`,
  },
]

const titleMap: Record<string, string> = {
  Dashboard: '概览仪表盘',
  Devices:   '设备资产清单',
  Tasks:     '任务执行大屏',
}
const currentTitle = computed(() => titleMap[activeKey.value] ?? 'NetWeaverGo')

watch(() => route.name, (name) => {
  if (name) activeKey.value = name as string
}, { immediate: true })

function handleNav(key: string) {
  activeKey.value = key
  router.push({ name: key })
}
</script>
