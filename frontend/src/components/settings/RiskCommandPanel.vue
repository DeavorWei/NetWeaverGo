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
          <el-button size="small" :icon="Refresh" @click="fetchRules" :loading="loading">
            刷新规则
          </el-button>
          <el-button size="small" type="primary" plain @click="savePolicySettings" :loading="savingPolicy">
            保存安全门禁配置
          </el-button>
        </div>
      </div>
      <p class="text-xs text-text-muted">
        命令下发前会由执行引擎（StreamEngine）进行实时安全校验，防止格式化、清空启动配置、核心进程意外删除等灾难性故障。
      </p>

      <!-- 灰度策略与安全开关配置区 -->
      <div class="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-4 mt-4 p-3 rounded-lg bg-bg-panel/40 border border-border/50">
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
            <el-option label="关闭自动应答 (off)" value="off" />
          </el-select>
        </div>
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">解析引擎灰度模式</label>
          <el-select v-model="policySettings.parserEngineMode" size="small" class="w-full">
            <el-option label="自适应 (auto - 默认)" value="auto" />
            <el-option label="强制 tree 引擎 (tree_only)" value="tree_only" />
            <el-option label="应急回退 legacy (legacy_only)" value="legacy_only" />
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
      <div class="flex items-center justify-between mb-3">
        <h4 class="text-sm font-semibold text-text-primary">风险命令规则清单（{{ rules.length }} 条）</h4>
        <div class="flex items-center gap-2">
          <el-button size="small" type="primary" :icon="Plus" @click="openCreateDialog">新建规则</el-button>
          <el-button size="small" :icon="RefreshLeft" @click="resetBuiltinRules" :loading="resetting">
            重置内置
          </el-button>
        </div>
      </div>

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
        <el-table-column prop="reason" label="拦截说明 / 风险理由" min-width="240" show-overflow-tooltip />
        <el-table-column prop="enabled" label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" size="small" @change="(v: boolean) => toggleEnabled(row, v)" />
          </template>
        </el-table-column>
        <el-table-column prop="builtin" label="属性" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.builtin ? 'success' : ''" size="small">
              {{ row.builtin ? '内置种子' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" align="center" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openEditDialog(row)">编辑</el-button>
            <el-button link type="danger" size="small" :disabled="row.builtin" @click="removeRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 规则编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="editingRule.id ? '编辑风险命令规则' : '新建风险命令规则'" width="560px" destroy-on-close>
      <el-form :model="editingRule" label-width="110px">
        <el-form-item label="适用厂商">
          <el-select
            v-model="editingRule.vendor"
            class="w-full"
            filterable
            allow-create
            default-first-option
            placeholder="* 表示全局通用"
          >
            <el-option label="* (全局通用)" value="*" />
            <el-option label="HUAWEI" value="huawei" />
            <el-option label="H3C" value="h3c" />
            <el-option label="CISCO" value="cisco" />
          </el-select>
        </el-form-item>
        <el-form-item label="产品分类">
          <el-input v-model="editingRule.category" placeholder="如 s / ce / ar / fw / wlan / *（可留空）" />
        </el-form-item>
        <el-form-item label="匹配正则">
          <el-input
            v-model="editingRule.pattern"
            type="textarea"
            :rows="2"
            placeholder="如 (?i)^\s*undo\s+ipsec\s+policy\b"
          />
        </el-form-item>
        <el-form-item label="拦截动作">
          <el-select v-model="editingRule.action" class="w-full">
            <el-option label="阻断 (block)" value="block" />
            <el-option label="人工审批 (confirm)" value="confirm" />
            <el-option label="告警 (warn)" value="warn" />
          </el-select>
        </el-form-item>
        <el-form-item label="风险理由">
          <el-input v-model="editingRule.reason" type="textarea" :rows="2" placeholder="面向工程师的拦截理由与处置建议" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="editingRule.enabled" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingRule" @click="submitRule">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Call } from '@wailsio/runtime'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, RefreshLeft } from '@element-plus/icons-vue'

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
const resetting = ref(false)

const policySettings = ref({
  riskCommandMode: 'warn',
  confirmPolicy: 'ask_user',
  parserEngineMode: 'auto',
  commandCacheEnabled: false,
  rawBufferLimitMB: 8
})

// 规则编辑弹窗状态
const dialogVisible = ref(false)
const savingRule = ref(false)
const editingRule = ref<Partial<RiskCommand>>({})

const fetchPolicySettings = async () => {
  try {
    const raw = await Call.ByName('ui.SettingsService.LoadSettings') as any
    if (raw) {
      if (raw.riskCommandMode) policySettings.value.riskCommandMode = raw.riskCommandMode
      if (raw.confirmPolicy) policySettings.value.confirmPolicy = raw.confirmPolicy
      if (raw.parserEngineMode) policySettings.value.parserEngineMode = raw.parserEngineMode
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
    raw.parserEngineMode = policySettings.value.parserEngineMode
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

const openCreateDialog = () => {
  editingRule.value = {
    id: 0,
    vendor: '*',
    category: '',
    pattern: '',
    action: 'warn',
    reason: '',
    enabled: true,
    builtin: false
  }
  dialogVisible.value = true
}

const openEditDialog = (row: RiskCommand) => {
  editingRule.value = { ...row }
  dialogVisible.value = true
}

const submitRule = async () => {
  const rule = editingRule.value
  if (!rule.pattern || !rule.pattern.trim()) {
    ElMessage.warning('匹配正则为必填项')
    return
  }
  if (!rule.vendor) {
    rule.vendor = '*'
  }
  if (!rule.action) {
    rule.action = 'warn'
  }
  savingRule.value = true
  try {
    await Call.ByName('ui.SettingsService.SaveRiskCommand', rule)
    ElMessage.success('风险命令规则已保存并热重载')
    dialogVisible.value = false
    await fetchRules()
  } catch (err: any) {
    ElMessage.error('保存风险命令规则失败: ' + (err?.message || err))
  } finally {
    savingRule.value = false
  }
}

const toggleEnabled = async (row: RiskCommand, value: boolean) => {
  try {
    await Call.ByName('ui.SettingsService.SaveRiskCommand', { ...row, enabled: value })
    ElMessage.success(value ? '规则已启用' : '规则已停用')
  } catch (err: any) {
    row.enabled = !value
    ElMessage.error('切换规则状态失败: ' + (err?.message || err))
  }
}

const removeRule = async (row: RiskCommand) => {
  try {
    await ElMessageBox.confirm(`确认删除自定义规则「${row.pattern}」？`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  try {
    await Call.ByName('ui.SettingsService.DeleteRiskCommand', row.id)
    ElMessage.success('规则已删除')
    await fetchRules()
  } catch (err: any) {
    ElMessage.error('删除规则失败: ' + (err?.message || err))
  }
}

const resetBuiltinRules = async () => {
  try {
    await ElMessageBox.confirm('将清空并恢复全部内置风险命令种子规则（自定义规则不受影响），确认继续？', '重置内置规则', {
      type: 'warning',
      confirmButtonText: '重置',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  resetting.value = true
  try {
    await Call.ByName('ui.SettingsService.ResetRiskCommandRules')
    ElMessage.success('内置风险命令规则已重置')
    await fetchRules()
  } catch (err: any) {
    ElMessage.error('重置内置规则失败: ' + (err?.message || err))
  } finally {
    resetting.value = false
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
