<template>
  <div class="flex flex-col h-screen w-screen overflow-hidden bg-bg-primary text-text-primary font-sans">
    <!-- 自定义标题栏 -->
    <TitleBar />

    <el-container class="flex-1 min-h-0">
      <!-- 侧边栏 -->
      <el-aside :width="collapsed ? '64px' : '224px'" class="transition-all duration-300 ease-in-out bg-bg-secondary border-r border-border flex flex-col">
        <!-- 导航菜单 -->
        <el-menu
          :default-active="activeKey"
          :collapse="collapsed"
          :collapse-transition="false"
          class="border-r-0 flex-1 overflow-y-auto scrollbar-custom bg-transparent custom-menu"
          @select="handleNav"
        >
          <el-menu-item 
            v-for="item in menuItems" 
            :key="item.key" 
            :index="item.key"
          >
            <el-icon><span v-html="item.icon" class="w-5 h-5 flex items-center justify-center"></span></el-icon>
            <template #title>
              <span class="font-medium">{{ item.label }}</span>
            </template>
          </el-menu-item>
        </el-menu>

        <!-- 折叠按钮 -->
        <div class="p-3 flex justify-center cursor-pointer hover:bg-bg-hover text-text-muted hover:text-text-primary transition-colors border-t border-border" @click="collapsed = !collapsed" :title="collapsed ? '展开侧边栏' : '折叠侧边栏'">
          <el-icon :size="18" class="transition-transform duration-300" :class="collapsed ? 'rotate-180' : ''">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 17l-5-5 5-5M18 17l-5-5 5-5" />
            </svg>
          </el-icon>
        </div>
      </el-aside>

      <!-- 主内容区 -->
      <el-container class="flex-col bg-bg-primary min-w-0">


        <!-- 内容主体 -->
        <el-main class="p-6 overflow-auto scrollbar-custom relative">
          <router-view v-slot="{ Component }">
            <Suspense>
              <template #default>
                <keep-alive include="NetworkCalc">
                  <component :is="Component" />
                </keep-alive>
              </template>
              <template #fallback>
                <RouteLoading />
              </template>
            </Suspense>
          </router-view>
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import TitleBar from "@/components/common/TitleBar.vue";
import RouteLoading from "@/components/common/RouteLoading.vue";
import { useTheme } from "@/composables/useTheme";
import { useTaskexecStore } from "@/stores/taskexecStore";

const router = useRouter();
const route = useRoute();
const taskexecStore = useTaskexecStore();

// 初始化主题
useTheme();

const collapsed = ref(false);
const activeKey = ref("Devices");

const menuItems = [
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
    key: "Tracert",
    label: "路径探测",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
    </svg>`,
  },
  {
    key: "FileServers",
    label: "文件服务器",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
      <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
      <line x1="6" y1="6" x2="6.01" y2="6"/>
      <line x1="6" y1="18" x2="6.01" y2="18"/>
    </svg>`,
  },
  {
    key: "SNMPMib",
    label: "SNMP MIB",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
    </svg>`,
  },
  {
    key: "SNMPTraps",
    label: "Trap 告警",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
      <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
    </svg>`,
  },
  {
    key: "SNMPPolling",
    label: "设备轮询",
    icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
      <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
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

<style scoped>
.custom-menu {
  --el-menu-bg-color: transparent;
  --el-menu-text-color: var(--color-text-secondary);
  --el-menu-hover-text-color: var(--color-text-primary);
  --el-menu-active-color: var(--color-accent);
  padding: 8px;
  border-right: none;
}
.custom-menu :deep(.el-menu-item) {
  border-radius: 8px;
  margin-bottom: 4px;
  height: 44px;
  line-height: 44px;
  transition: all 0.2s;
  border: 1px solid transparent;
}
.custom-menu :deep(.el-menu-item:hover) {
  background-color: var(--color-bg-hover);
}
.custom-menu :deep(.el-menu-item.is-active) {
  background-color: var(--color-accent-bg);
  border-color: rgba(59, 130, 246, 0.3);
  box-shadow: var(--shadow-glow);
}
html.dark .custom-menu :deep(.el-menu-item.is-active) {
  border-color: rgba(96, 165, 250, 0.3);
}
</style>
