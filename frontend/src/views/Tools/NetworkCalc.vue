<script setup lang="ts">
import { ref } from 'vue'
import IPv4Calc from '../../components/network/IPv4Calc.vue'
import IPv6Calc from '../../components/network/IPv6Calc.vue'

defineOptions({ name: 'NetworkCalc' })

const activeTab = ref<'ipv4' | 'ipv6'>('ipv4')
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- V4/V6 切换开关 -->
    <div class="w-full relative z-10 mb-6 flex justify-start">
      <div class="bg-bg-tertiary/20 backdrop-blur-sm p-1 rounded-lg border border-border inline-flex">
        <button 
          @click="activeTab = 'ipv4'"
          :class="[
            'px-5 py-1.5 rounded-md text-sm font-medium transition-all duration-300',
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
            'px-5 py-1.5 rounded-md text-sm font-medium transition-all duration-300',
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
        <keep-alive>
          <IPv4Calc v-if="activeTab === 'ipv4'" />
          <IPv6Calc v-else-if="activeTab === 'ipv6'" />
        </keep-alive>
      </transition>
    </main>
  </div>
</template>

