<template>
  <div class="col-span-7 bg-bg-secondary rounded-lg border border-border p-4 flex flex-col min-h-0">
    <div class="flex items-center justify-between pb-3 border-b border-border">
      <div class="flex items-center gap-2">
        <span class="font-medium text-sm text-text-primary">回显测试面板 (Echo Test Pane)</span>
        <span v-if="selectedTpl" class="text-xs text-text-muted">
          当前载入: <span class="font-mono text-blue-400">{{ selectedTpl.commandKey }}</span>
          ({{ selectedTpl.engine }})
        </span>
      </div>
      <el-button type="success" size="small" :loading="testing" @click="$emit('run', raw)">
        ▶ 运行解析测试
      </el-button>
    </div>

    <div class="grid grid-rows-2 gap-3 flex-1 min-h-0 mt-3">
      <div class="flex flex-col min-h-0">
        <div class="text-xs text-text-muted mb-1 flex justify-between items-center">
          <span>原始 CLI 回显文本 (Raw Echo)</span>
          <span class="text-text-muted/60 text-[11px]">粘贴设备真实回显进行验证</span>
        </div>
        <textarea
          v-model="raw"
          class="flex-1 w-full bg-bg-primary text-text-primary p-2.5 rounded border border-border font-mono text-xs focus:outline-none focus:border-blue-500 resize-none"
          placeholder="在此粘贴设备的命令原始回显文本，例如 display device、display interface 等..."
        ></textarea>
      </div>

      <div class="flex flex-col min-h-0 border border-border rounded bg-bg-primary p-3">
        <div class="flex items-center justify-between pb-2 border-b border-border/50 text-xs">
          <div class="flex items-center gap-3">
            <span class="font-medium text-text-primary">
              解析结果
              <span v-if="result && result.success" class="text-blue-400 font-mono">
                ({{ result.count }} 行)
              </span>
            </span>
            <el-radio-group :model-value="viewMode" @update:model-value="$emit('update:viewMode', $event)" size="small">
              <el-radio-button label="table">📊 表格</el-radio-button>
              <el-radio-button label="tree">🌲 树状</el-radio-button>
              <el-radio-button label="json">📄 JSON</el-radio-button>
              <el-radio-button label="highlight">🎯 高亮</el-radio-button>
            </el-radio-group>
          </div>
          <span v-if="result?.error" class="text-rose-400 truncate max-w-sm text-xs" :title="result.error">
            ⚠ {{ result.error }}
          </span>
        </div>

        <div class="flex-1 overflow-auto mt-2">
          <!-- 表格视图 -->
          <table v-if="viewMode === 'table' && result && result.results.length > 0" class="w-full text-xs text-left border-collapse">
            <thead>
              <tr class="bg-bg-secondary/60 text-text-muted border-b border-border font-mono">
                <th class="p-1.5 w-10 text-center">#</th>
                <th v-for="col in columns" :key="col" class="p-1.5 font-medium">{{ col }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, idx) in result.results" :key="idx" class="border-b border-border/30 hover:bg-bg-hover/50 font-mono">
                <td class="p-1.5 text-center text-text-muted/60">{{ idx + 1 }}</td>
                <td v-for="col in columns" :key="col" class="p-1.5 text-text-primary">
                  {{ row[col] !== undefined && row[col] !== '' ? row[col] : '-' }}
                </td>
              </tr>
            </tbody>
          </table>

          <!-- 树状折叠视图 -->
          <div v-else-if="viewMode === 'tree' && result && result.results.length > 0" class="space-y-2 p-1">
            <el-collapse>
              <el-collapse-item v-for="(row, idx) in result.results" :key="idx" :name="idx">
                <template #title>
                  <div class="flex items-center gap-2 font-mono text-xs">
                    <span class="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 font-bold">#{{ idx + 1 }}</span>
                    <span class="text-text-primary font-medium">{{ rowPrimarySummary(row) }}</span>
                    <span class="text-text-muted text-[11px]">({{ Object.keys(row).length }} 字段)</span>
                  </div>
                </template>
                <div class="grid grid-cols-2 gap-2 p-2 bg-bg-secondary/40 rounded text-xs font-mono">
                  <div v-for="(val, key) in row" :key="key" class="flex items-center justify-between border-b border-border/20 py-1">
                    <span class="text-text-muted">{{ key }}:</span>
                    <span class="text-emerald-400 font-semibold">{{ val || '-' }}</span>
                  </div>
                </div>
              </el-collapse-item>
            </el-collapse>
          </div>

          <!-- JSON 格式化视图 -->
          <pre
            v-else-if="viewMode === 'json' && result && result.results.length > 0"
            class="h-full bg-bg-secondary/60 p-3 rounded text-xs font-mono overflow-auto text-emerald-400 select-text"
          >{{ JSON.stringify(result.results, null, 2) }}</pre>

          <!-- 匹配高亮视图 -->
          <div v-else-if="viewMode === 'highlight'" class="text-xs font-mono">
            <div v-if="!result || !result.matches || result.matches.length === 0" class="text-text-muted py-8 text-center">
              当前模板未产出命中区间（regex/aggregate 引擎或无匹配）。
            </div>
            <pre v-else class="whitespace-pre-wrap break-all leading-relaxed"><span
                v-for="(seg, i) in highlightSegments"
                :key="i"
                :class="seg.hit ? 'bg-emerald-500/30 text-emerald-300 rounded px-0.5' : 'text-text-secondary'"
              >{{ seg.text }}</span></pre>
          </div>

          <!-- 空状态 -->
          <div v-else-if="!testing && (!result || result.results.length === 0)" class="h-full flex items-center justify-center text-xs text-text-muted">
            点击上方"运行解析测试"查看解析结果。
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { UserParseTemplateVO, TestParseTemplateResult } from '@/services/parseTemplateApi'

interface ParseMatch { rule: string; start: number; end: number; text: string }

// 绑定模型尚未重新生成，matches 为本次新增的可选字段，用交叉类型向后兼容声明
type ResultWithMatches = TestParseTemplateResult & { matches?: ParseMatch[] }

const props = defineProps<{
  selectedTpl: UserParseTemplateVO | null
  result: ResultWithMatches | null
  testing: boolean
  columns: string[]
  viewMode: 'table' | 'tree' | 'json' | 'highlight'
}>()

defineEmits<{
  (e: 'run', raw: string): void
  (e: 'update:viewMode', mode: 'table' | 'tree' | 'json' | 'highlight'): void
}>()

const raw = defineModel<string>('raw', { default: '' })

const highlightSegments = computed(() => {
  const text = raw.value
  const matches = props.result?.matches ?? []
  if (!text || matches.length === 0) return [{ text, hit: false }]
  const segs: { text: string; hit: boolean }[] = []
  let cursor = 0
  for (const m of matches) {
    if (m.start < cursor || m.start > text.length) continue
    if (m.start > cursor) segs.push({ text: text.slice(cursor, m.start), hit: false })
    segs.push({ text: text.slice(m.start, Math.min(m.end, text.length)), hit: true })
    cursor = Math.max(cursor, m.end)
  }
  if (cursor < text.length) segs.push({ text: text.slice(cursor), hit: false })
  return segs
})

function rowPrimarySummary(row: Record<string, any>): string {
  const keys = Object.keys(row)
  const first = keys[0]
  if (!first) return '(空)'
  const second = keys[1]
  return row[first] || (second ? row[second] : '') || JSON.stringify(row).slice(0, 40)
}
</script>
