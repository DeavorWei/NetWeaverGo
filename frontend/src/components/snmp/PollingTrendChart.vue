<template>
  <div class="polling-trend-chart">
    <!-- 标题栏 -->
    <div class="chart-header">
      <h3 class="chart-title">{{ title }}</h3>
      <div class="chart-controls">
        <!-- 时间范围选择 -->
        <select v-model="selectedDuration" class="duration-select" @change="onDurationChange">
          <option v-for="opt in durationOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <!-- 刷新按钮 -->
        <button class="refresh-btn" @click="refreshData" :disabled="loading">
          <span class="icon" :class="{ 'is-loading': loading }">🔄</span>
        </button>
      </div>
    </div>

    <!-- 图表区域 -->
    <div class="chart-container" ref="chartContainer">
      <div v-if="loading" class="chart-loading">
        <span class="loading-spinner"></span>
        <span>加载中...</span>
      </div>
      <div v-else-if="error" class="chart-error">
        <span class="error-icon">⚠️</span>
        <span>{{ error }}</span>
      </div>
      <div v-else-if="dataPoints.length === 0" class="chart-empty">
        <span class="empty-icon">📊</span>
        <span>暂无数据</span>
      </div>
      <svg v-else class="trend-svg" :viewBox="`0 0 ${chartWidth} ${chartHeight}`">
        <!-- 网格线 -->
        <g class="grid-lines">
          <line
            v-for="i in gridLines"
            :key="'h' + i"
            :x1="padding.left"
            :y1="i"
            :x2="chartWidth - padding.right"
            :y2="i"
            class="grid-line"
          />
        </g>

        <!-- X轴 -->
        <line
          :x1="padding.left"
          :y1="chartHeight - padding.bottom"
          :x2="chartWidth - padding.right"
          :y2="chartHeight - padding.bottom"
          class="axis-line"
        />

        <!-- Y轴 -->
        <line
          :x1="padding.left"
          :y1="padding.top"
          :x2="padding.left"
          :y2="chartHeight - padding.bottom"
          class="axis-line"
        />

        <!-- X轴标签 -->
        <g class="x-labels">
          <text
            v-for="(label, index) in xLabels"
            :key="'x' + index"
            :x="label.x"
            :y="chartHeight - padding.bottom + 20"
            class="axis-label"
            text-anchor="middle"
          >
            {{ label.text }}
          </text>
        </g>

        <!-- Y轴标签 -->
        <g class="y-labels">
          <text
            v-for="(label, index) in yLabels"
            :key="'y' + index"
            :x="padding.left - 10"
            :y="label.y"
            class="axis-label"
            text-anchor="end"
            dominant-baseline="middle"
          >
            {{ label.text }}
          </text>
        </g>

        <!-- 数据线 -->
        <polyline
          :points="linePoints"
          class="data-line"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        />

        <!-- 数据点 -->
        <g class="data-points">
          <circle
            v-for="(point, index) in plottedPoints"
            :key="index"
            :cx="point.x"
            :cy="point.y"
            :r="point.isNumeric ? 4 : 3"
            :class="['data-point', { 'numeric': point.isNumeric }]"
          />
        </g>

        <!-- 数据区域填充 -->
        <polygon
          v-if="plottedPoints.length > 0"
          :points="areaPoints"
          class="data-area"
        />
      </svg>
    </div>

    <!-- 图例和统计信息 -->
    <div v-if="dataPoints.length > 0" class="chart-footer">
      <div class="chart-stats">
        <span class="stat-item">
          <span class="stat-label">数据点:</span>
          <span class="stat-value">{{ dataPoints.length }}</span>
        </span>
        <span v-if="numericStats" class="stat-item">
          <span class="stat-label">最小值:</span>
          <span class="stat-value">{{ numericStats.min }}</span>
        </span>
        <span v-if="numericStats" class="stat-item">
          <span class="stat-label">最大值:</span>
          <span class="stat-value">{{ numericStats.max }}</span>
        </span>
        <span v-if="numericStats" class="stat-item">
          <span class="stat-label">平均值:</span>
          <span class="stat-value">{{ numericStats.avg }}</span>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { SNMPPollingAPI } from '../../services/snmpApi'
import { TREND_DURATION_OPTIONS } from '../../types/snmp'
import type { TrendDataPoint } from '../../types/snmp'
import { getLogger } from '../../utils/logger'

const logger = getLogger()

// Props
interface Props {
  targetId: number
  oid?: string
  title?: string
  height?: number
}

const props = withDefaults(defineProps<Props>(), {
  title: '轮询趋势图',
  height: 300,
})

// Emits
const emit = defineEmits<{
  (e: 'error', error: string): void
  (e: 'loaded'): void
}>()

// 状态
const loading = ref(false)
const error = ref<string | null>(null)
const dataPoints = ref<TrendDataPoint[]>([])
const selectedDuration = ref('24h')

// 图表尺寸
const chartWidth = 600
const chartHeight = computed(() => props.height)
const padding = { top: 20, right: 20, bottom: 40, left: 60 }

// 时间范围选项
const durationOptions = TREND_DURATION_OPTIONS

// 计算网格线位置
const gridLines = computed(() => {
  const lines: number[] = []
  const step = (chartHeight.value - padding.top - padding.bottom) / 5
  for (let i = 0; i <= 5; i++) {
    lines.push(padding.top + step * i)
  }
  return lines
})

// 计算数值范围
const numericRange = computed(() => {
  const numericPoints = dataPoints.value.filter(p => p.numeric)
  if (numericPoints.length === 0) return null

  const values = numericPoints.map(p => parseFloat(p.value))
  const min = Math.min(...values)
  const max = Math.max(...values)
  const padding_range = (max - min) * 0.1 || 1

  return {
    min: min - padding_range,
    max: max + padding_range,
  }
})

// 计算X轴标签
const xLabels = computed(() => {
  if (dataPoints.value.length === 0) return []

  const labels: { x: number; text: string }[] = []
  const plotWidth = chartWidth - padding.left - padding.right
  const pointCount = dataPoints.value.length

  // 显示 5 个时间标签
  const labelCount = Math.min(5, pointCount)
  const step = Math.floor(pointCount / labelCount)

  for (let i = 0; i < labelCount; i++) {
    const index = i * step
    if (index < dataPoints.value.length) {
      const point = dataPoints.value[index]
      if (point) {
        const x = padding.left + (index / (pointCount - 1)) * plotWidth
        const date = new Date(point.timestamp)
        const text = formatTimeLabel(date)
        labels.push({ x, text })
      }
    }
  }

  return labels
})

// 计算Y轴标签
const yLabels = computed(() => {
  const labels: { y: number; text: string }[] = []
  const range = numericRange.value

  if (!range) {
    // 非数值型数据，显示序号
    for (let i = 0; i <= 5; i++) {
      const y = padding.top + ((chartHeight.value - padding.top - padding.bottom) / 5) * i
      labels.push({ y, text: String(5 - i) })
    }
  } else {
    // 数值型数据
    for (let i = 0; i <= 5; i++) {
      const y = padding.top + ((chartHeight.value - padding.top - padding.bottom) / 5) * i
      const value = range.max - ((range.max - range.min) / 5) * i
      labels.push({ y, text: formatNumber(value) })
    }
  }

  return labels
})

// 计算绘图点坐标
const plottedPoints = computed(() => {
  if (dataPoints.value.length === 0) return []

  const plotWidth = chartWidth - padding.left - padding.right
  const plotHeight = chartHeight.value - padding.top - padding.bottom
  const range = numericRange.value

  return dataPoints.value.map((point, index) => {
    const x = padding.left + (index / (dataPoints.value.length - 1)) * plotWidth

    let y: number
    if (range && point.numeric) {
      const value = parseFloat(point.value)
      const normalizedValue = (value - range.min) / (range.max - range.min)
      y = chartHeight.value - padding.bottom - normalizedValue * plotHeight
    } else {
      // 非数值型数据，按序号排列
      const index_normalized = index / (dataPoints.value.length - 1)
      y = chartHeight.value - padding.bottom - index_normalized * plotHeight
    }

    return { x, y, isNumeric: point.numeric }
  })
})

// 计算折线点
const linePoints = computed(() => {
  return plottedPoints.value.map(p => `${p.x},${p.y}`).join(' ')
})

// 计算区域填充点
const areaPoints = computed(() => {
  if (plottedPoints.value.length === 0) return ''

  const points = plottedPoints.value.map(p => `${p.x},${p.y}`)
  const firstPoint = plottedPoints.value[0]
  const lastPoint = plottedPoints.value[plottedPoints.value.length - 1]
  if (!firstPoint || !lastPoint) return ''
  const firstX = firstPoint.x
  const lastX = lastPoint.x
  const bottomY = chartHeight.value - padding.bottom

  return `${firstX},${bottomY} ${points.join(' ')} ${lastX},${bottomY}`
})

// 计算数值统计
const numericStats = computed(() => {
  const numericPoints = dataPoints.value.filter(p => p.numeric)
  if (numericPoints.length === 0) return null

  const values = numericPoints.map(p => parseFloat(p.value))
  const min = Math.min(...values)
  const max = Math.max(...values)
  const avg = values.reduce((a, b) => a + b, 0) / values.length

  return {
    min: formatNumber(min),
    max: formatNumber(max),
    avg: formatNumber(avg),
  }
})

// 方法
async function refreshData() {
  if (loading.value) return

  loading.value = true
  error.value = null

  try {
    if (props.oid) {
      // 获取单个 OID 的趋势数据
      const trend = await SNMPPollingAPI.getPollingTrend(
        props.targetId,
        props.oid,
        selectedDuration.value
      )
      dataPoints.value = trend.dataPoints
    } else {
      // 获取整体历史数据
      const history = await SNMPPollingAPI.getPollingHistory(
        props.targetId,
        selectedDuration.value
      )
      // 将历史数据转换为趋势数据点
      dataPoints.value = history.dataPoints.map((dp) => ({
        timestamp: dp.timestamp,
        value: Object.values(dp.values).join(', '),
        numeric: Object.values(dp.values).some(v => !isNaN(parseFloat(v as string))),
      }))
    }

    emit('loaded')
    logger.debug(`PollingTrendChart: 数据已刷新 - ${dataPoints.value.length} 个数据点`)
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : '加载数据失败'
    emit('error', error.value)
    logger.error('PollingTrendChart: 加载数据失败', 'PollingTrendChart', err)
  } finally {
    loading.value = false
  }
}

function onDurationChange() {
  refreshData()
}

function formatTimeLabel(date: Date): string {
  const now = new Date()
  const diff = now.getTime() - date.getTime()

  // 根据时间差选择不同的格式
  if (diff < 3600000) {
    // 1小时内，显示分钟
    return `${date.getMinutes()}分`
  } else if (diff < 86400000) {
    // 24小时内，显示时:分
    return `${date.getHours()}:${String(date.getMinutes()).padStart(2, '0')}`
  } else {
    // 超过24小时，显示月-日
    return `${date.getMonth() + 1}-${date.getDate()}`
  }
}

function formatNumber(value: number): string {
  if (Math.abs(value) >= 1000000) {
    return (value / 1000000).toFixed(1) + 'M'
  } else if (Math.abs(value) >= 1000) {
    return (value / 1000).toFixed(1) + 'K'
  } else if (Number.isInteger(value)) {
    return value.toString()
  } else {
    return value.toFixed(2)
  }
}

// 监听属性变化
watch(() => props.targetId, refreshData)
watch(() => props.oid, refreshData)

// 生命周期
onMounted(() => {
  refreshData()
})
</script>

<style scoped>
.polling-trend-chart {
  background: var(--bg-secondary, #1e1e2e);
  border-radius: 8px;
  padding: 16px;
  border: 1px solid var(--border-color, #313244);
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.chart-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #cdd6f4);
}

.chart-controls {
  display: flex;
  gap: 8px;
  align-items: center;
}

.duration-select {
  background: var(--bg-tertiary, #313244);
  border: 1px solid var(--border-color, #45475a);
  border-radius: 4px;
  padding: 4px 8px;
  color: var(--text-primary, #cdd6f4);
  font-size: 12px;
  cursor: pointer;
}

.duration-select:hover {
  border-color: var(--accent-color, #89b4fa);
}

.refresh-btn {
  background: transparent;
  border: 1px solid var(--border-color, #45475a);
  border-radius: 4px;
  padding: 4px 8px;
  cursor: pointer;
  color: var(--text-secondary, #a6adc8);
  transition: all 0.2s;
}

.refresh-btn:hover:not(:disabled) {
  border-color: var(--accent-color, #89b4fa);
  color: var(--accent-color, #89b4fa);
}

.refresh-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.icon {
  display: inline-block;
}

.icon.is-loading {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.chart-container {
  position: relative;
  width: 100%;
  min-height: 200px;
}

.chart-loading,
.chart-error,
.chart-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 200px;
  color: var(--text-secondary, #a6adc8);
  gap: 8px;
}

.loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border-color, #45475a);
  border-top-color: var(--accent-color, #89b4fa);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.error-icon,
.empty-icon {
  font-size: 32px;
}

.trend-svg {
  width: 100%;
  height: auto;
}

.grid-line {
  stroke: var(--border-color, #313244);
  stroke-dasharray: 2, 2;
}

.axis-line {
  stroke: var(--border-color, #45475a);
  stroke-width: 1;
}

.axis-label {
  fill: var(--text-secondary, #a6adc8);
  font-size: 10px;
}

.data-line {
  stroke: var(--accent-color, #89b4fa);
  stroke-linecap: round;
  stroke-linejoin: round;
}

.data-point {
  fill: var(--accent-color, #89b4fa);
  stroke: var(--bg-secondary, #1e1e2e);
  stroke-width: 1;
}

.data-point.numeric {
  fill: var(--success-color, #a6e3a1);
}

.data-area {
  fill: var(--accent-color, #89b4fa);
  fill-opacity: 0.1;
}

.chart-footer {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border-color, #313244);
}

.chart-stats {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.stat-item {
  display: flex;
  gap: 4px;
  font-size: 12px;
}

.stat-label {
  color: var(--text-secondary, #a6adc8);
}

.stat-value {
  color: var(--text-primary, #cdd6f4);
  font-weight: 500;
}
</style>
