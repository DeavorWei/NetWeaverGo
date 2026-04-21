<template>
  <div
    class="flex flex-col h-screen w-screen overflow-hidden bg-bg-primary text-text-primary font-sans"
  >
    <!-- 自定义标题栏 -->
    <TitleBar />

    <!-- 主内容区域 -->
    <div class="flex flex-1 min-h-0">
      <!-- 侧边栏 -->
      <aside
        :class="[
          'flex flex-col transition-all duration-300 ease-in-out bg-bg-secondary flex-shrink-0',
          collapsed ? 'w-16' : 'w-56',
        ]"
      >
        <!-- 导航菜单 -->
        <nav class="flex-1 py-4 px-2 space-y-1 overflow-y-auto">
          <button
            v-for="item in menuItems"
            :key="item.key"
            @click="handleNav(item.key)"
            :class="[
              'w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 group',
              activeKey === item.key
                ? 'bg-accent-bg text-accent border border-accent/30 shadow-glow'
                : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary border border-transparent',
            ]"
          >
            <span class="flex-shrink-0 w-5 h-5" v-html="item.icon"></span>
            <span
              v-if="!collapsed"
              class="animate-fade-in whitespace-nowrap overflow-hidden flex-1 text-left"
              >{{ item.label }}</span
            >
          </button>
        </nav>

        <!-- 折叠按钮 -->
        <div class="p-3">
          <button
            @click="collapsed = !collapsed"
            class="w-full flex items-center justify-center p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-hover transition-all duration-200"
            :title="collapsed ? '展开侧边栏' : '折叠侧边栏'"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-4 h-4 transition-transform duration-300"
              :class="collapsed ? 'rotate-180' : ''"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M11 17l-5-5 5-5M18 17l-5-5 5-5" />
            </svg>
          </button>
        </div>
      </aside>

      <!-- 主内容区 -->
      <div class="flex flex-col flex-1 min-w-0">
        <!-- 顶部导航栏 -->
        <header
          class="flex items-center justify-between px-6 h-16 bg-bg-secondary flex-shrink-0"
        >
          <div class="flex items-center gap-3">
            <h1 class="text-base font-semibold text-text-primary">
              {{ currentTitle }}
            </h1>
          </div>
          <div class="flex items-center gap-4">
            <!-- 使用主题切换组件 -->
            <ThemeSwitch />
            <div class="text-xs text-text-muted font-mono">v1.0</div>
          </div>
        </header>

        <!-- 内容主体 -->
        <main class="flex-1 overflow-auto scrollbar-custom bg-bg-primary p-6">
          <Suspense>
            <template #default>
              <router-view />
            </template>
            <template #fallback>
              <RouteLoading />
            </template>
          </Suspense>
        </main>
      </div>
    </div>
  </div>
  <!-- 全局 Toast 通知 -->
  <GlobalToast />
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import ThemeSwitch from "@/components/common/ThemeSwitch.vue";
import TitleBar from "@/components/common/TitleBar.vue";
import GlobalToast from "@/components/common/GlobalToast.vue";
import RouteLoading from "@/components/common/RouteLoading.vue";
import { useTheme } from "@/composables/useTheme";
import { useTaskexecStore } from "@/stores/taskexecStore";

const router = useRouter();
const route = useRoute();
const taskexecStore = useTaskexecStore();

// 初始化主题
useTheme();

const collapsed = ref(false);
const activeKey = ref("Dashboard");

const menuItems = [
  {
    key: "Dashboard",
    label: "仪表盘",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
      <rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/>
    </svg>`,
  },
  {
    key: "Devices",
    label: "设备资产",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/>
      <line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
    </svg>`,
  },
  {
    key: "Commands",
    label: "命令管理",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
    </svg>`,
  },
  {
    key: "Tasks",
    label: "任务创建",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
    </svg>`,
  },
  {
    key: "TaskExecution",
    label: "任务执行",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polygon points="5 3 19 12 5 21 5 3"/>
    </svg>`,
  },
  {
    key: "Topology",
    label: "拓扑图谱",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="5" cy="5" r="2"/><circle cx="19" cy="5" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="5" cy="19" r="2"/><circle cx="19" cy="19" r="2"/>
      <path d="M7 5h10M12 7v3M7 19h10M5 7v10M19 7v10"/>
    </svg>`,
  },
  {
    key: "PlanCompare",
    label: "规划比对",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <path d="M14 2v6h6"/>
      <path d="M9 13h6M9 17h6"/>
    </svg>`,
  },
  {
    key: "TopologyCommandConfig",
    label: "拓扑命令配置",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 3v18M3 12h18"/>
      <circle cx="12" cy="12" r="9"/>
      <path d="M7.5 12h9M12 7.5v9"/>
    </svg>`,
  },
  {
    key: "NetworkCalc",
    label: "子网计算",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/>
      <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
    </svg>`,
  },
  {
    key: "ConfigForge",
    label: "配置生成",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
    </svg>`,
  },
  {
    key: "ProtocolRef",
    label: "端口速查",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>
    </svg>`,
  },
  {
    key: "BatchPing",
    label: "批量 Ping",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/>
      <path d="M12 2v4M12 18v4M2 12h4M18 12h4"/>
    </svg>`,
  },

  {
    key: "Settings",
    label: "系统设置",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
    </svg>`,
  },
];

const titleMap: Record<string, string> = {
  Dashboard: "仪表盘",
  Devices: "设备资产清单",
  Commands: "命令组管理",
  Tasks: "任务创建",
  TaskExecution: "任务执行大屏",
  Topology: "拓扑图谱",
  PlanCompare: "规划比对",
  NetworkCalc: "子网计算器",
  ProtocolRef: "端口速查",
  ConfigForge: "配置生成",
  BatchPing: "批量 Ping 检测",
  TopologyCommandConfig: "拓扑命令配置中心",
  Settings: "系统设置",
};

const currentTitle = computed(() => titleMap[activeKey.value] ?? "NetWeaverGo");

watch(
  () => route.name,
  (name) => {
    if (name) {
      activeKey.value = name as string;
    }
  },
  { immediate: true },
);

function handleNav(key: string) {
  activeKey.value = key;
  router.push({ name: key });
}

onMounted(() => {
  taskexecStore.initListeners();

  // 预加载高频页面，提升用户体验
  setTimeout(() => {
    import("@/views/Devices.vue");
    import("@/views/Tasks.vue");
    import("@/views/TaskExecution.vue");
  }, 1000);
});

onUnmounted(() => {
  taskexecStore.cleanupListeners();
});
</script>
