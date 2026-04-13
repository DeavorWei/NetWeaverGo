import { ref, computed } from 'vue'
import { TaskExecutionAPI } from '@/services/api'
import type {
  ReplayOptions,
  ReplayResult,
  ReplayStatistics
} from '@/types/taskexec'

// 从 bindings 导入类型
import type { TopologyReplayRecord, ReplayableRunInfo } from '@/bindings/github.com/NetWeaverGo/core/internal/taskexec/models'

export function useTopologyReplay() {
  // 状态
  const replaying = ref(false)
  const replayProgress = ref(0)
  const replayPhase = ref('')
  const replayMessage = ref('')
  const replayResult = ref<ReplayResult | null>(null)
  const replayError = ref<string | null>(null)
  
  // 可重放的运行列表
  const replayableRuns = ref<ReplayableRunInfo[]>([])
  
  // 重放历史
  const replayHistory = ref<TopologyReplayRecord[]>([])

  // 计算属性
  const hasReplayableRuns = computed(() => replayableRuns.value.length > 0)
  const isReplaySuccess = computed(() => replayResult.value?.status === 'completed')
  const isReplayFailed = computed(() => replayResult.value?.status === 'failed')
  const isReplayCancelled = computed(() => replayResult.value?.status === 'cancelled')

  /**
   * 从历史Raw文件重放拓扑构建
   */
  const replayTopology = async (
    originalRunId: string,
    options: ReplayOptions = { clearExisting: false, parserVersion: '', deviceIps: [], skipBuild: false }
  ): Promise<ReplayResult> => {
    replaying.value = true
    replayProgress.value = 0
    replayPhase.value = 'scan'
    replayMessage.value = '正在扫描Raw文件...'
    replayError.value = null
    replayResult.value = null

    try {
      // 使用正确的 API 调用模式
      const result = await TaskExecutionAPI.replayTopologyFromRaw(originalRunId, options)

      // 更新状态
      if (result) {
        replayResult.value = result
        replayProgress.value = 100
        replayPhase.value = 'completed'
        replayMessage.value = result.status === 'completed'
          ? `重放完成：解析 ${result.statistics.parsedDevices} 设备，生成 ${result.statistics.retainedEdges} 条拓扑边`
          : `重放${result.status}: ${result.errors?.join('; ') || '未知错误'}`
        return result
      } else {
        throw new Error('重放返回空结果')
      }
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : String(err)
      replayError.value = errorMsg
      replayPhase.value = 'failed'
      replayMessage.value = `重放失败: ${errorMsg}`
      
      const failedResult: ReplayResult = {
        replayRunId: '',
        status: 'failed',
        statistics: {} as ReplayStatistics,
        errors: [errorMsg],
        startedAt: new Date().toISOString(),
        finishedAt: new Date().toISOString()
      }
      replayResult.value = failedResult
      return failedResult
    } finally {
      replaying.value = false
    }
  }

  /**
   * 获取可重放的运行列表
   */
  const loadReplayableRuns = async (limit: number = 50): Promise<void> => {
    try {
      const runs = await TaskExecutionAPI.listReplayableRuns(limit)
      replayableRuns.value = (runs || []).filter((r: ReplayableRunInfo) => r.hasRawFiles)
    } catch (err) {
      console.error('获取可重放运行列表失败:', err)
      replayableRuns.value = []
    }
  }

  /**
   * 获取重放历史
   */
  const loadReplayHistory = async (originalRunId: string): Promise<void> => {
    try {
      const history = await TaskExecutionAPI.getReplayHistory(originalRunId)
      replayHistory.value = history || []
    } catch (err) {
      console.error('获取重放历史失败:', err)
      replayHistory.value = []
    }
  }

  /**
   * 重置状态
   */
  const resetState = () => {
    replaying.value = false
    replayProgress.value = 0
    replayPhase.value = ''
    replayMessage.value = ''
    replayResult.value = null
    replayError.value = null
  }

  /**
   * 格式化统计信息
   */
  const formatStatistics = (stats: ReplayStatistics): string => {
    const lines: string[] = []
    
    if (stats.totalRawFiles) {
      lines.push(`扫描 ${stats.totalRawFiles} 个Raw文件`)
    }
    if (stats.parsedDevices) {
      lines.push(`解析 ${stats.parsedDevices} 台设备`)
    }
    if (stats.lldpCount) {
      lines.push(`LLDP邻居: ${stats.lldpCount}`)
    }
    if (stats.fdbCount) {
      lines.push(`FDB条目: ${stats.fdbCount}`)
    }
    if (stats.arpCount) {
      lines.push(`ARP条目: ${stats.arpCount}`)
    }
    if (stats.retainedEdges) {
      lines.push(`生成拓扑边: ${stats.retainedEdges}`)
    }
    if (stats.conflictEdges) {
      lines.push(`冲突边: ${stats.conflictEdges}`)
    }
    if (stats.totalDurationMs) {
      lines.push(`耗时: ${stats.totalDurationMs}ms`)
    }
    
    return lines.join('\n')
  }

  return {
    // 状态
    replaying,
    replayProgress,
    replayPhase,
    replayMessage,
    replayResult,
    replayError,
    replayableRuns,
    replayHistory,
    
    // 计算属性
    hasReplayableRuns,
    isReplaySuccess,
    isReplayFailed,
    isReplayCancelled,
    
    // 方法
    replayTopology,
    loadReplayableRuns,
    loadReplayHistory,
    resetState,
    formatStatistics
  }
}
