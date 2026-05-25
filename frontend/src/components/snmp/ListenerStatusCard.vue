<template>
  <div class="listener-status-card" :class="{ 'is-running': isRunning, 'is-stopped': !isRunning }">
    <!-- 状态指示灯和标题 -->
    <div class="card-header">
      <div class="status-indicator">
        <span
          :class="[
            'status-dot',
            isRunning ? 'dot-running' : 'dot-stopped'
          ]"
        ></span>
        <span class="status-text">{{ isRunning ? '运行中' : '已停止' }}</span>
      </div>
      <span class="card-title">Trap 监听器</span>
    </div>

    <!-- 状态信息 -->
    <div class="card-body">
      <!-- 监听地址 -->
      <div class="info-row" v-if="status">
        <span class="info-label">监听地址</span>
        <span class="info-value">{{ status.listenAddr || '-' }}</span>
      </div>

      <!-- 已接收 Trap 总数 -->
      <div class="info-row" v-if="status">
        <span class="info-label">已接收 Trap</span>
        <span class="info-value highlight">{{ status.totalTraps }}</span>
      </div>

      <!-- 已过滤数 -->
      <div class="info-row" v-if="status">
        <span class="info-label">已过滤</span>
        <span class="info-value">{{ status.filteredOut }}</span>
      </div>

      <!-- 最后 Trap 时间 -->
      <div class="info-row" v-if="status && status.lastTrapTime">
        <span class="info-label">最后接收</span>
        <span class="info-value">{{ formatTime(status.lastTrapTime) }}</span>
      </div>

      <!-- 启动时间 -->
      <div class="info-row" v-if="status && isRunning && status.startTime">
        <span class="info-label">启动时间</span>
        <span class="info-value">{{ formatTime(status.startTime) }}</span>
      </div>

      <!-- 处理统计 -->
      <div v-if="status && status.handlerStats" class="handler-stats">
        <div class="stats-title">处理统计</div>
        <div class="stats-grid">
          <div class="stat-item">
            <span class="stat-value">{{ status.handlerStats.totalReceived }}</span>
            <span class="stat-label">接收</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ status.handlerStats.totalStored }}</span>
            <span class="stat-label">存储</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ status.handlerStats.totalFiltered }}</span>
            <span class="stat-label">过滤</span>
          </div>
          <div class="stat-item">
            <span class="stat-value error">{{ status.handlerStats.totalErrors }}</span>
            <span class="stat-label">错误</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="card-footer">
      <el-button
        v-if="!isRunning"
        type="success"
        size="small"
        :loading="operating"
        @click="handleStart"
      >
        启动监听
      </el-button>
      <el-button
        v-else
        type="danger"
        size="small"
        :loading="operating"
        @click="handleStop"
      >
        停止监听
      </el-button>
      <el-button
        size="small"
        :loading="refreshing"
        @click="handleRefresh"
      >
        刷新
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ListenerStatus } from '@/types/snmp'

// Props
interface Props {
  status: ListenerStatus | null
  operating?: boolean
  refreshing?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  operating: false,
  refreshing: false,
})

// Emits
const emit = defineEmits<{
  (e: 'start'): void
  (e: 'stop'): void
  (e: 'refresh'): void
}>()

// 是否运行中
const isRunning = computed(() => props.status?.isRunning ?? false)

// 启动监听
function handleStart() {
  emit('start')
}

// 停止监听
function handleStop() {
  emit('stop')
}

// 刷新状态
function handleRefresh() {
  emit('refresh')
}

// 格式化时间
function formatTime(timeStr: string): string {
  if (!timeStr) return '-'
  try {
    const date = new Date(timeStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}
</script>

<style scoped>
.listener-status-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-radius: 8px;
  border: 1px solid var(--color-border-default);
  background-color: var(--color-bg-secondary);
  transition: border-color 0.3s;
}

.listener-status-card.is-running {
  border-color: var(--color-success-15);
}

.listener-status-card.is-stopped {
  border-color: var(--color-error-15);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.dot-running {
  background-color: var(--color-success);
  box-shadow: var(--shadow-success-glow);
  animation: pulse 2s infinite;
}

.dot-stopped {
  background-color: var(--color-error);
  box-shadow: 0 0 8px rgba(239, 68, 68, 0.3);
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.status-text {
  font-size: 13px;
  font-weight: 500;
}

.is-running .status-text {
  color: var(--color-success);
}

.is-stopped .status-text {
  color: var(--color-error);
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
}

.info-label {
  font-size: 12px;
  color: var(--color-text-muted);
}

.info-value {
  font-size: 12px;
  color: var(--color-text-primary);
  font-family: 'Consolas', monospace;
}

.info-value.highlight {
  color: var(--color-accent-primary);
  font-weight: 600;
}

.handler-stats {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--color-border-default);
}

.stats-title {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  margin-bottom: 8px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.stat-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.stat-value.error {
  color: var(--color-error);
}

.stat-label {
  font-size: 10px;
  color: var(--color-text-muted);
}

.card-footer {
  display: flex;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--color-border-default);
}
</style>