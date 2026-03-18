<script setup lang="ts">
defineProps<{
  outputBlocks: string[];
  isBuilding: boolean;
  ipBindingEnabled: boolean;
  isCopied: boolean;
}>();

const emit = defineEmits<{
  sendCommand: [];
  sendTask: [];
  downloadAll: [];
  downloadSplit: [];
  copyAll: [];
}>();
</script>

<template>
  <div
    class="min-h-[400px] md:h-full md:min-h-[700px] glass-panel border border-border relative flex flex-col overflow-hidden"
  >
    <div class="card-header">
      <h2 class="card-header-title shrink-0">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-success"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"
          />
        </svg>
        生成预览
        <span
          class="ml-2 px-2 py-0.5 bg-accent-bg text-accent text-xs rounded-full font-mono"
          >{{ outputBlocks.length }} blocks</span
        >
        <span
          v-if="isBuilding"
          class="ml-2 text-xs text-text-muted animate-pulse"
          >构建中...</span
        >
      </h2>

      <!-- 功能按钮区 -->
      <div class="flex space-x-2 items-center" v-if="outputBlocks.length > 0">
        <!-- IP绑定模式 -->
        <button
          v-if="ipBindingEnabled"
          @click="emit('sendTask')"
          class="btn btn-sm btn-secondary group relative"
          title="发送到任务执行"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-warning"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <polygon
              points="5 3 19 12 5 21 5 3"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            />
          </svg>
        </button>
        <!-- 发送到命令管理 -->
        <button
          @click="emit('sendCommand')"
          class="btn btn-sm btn-secondary group relative"
          title="发送到命令管理"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-success"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
        </button>

        <button
          @click="emit('downloadSplit')"
          class="btn btn-sm btn-secondary group relative"
          title="分块下载"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-accent"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2"
            />
          </svg>
        </button>
        <button
          @click="emit('downloadAll')"
          class="btn btn-sm btn-secondary group relative"
          title="合并下载"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-info"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
        </button>
        <button
          @click="emit('copyAll')"
          class="btn btn-sm group relative"
          :class="isCopied ? 'btn-success' : 'btn-primary'"
          title="复制全部"
        >
          <svg
            v-if="!isCopied"
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
            />
          </svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto p-5 scrollbar-custom">
      <template v-if="outputBlocks.length > 0">
        <div
          v-for="(block, idx) in outputBlocks"
          :key="idx"
          class="p-4 mb-4 bg-bg-tertiary/60 backdrop-blur-sm border border-border rounded-xl font-mono text-sm whitespace-pre-wrap text-text-primary shadow-sm transition-all hover:shadow-md"
        >
          {{ block }}
        </div>
      </template>
      <div
        v-else
        class="h-full flex flex-col items-center justify-center text-text-muted space-y-3"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-12 w-12 text-text-muted/50"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"
          />
        </svg>
        <span class="text-sm font-medium">配置等待生成...</span>
      </div>
    </div>
  </div>
</template>
