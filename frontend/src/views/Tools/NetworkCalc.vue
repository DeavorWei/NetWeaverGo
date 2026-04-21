<script setup lang="ts">
import { ref } from 'vue'
import IPv4Calc from '../../components/network/IPv4Calc.vue'
import IPv6Calc from '../../components/network/IPv6Calc.vue'

const activeTab = ref<'ipv4' | 'ipv6'>('ipv4')
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- V4/V6 切换开关 -->
    <div class="w-full relative z-10 mb-6 flex justify-center">
      <div class="bg-bg-tertiary/20 glass p-1 rounded-xl border border-border inline-flex">
        <button 
          @click="activeTab = 'ipv4'"
          :class="[
            'px-6 py-2.5 rounded-lg text-sm font-medium transition-all duration-300',
            activeTab === 'ipv4' 
              ? 'bg-accent text-white shadow-glow' 
              : 'text-text-muted hover:text-accent hover:bg-bg-hover/50'
          ]"
        >
          IPv4 计算器
        </button>
        <button 
          @click="activeTab = 'ipv6'"
          :class="[
            'px-6 py-2.5 rounded-lg text-sm font-medium transition-all duration-300',
            activeTab === 'ipv6' 
              ? 'bg-accent text-white shadow-glow' 
              : 'text-text-muted hover:text-accent hover:bg-bg-hover/50'
          ]"
        >
          IPv6 计算器
        </button>
      </div>
    </div>

    <main class="flex-1 w-full relative z-10 overflow-auto">
      <transition name="fade" mode="out-in">
        <IPv4Calc v-if="activeTab === 'ipv4'" />
        <IPv6Calc v-else-if="activeTab === 'ipv6'" />
      </transition>
    </main>
  </div>
</template>

<style scoped>
/* fade 过渡动画已移至全局 _animations.css */
</style>
