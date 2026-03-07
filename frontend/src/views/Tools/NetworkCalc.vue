<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import IPv4Calc from '../../components/network/IPv4Calc.vue'
import IPv6Calc from '../../components/network/IPv6Calc.vue'

const router = useRouter()
const activeTab = ref<'ipv4' | 'ipv6'>('ipv4')

const versions = {
  ipv4: { version: '1.0', buildDate: '2026228' },
  ipv6: { version: '0.1', buildDate: '2026228' }
}

const currentVersionInfo = computed(() => versions[activeTab.value])

const goBack = () => {
  router.push('/')
}
</script>

<template>
  <div class="min-h-screen w-screen flex flex-col items-center py-6 md:py-12 relative bg-transparent px-3 sm:px-6">
    <div class="absolute top-1/4 left-1/4 w-96 h-96 bg-indigo-500/10 rounded-full blur-[120px] pointer-events-none"></div>
    <div class="absolute bottom-1/4 right-1/4 w-96 h-96 bg-sky-500/10 rounded-full blur-[100px] pointer-events-none"></div>

    <!-- 顶部导航 -->
    <div class="w-full max-w-4xl mb-6 flex flex-col md:flex-row md:items-center md:justify-between gap-4 relative z-10">
      <div>
        <h1 class="heading-1 mb-2">网络计算器</h1>
        <p class="text-desc">子网划分、掩码转换与网段规划</p>
        <p class="text-hint mt-1 transition-all">Version:{{ currentVersionInfo.version }} BuildDate {{ currentVersionInfo.buildDate }}</p>
      </div>
      
      <!-- 返回首页按钮 -->
      <button 
        @click="goBack" 
        class="inline-flex items-center justify-center px-4 py-2 border border-slate-300 dark:border-slate-600 rounded-lg shadow-sm text-sm font-medium text-slate-700 dark:text-slate-300 bg-white/50 dark:bg-slate-800/50 hover:bg-slate-50 dark:hover:bg-slate-700 backdrop-blur-sm transition-all self-start md:self-auto"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2 -ml-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
        </svg>
        返回首页
      </button>
    </div>

    <!-- V4/V6 切换开关 -->
    <div class="w-full max-w-4xl relative z-10 mb-6 flex justify-center">
      <div class="bg-white/50 dark:bg-slate-800/50 backdrop-blur-md p-1 rounded-xl shadow-sm border border-slate-200/50 dark:border-slate-700/50 inline-flex">
        <button 
          @click="activeTab = 'ipv4'"
          :class="[
            'px-6 py-2.5 rounded-lg text-sm font-medium transition-all duration-300',
            activeTab === 'ipv4' 
              ? 'bg-indigo-500 text-white shadow-md' 
              : 'text-slate-600 dark:text-slate-400 hover:text-indigo-500 dark:hover:text-indigo-400'
          ]"
        >
          IPv4 计算器
        </button>
        <button 
          @click="activeTab = 'ipv6'"
          :class="[
            'px-6 py-2.5 rounded-lg text-sm font-medium transition-all duration-300',
            activeTab === 'ipv6' 
              ? 'bg-indigo-500 text-white shadow-md' 
              : 'text-slate-600 dark:text-slate-400 hover:text-indigo-500 dark:hover:text-indigo-400'
          ]"
        >
          IPv6 计算器
        </button>
      </div>
    </div>

    <main class="w-full max-w-4xl relative z-10">
      <transition name="fade" mode="out-in">
        <IPv4Calc v-if="activeTab === 'ipv4'" />
        <IPv6Calc v-else-if="activeTab === 'ipv6'" />
      </transition>
    </main>
  </div>
</template>

<style scoped>
.glass-panel {
  background: rgba(255, 255, 255, 0.7);
  box-shadow: 0 8px 32px 0 rgba(31, 38, 135, 0.07);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.5);
}

.dark .glass-panel {
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.2);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}
.fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}
.fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
