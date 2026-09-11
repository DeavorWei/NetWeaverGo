<template>
  <div class="risk-command-panel">
    <div class="mb-4 p-4 rounded-xl bg-bg-card border border-border shadow-card">
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-danger" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <h3 class="text-sm font-semibold text-text-primary">高危风险命令拦截体系</h3>
        </div>
        <div class="flex items-center gap-2">
          <el-button size="small" type="primary" plain @click="fetchRules" :loading="loading">
            刷新规则
          </el-button>
          <el-button size="small" type="primary" @click="savePolicySettings" :loading="savingPolicy">
            保存安全门禁配置
          </el-button>
        </div>
      </div>
      <p class="text-xs text-text-muted">
        命令下发前会由执行引擎（StreamEngine）进行实时安全校验，防止格式化、清空启动配置、核心进程意外删除等灾难性故障。
      </p>

      <!-- 灰度策略与安全开关配置区 -->
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mt-4 p-3 rounded-lg bg-bg-panel/40 border border-border/50">
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">拦截灰度模式</label>
          <el-select v-model="policySettings.riskCommandMode" size="small" class="w-full">
            <el-option label="灰度告警 (warn - 默认)" value="warn" />
            <el-option label="严格拦截 (enforce)" value="enforce" />
            <el-option label="关闭校验 (off)" value="off" />
          </el-select>
        </div>
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">交互确认策略 [Y/N]</label>
          <el-select v-model="policySettings.confirmPolicy" size="small" class="w-full">
            <el-option label="人工挂起审批 (ask_user - 默认)" value="ask_user" />
            <el-option label="自动确认 (auto_yes)" value="auto_yes" />
            <el-option label="自动取消 (auto_no)" value="auto_no" />
          </el-select>
        </div>
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">单设备命令缓存</label>
          <div class="flex items-center h-8">
            <el-switch v-model="policySettings.commandCacheEnabled" inline-prompt active-text="启用" inactive-text="禁用" />
            <span class="text-xs text-text-muted ml-2">命中复用回显</span>
          </div>
        </div>
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">单命令内存防爆上限</label>
          <div class="flex items-center gap-1">
            <el-input-number v-model="policySettings.rawBufferLimitMB" :min="1" :max="64" size="small" class="w-full" />
            <span class="text-xs text-text-muted">MB</span>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-3 mt-3">
        <div class="p-2.5 rounded-lg bg-danger/10 border border-danger/20 text-xs">
          <span class="font-bold text-danger">阻断 (Block)：</span>
          <span class="text-text-secondary">拒绝物理下发并降为单命令失败，不终止整机 Run，遵循 ContinueOnCmdError。</span>
        </div>
        <div class="p-2.5 rounded-lg bg-warning/10 border border-warning/20 text-xs">
          <span class="font-bold text-warning">确认 (Confirm)：</span>
          <span class="text-text-secondary">挂起任务触发工程师人工审批，如重启设备、关闭接口、删除核心协议。</span>
        </div>
        <div class="p-2.5 rounded-lg bg-info/10 border border-info/20 text-xs">
          <span class="font-bold text-info">告警 (Warn)：</span>
          <span class="text-text-secondary">输出高危审计警告日志后放行，如全局调试 (debugging) 等。</span>
        </div>
      </div>
    </div>

    <!-- 规则表格 -->
    <div class="bg-bg-card border border-border rounded-xl p-4 shadow-card">
      <el-table :data="rules" v-loading="loading" stripe style="width: 100%" class="custom-table">
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="vendor" label="适用厂商" width="110">
          <template #default="{ row }">
            <el-tag :type="row.vendor === '*' ? 'info' : 'primary'" size="small">
              {{ row.vendor === '*' ? '全局通用 (*)' : row.vendor.toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="pattern" label="匹配正则 (Pattern)" min-width="220">
          <template #default="{ row }">
            <code class="px-1.5 py-0.5 rounded bg-bg-panel text-xs text-accent font-mono">{{ row.pattern }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="action" label="拦截动作" width="100" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.action === 'block' ? 'danger' : row.action === 'confirm' ? 'warning' : 'info'"
              size="small"
              effect="dark"
            >
              {{ row.action === 'block' ? '阻断' : row.action === 'confirm' ? '人工审批' : '告警' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="拦截说明 / 风险理由" min-width="260" show-overflow-tooltip />
        <el-table-column prop="builtin" label="属性" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.builtin ? 'success' : ''" size="small">
              {{ row.builtin ? '内置种子' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Call } from '@wailsio/runtime'
import { ElMessage } from 'element-plus'

interface RiskCommand {
  id: number
  vendor: string
  category: string
  pattern: string
  action: 'block' | 'confirm' | 'warn'
  reason: string
  enabled: boolean
  builtin: boolean
}

const rules = ref<RiskCommand[]>([])
const loading = ref(false)
const savingPolicy = ref(false)

const policySettings = ref({
  riskCommandMode: 'warn',
  confirmPolicy: 'ask_user',
  commandCacheEnabled: false,
  rawBufferLimitMB: 8
})

const fetchPolicySettings = async () => {
  try {
    const raw = await Call.ByName('ui.SettingsService.LoadSettings') as any
    if (raw) {
      if (raw.riskCommandMode) policySettings.value.riskCommandMode = raw.riskCommandMode
      if (raw.confirmPolicy) policySettings.value.confirmPolicy = raw.confirmPolicy
      if (typeof raw.commandCacheEnabled === 'boolean') policySettings.value.commandCacheEnabled = raw.commandCacheEnabled
      if (typeof raw.rawBufferLimitMB === 'number' && raw.rawBufferLimitMB > 0) policySettings.value.rawBufferLimitMB = raw.rawBufferLimitMB
    }
  } catch (err: any) {
    // 静默兜底
  }
}

const savePolicySettings = async () => {
  savingPolicy.value = true
  try {
    const raw = (await Call.ByName('ui.SettingsService.LoadSettings')) as any || {}
    raw.riskCommandMode = policySettings.value.riskCommandMode
    raw.confirmPolicy = policySettings.value.confirmPolicy
    raw.commandCacheEnabled = policySettings.value.commandCacheEnabled
    raw.rawBufferLimitMB = policySettings.value.rawBufferLimitMB
    await Call.ByName('ui.SettingsService.SaveSettings', raw)
    ElMessage.success('安全门禁与灰度策略配置已保存并即时生效')
  } catch (err: any) {
    ElMessage.error('保存策略配置失败: ' + (err?.message || err))
  } finally {
    savingPolicy.value = false
  }
}

const fetchRules = async () => {
  loading.value = true
  try {
    const res = await Call.ByName('ui.SettingsService.GetRiskCommands') as RiskCommand[]
    if (Array.isArray(res)) {
      rules.value = res
    }
  } catch (err: any) {
    ElMessage.error('获取风险命令规则列表失败: ' + (err?.message || err))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchRules()
  fetchPolicySettings()
})
</script>

<style scoped>
.risk-command-panel {
  width: 100%;
}
</style>
