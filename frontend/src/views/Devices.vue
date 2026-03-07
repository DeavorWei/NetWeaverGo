<template>
  <div class="animate-slide-in space-y-6">
    <!-- 页面标题 + 统计 -->
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm text-text-muted">共 {{ data.length }} 台已注册设备</p>
      </div>
    </div>

    <!-- 数据表格 -->
    <div class="bg-bg-card border border-border rounded-xl shadow-card overflow-hidden">
      <div class="overflow-auto scrollbar-custom max-h-[calc(100vh-220px)]">
        <table class="w-full text-sm">
          <thead class="sticky top-0 z-10">
            <tr class="bg-bg-panel border-b border-border">
              <th class="px-5 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider">#</th>
              <th class="px-5 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider">IP 地址</th>
              <th class="px-5 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider">端口</th>
              <th class="px-5 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider">用户名</th>
              <th class="px-5 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider">密码</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-if="data.length === 0">
              <td colspan="5" class="px-5 py-12 text-center text-text-muted">
                <div class="flex flex-col items-center gap-3">
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-10 h-10 text-text-muted/40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/>
                  </svg>
                  <span class="text-sm">暂无设备数据</span>
                </div>
              </td>
            </tr>
            <tr
              v-for="(row, idx) in pagedData"
              :key="idx"
              class="hover:bg-bg-hover transition-colors duration-150 group"
            >
              <td class="px-5 py-3.5 text-text-muted font-mono text-xs">{{ (page - 1) * pageSize + idx + 1 }}</td>
              <td class="px-5 py-3.5">
                <span class="font-mono text-accent font-medium">{{ row.IP }}</span>
              </td>
              <td class="px-5 py-3.5 text-text-secondary font-mono">{{ row.Port }}</td>
              <td class="px-5 py-3.5 text-text-secondary">{{ row.Username }}</td>
              <td class="px-5 py-3.5">
                <span class="font-mono text-text-muted tracking-widest text-xs">{{ '••••••••' }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页 -->
      <div v-if="totalPages > 1" class="flex items-center justify-between px-5 py-3.5 border-t border-border bg-bg-panel">
        <span class="text-xs text-text-muted">第 {{ page }} / {{ totalPages }} 页，共 {{ data.length }} 条</span>
        <div class="flex items-center gap-2">
          <button
            @click="page = Math.max(1, page - 1)"
            :disabled="page === 1"
            class="px-3 py-1.5 text-xs rounded-lg border border-border text-text-secondary hover:border-accent/50 hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed transition-all duration-200"
          >上一页</button>
          <button
            @click="page = Math.min(totalPages, page + 1)"
            :disabled="page === totalPages"
            class="px-3 py-1.5 text-xs rounded-lg border border-border text-text-secondary hover:border-accent/50 hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed transition-all duration-200"
          >下一页</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
// @ts-ignore
import { EnsureConfig } from '../bindings/github.com/NetWeaverGo/core/internal/ui/appservice.js'

const data     = ref<any[]>([])
const page     = ref(1)
const pageSize = 12

const totalPages = computed(() => Math.max(1, Math.ceil(data.value.length / pageSize)))
const pagedData  = computed(() => {
  const start = (page.value - 1) * pageSize
  return data.value.slice(start, start + pageSize)
})

onMounted(async () => {
  try {
    const [assets] = await EnsureConfig()
    if (assets) data.value = assets
  } catch (e) {
    console.error('Failed to load devices', e)
  }
})
</script>
