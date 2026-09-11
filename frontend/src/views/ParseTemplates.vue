<template>
  <div class="h-full flex flex-col p-6 space-y-4">
    <!-- 顶部标题与操作栏 -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-xl font-bold text-text-primary flex items-center gap-2">
          <span>解析模板管理 (Parse Templates)</span>
          <span class="text-xs px-2 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20">P0 声明式规则树</span>
        </h1>
        <p class="text-xs text-text-muted mt-1">
          管理与配置 CLI 原始回显的结构化提取模板。支持单正则 (regex)、多行聚合 (aggregate) 与二维规则树 (tree) 引擎。
        </p>
      </div>
      <div class="flex items-center gap-3">
        <el-radio-group v-model="selectedVendor" size="small" @change="loadTemplates">
          <el-radio-button label="huawei">华为 (Huawei)</el-radio-button>
          <el-radio-button label="h3c">新华三 (H3C)</el-radio-button>
          <el-radio-button label="cisco">思科 (Cisco)</el-radio-button>
        </el-radio-group>
        <el-button type="primary" size="small" @click="openCreateDialog">
          + 新增解析模板
        </el-button>
      </div>
    </div>

    <!-- 主工作区：左侧模板列表，右侧在线实时测试工作台 -->
    <div class="flex-1 grid grid-cols-12 gap-4 min-h-0">
      <!-- 模板列表 -->
      <div class="col-span-5 bg-bg-secondary rounded-lg border border-border p-4 flex flex-col min-h-0">
        <div class="flex items-center justify-between pb-3 border-b border-border">
          <span class="font-medium text-sm text-text-primary">模板列表 ({{ templates.length }})</span>
          <el-button size="small" link @click="loadTemplates">刷新</el-button>
        </div>
        <div class="flex-1 overflow-y-auto mt-2 space-y-2 pr-1">
          <div
            v-for="tpl in templates"
            :key="tpl.id"
            @click="selectTemplate(tpl)"
            :class="[
              'p-3 rounded border cursor-pointer transition-colors',
              selectedTpl?.id === tpl.id
                ? 'bg-blue-500/10 border-blue-500/50'
                : 'bg-bg-primary/50 border-border hover:border-text-muted/40'
            ]"
          >
            <div class="flex items-center justify-between">
              <span class="font-medium text-sm text-text-primary font-mono">{{ tpl.commandKey }}</span>
              <div class="flex items-center gap-1.5">
                <span class="text-xs px-1.5 py-0.5 rounded font-mono" :class="getEngineBadgeClass(tpl.engine)">
                  {{ tpl.engine }}
                </span>
                <span v-if="tpl.enabled" class="text-xs text-emerald-400">● 启用</span>
                <span v-else class="text-xs text-text-muted">○ 停用</span>
              </div>
            </div>
            <div class="text-xs text-text-muted mt-1 truncate">
              {{ tpl.description || '无描述' }}
            </div>
            <div v-if="getAppliesToText(tpl.appliesTo)" class="text-[11px] text-cyan-400 font-mono mt-1 truncate">
              <span class="px-1.5 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20">
                🎯 {{ getAppliesToText(tpl.appliesTo) }}
              </span>
            </div>
            <div class="flex justify-between items-center mt-2 pt-2 border-t border-border/50 text-xs">
              <span class="text-text-muted/60 font-mono text-[11px]">rev.{{ tpl.revision }}</span>
              <div class="flex gap-2">
                <el-button size="small" link type="primary" @click.stop="openEditDialog(tpl)">编辑</el-button>
                <el-button size="small" link type="danger" @click.stop="handleDelete(tpl)">删除</el-button>
              </div>
            </div>
          </div>
          <div v-if="templates.length === 0" class="text-center py-12 text-text-muted text-xs">
            当前厂商暂无自定义模板，采集时将采用系统内置模板。
          </div>
        </div>
      </div>

      <!-- 右侧在线测试台 -->
      <div class="col-span-7 bg-bg-secondary rounded-lg border border-border p-4 flex flex-col min-h-0">
        <div class="flex items-center justify-between pb-3 border-b border-border">
          <div class="flex items-center gap-2">
            <span class="font-medium text-sm text-text-primary">回显测试面板 (Echo Test Pane)</span>
            <span v-if="selectedTpl" class="text-xs text-text-muted">
              当前载入: <span class="font-mono text-blue-400">{{ selectedTpl.commandKey }}</span>
              ({{ selectedTpl.engine }})
            </span>
          </div>
          <div class="flex items-center gap-2">
            <el-button type="success" size="small" :loading="testing" @click="runTest">
              ▶ 运行解析测试
            </el-button>
          </div>
        </div>

        <div class="grid grid-rows-2 gap-3 flex-1 min-h-0 mt-3">
          <!-- 原始回显输入区 -->
          <div class="flex flex-col min-h-0">
            <div class="text-xs text-text-muted mb-1 flex justify-between items-center">
              <span>原始 CLI 回显文本 (Raw Echo)</span>
              <span class="text-text-muted/60 text-[11px]">粘贴设备真实回显进行验证</span>
            </div>
            <textarea
              v-model="rawEchoInput"
              class="flex-1 w-full bg-bg-primary text-text-primary p-2.5 rounded border border-border font-mono text-xs focus:outline-none focus:border-blue-500 resize-none"
              placeholder="在此粘贴设备的命令原始回显文本，例如 display device、display interface 等..."
            ></textarea>
          </div>

          <!-- 解析结果多视图预览区 (表格 / 树状 / JSON 格式化) -->
          <div class="flex flex-col min-h-0 border border-border rounded bg-bg-primary p-3">
            <div class="flex items-center justify-between pb-2 border-b border-border/50 text-xs">
              <div class="flex items-center gap-3">
                <span class="font-medium text-text-primary">
                  解析结果
                  <span v-if="testResult && testResult.success" class="text-blue-400 font-mono">
                    ({{ testResult.count }} 行)
                  </span>
                </span>
                <el-radio-group v-model="resultViewMode" size="small">
                  <el-radio-button label="table">📊 表格视图</el-radio-button>
                  <el-radio-button label="tree">🌲 树状视图</el-radio-button>
                  <el-radio-button label="json">📄 JSON 格式化</el-radio-button>
                </el-radio-group>
              </div>
              <span v-if="testResult?.error" class="text-rose-400 truncate max-w-sm text-xs" :title="testResult.error">
                ⚠ {{ testResult.error }}
              </span>
            </div>

            <div class="flex-1 overflow-auto mt-2">
              <!-- 表格视图 -->
              <table v-if="resultViewMode === 'table' && testResult && testResult.results.length > 0" class="w-full text-xs text-left border-collapse">
                <thead>
                  <tr class="bg-bg-secondary/60 text-text-muted border-b border-border font-mono">
                    <th class="p-1.5 w-10 text-center">#</th>
                    <th v-for="col in resultColumns" :key="col" class="p-1.5 font-medium">
                      {{ col }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(row, idx) in testResult.results"
                    :key="idx"
                    class="border-b border-border/30 hover:bg-bg-hover/50 font-mono"
                  >
                    <td class="p-1.5 text-center text-text-muted/60">{{ idx + 1 }}</td>
                    <td v-for="col in resultColumns" :key="col" class="p-1.5 text-text-primary">
                      {{ row[col] !== undefined && row[col] !== '' ? row[col] : '-' }}
                    </td>
                  </tr>
                </tbody>
              </table>

              <!-- 树状折叠视图 (真正的 Tree View) -->
              <div v-else-if="resultViewMode === 'tree' && testResult && testResult.results.length > 0" class="space-y-2 p-1">
                <el-collapse>
                  <el-collapse-item
                    v-for="(row, idx) in testResult.results"
                    :key="idx"
                    :name="idx"
                  >
                    <template #title>
                      <div class="flex items-center gap-2 font-mono text-xs">
                        <span class="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 font-bold">#{{ idx + 1 }}</span>
                        <span class="text-text-primary font-medium">{{ getRowPrimarySummary(row) }}</span>
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
                v-else-if="resultViewMode === 'json' && testResult && testResult.results.length > 0"
                class="h-full bg-bg-secondary/60 p-3 rounded text-xs font-mono overflow-auto text-emerald-400 select-text"
              >{{ JSON.stringify(testResult.results, null, 2) }}</pre>

              <!-- 空状态 -->
              <div v-else-if="!testing && (!testResult || testResult.results.length === 0)" class="h-full flex items-center justify-center text-xs text-text-muted">
                点击上方"运行解析测试"查看解析结果。
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 模板新增/编辑弹窗 (支持 groupIndex 编辑、清空 aggregate、M5 锁定、边改边测) -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑解析模板' : '新建解析模板'"
      width="960px"
      destroy-on-close
      top="5vh"
    >
      <el-form :model="formData" label-width="110px" size="small">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="厂商">
              <el-select v-model="formData.vendor" class="w-full" :disabled="isEdit">
                <el-option label="华为 (Huawei)" value="huawei" />
                <el-option label="新华三 (H3C)" value="h3c" />
                <el-option label="思科 (Cisco)" value="cisco" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="命令标识 (Key)">
              <el-input
                v-model="formData.commandKey"
                placeholder="如 display_device、display_interface 等"
                :disabled="isEdit"
              />
              <span v-if="isEdit" class="text-[11px] text-text-muted">
                * 模板唯一标识，已锁定不可篡改
              </span>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="解析引擎">
              <el-select v-model="formData.engine" class="w-full">
                <el-option label="tree (二维规则树，推荐表格/分块型回显)" value="tree" />
                <el-option label="regex (单条纯正则，支持命名捕获组)" value="regex" />
                <el-option label="aggregate (多行状态机聚合)" value="aggregate" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="启用状态">
              <el-switch v-model="formData.enabled" active-text="已启用" inactive-text="已停用" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="描述说明">
          <el-input v-model="formData.description" placeholder="模板用途或版本说明" />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="适用款型 (Models)">
              <el-input
                v-model="appliesToModelsInput"
                placeholder="如 S57*, CE68*, NE40E (逗号分隔，留空适用全款型)"
                clearable
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="适用版本 (Versions)">
              <el-input
                v-model="appliesToVersionsInput"
                placeholder="如 V200*, V300R019* (逗号分隔，留空适用全版本)"
                clearable
              />
            </el-form-item>
          </el-col>
        </el-row>

        <!-- Tree 规则树配置区 (含 groupIndex 编辑) -->
        <div v-if="formData.engine === 'tree'" class="border border-border rounded p-3 bg-bg-secondary/40 mb-3">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-3">
              <span class="font-medium text-xs text-text-primary">规则树规则定义 (Tree Rules)</span>
              <div class="flex items-center gap-1.5">
                <span class="text-xs text-text-muted">拍平最大深度 (MaxLevel):</span>
                <el-input-number
                  v-model="maxOutputLevel"
                  :min="0"
                  :max="10"
                  size="small"
                  controls-position="right"
                  class="w-20"
                />
                <span class="text-[11px] text-text-muted">(0 为不限深度)</span>
              </div>
            </div>
            <el-button type="primary" size="small" link @click="addTreeRule">+ 添加规则项</el-button>
          </div>

          <el-table :data="treeRules" size="small" border class="w-full" max-height="260">
            <el-table-column label="字段名" min-width="95">
              <template #default="{ row }">
                <el-input v-model="row.parseItem" size="small" placeholder="如 slot" />
              </template>
            </el-table-column>
            <el-table-column label="父字段" min-width="85">
              <template #default="{ row }">
                <el-input v-model="row.parentItem" size="small" placeholder="空为根" />
              </template>
            </el-table-column>
            <el-table-column label="列表?" width="55" align="center">
              <template #default="{ row }">
                <el-checkbox v-model="row.isList" />
              </template>
            </el-table-column>
            <el-table-column label="分块正则 (SplitRegex)" min-width="135">
              <template #default="{ row }">
                <el-input v-model="row.splitRegex" size="small" placeholder="如 (?m)^Slot\s+(\d+):" />
                <div v-if="checkUnsupportedRegex(row.splitRegex)" class="text-[10px] text-amber-400">
                  ⚠ 包含不支持的前瞻/后顾
                </div>
              </template>
            </el-table-column>
            <el-table-column label="取值正则 (ParseRegex)" min-width="135">
              <template #default="{ row }">
                <el-input v-model="row.parseRegex" size="small" placeholder="如 Type:\s*(\S+)" />
                <div v-if="checkUnsupportedRegex(row.parseRegex)" class="text-[10px] text-amber-400">
                  ⚠ 包含不支持的前瞻/后顾
                </div>
              </template>
            </el-table-column>
            <el-table-column label="组号" width="60" align="center">
              <template #default="{ row }">
                <el-input-number
                  v-model="row.groupIndex"
                  :min="1"
                  :max="9"
                  size="small"
                  controls-position="right"
                  class="w-full"
                />
              </template>
            </el-table-column>
            <el-table-column label="输出?" width="55" align="center">
              <template #default="{ row }">
                <el-checkbox v-model="row.isOutput" />
              </template>
            </el-table-column>
            <el-table-column label="默认值" min-width="70">
              <template #default="{ row }">
                <el-input v-model="row.defaultValue" size="small" placeholder="空或N/A" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="50" align="center">
              <template #default="{ $index }">
                <el-button type="danger" link size="small" @click="removeTreeRule($index)">删</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="text-[11px] text-text-muted mt-2">
            💡 规则说明：分块正则 (SplitRegex) 匹配各块头；取值正则 (ParseRegex) 匹配属性；组号指定抽取第几捕获组（缺省 1）；Go RE2 建议使用 <code class="text-amber-300">(?m)</code> 开启多行，不支持正向/负向预查。
          </div>
        </div>

        <!-- Regex 正则配置区 -->
        <div v-if="formData.engine === 'regex'" class="border border-border rounded p-3 bg-bg-secondary/40 mb-3 space-y-2">
          <div class="flex items-center justify-between">
            <span class="font-medium text-xs text-text-primary">单条正则匹配定义</span>
            <el-checkbox v-model="formData.multiline">启用多行匹配 (Multiline)</el-checkbox>
          </div>
          <el-form-item label="正则模式" class="!mb-0">
            <el-input
              v-model="formData.pattern"
              type="textarea"
              :rows="3"
              placeholder="如 (?m)^(?P<interface>\S+)\s+(?P<status>UP|DOWN)\s+(?P<protocol>UP|DOWN)"
            />
          </el-form-item>
          <div v-if="checkUnsupportedRegex(formData.pattern)" class="text-[11px] text-amber-400">
            ⚠ 检测到模式中包含 Go RE2 不支持的前瞻 (?= 或后顾 (?<= 语法，请改写为捕获组。
          </div>
          <div class="text-[11px] text-text-muted">
            💡 支持 Go 语法命名捕获组 <code class="text-amber-300">(?P&lt;name&gt;...)</code> 或顺序捕获组（无命名组将按 "1", "2" 数字键输出）。
          </div>
        </div>

        <!-- Aggregate 聚合配置区 -->
        <div v-if="formData.engine === 'aggregate'" class="border border-border rounded p-3 bg-bg-secondary/40 mb-3 space-y-2">
          <div class="flex items-center justify-between">
            <span class="font-medium text-xs text-text-primary">多行聚合状态机配置 (JSON)</span>
            <span class="text-[11px] text-text-muted">清空文本框并保存即可清除旧聚合配置</span>
          </div>
          <el-form-item label="聚合 JSON" class="!mb-0">
            <el-input
              v-model="aggregateJsonStr"
              type="textarea"
              :rows="4"
              class="font-mono text-xs"
              placeholder='{\n  "recordStart": ["^Eth-Trunk"],\n  "captureRules": [\n    {"pattern": "Eth-Trunk(?P<trunkId>\\d+)", "mode": "set"}\n  ]\n}'
            />
          </el-form-item>
        </div>

        <!-- 字段映射配置折叠区 -->
        <el-collapse class="mb-3">
          <el-collapse-item title="字段重命名映射 (Field Mapping, 可选)" name="fieldMapping">
            <div class="space-y-2 p-2 bg-bg-secondary/30 rounded">
              <div class="flex justify-between items-center text-xs text-text-muted">
                <span>将提取的原始字段名重命名为标准字段名</span>
                <el-button size="small" link type="primary" @click="addFieldMappingRow">+ 添加映射项</el-button>
              </div>
              <div v-for="(item, idx) in fieldMappingList" :key="idx" class="flex items-center gap-2">
                <el-input v-model="item.src" size="small" placeholder="原始提取字段 (如 1 或 slotId)" class="flex-1" />
                <span class="text-text-muted">➔</span>
                <el-input v-model="item.dest" size="small" placeholder="重命名输出 (如 slot_id)" class="flex-1" />
                <el-button size="small" link type="danger" @click="removeFieldMappingRow(idx)">删</el-button>
              </div>
              <div v-if="fieldMappingList.length === 0" class="text-xs text-text-muted text-center py-1">
                暂无映射配置，保持原始提取字段名输出。
              </div>
            </div>
          </el-collapse-item>
        </el-collapse>

        <!-- 边改边测 (Live Test) 折叠自测区 -->
        <el-collapse>
          <el-collapse-item title="🧪 边改边测 (无需保存，直接自测当前表单规则)" name="liveTest">
            <div class="space-y-2 p-2 bg-bg-secondary/30 rounded">
              <div class="flex justify-between items-center">
                <span class="text-xs text-text-muted">粘贴测试回显：</span>
                <el-button type="success" size="small" :loading="liveTesting" @click="runLiveTest">
                  ⚡ 立即测试当前规则
                </el-button>
              </div>
              <el-input
                v-model="liveTestInput"
                type="textarea"
                :rows="3"
                placeholder="粘贴一段简短的 CLI 回显文本以验证上方编辑的规则..."
                class="font-mono text-xs"
              />
              <div v-if="liveTestResult" class="p-2 rounded border border-border/50 bg-bg-primary text-xs font-mono">
                <div v-if="!liveTestResult.success" class="text-rose-400">
                  ❌ 测试失败: {{ liveTestResult.error }}
                </div>
                <div v-else class="text-emerald-400">
                  ✔ 测试通过: 成功解析出 {{ liveTestResult.count }} 行数据
                  <div class="max-h-36 overflow-auto mt-1 border-t border-border/30 pt-1 text-text-primary">
                    <pre>{{ JSON.stringify(liveTestResult.results, null, 2) }}</pre>
                  </div>
                </div>
              </div>
            </div>
          </el-collapse-item>
        </el-collapse>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">保存模板</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  ParseTemplateAPI,
  type UserParseTemplateVO,
  type SaveParseTemplateRequest,
  type TestParseTemplateResult,
  type TreeRule
} from '@/services/parseTemplateApi';

const selectedVendor = ref('huawei');
const templates = ref<UserParseTemplateVO[]>([]);
const selectedTpl = ref<UserParseTemplateVO | null>(null);

const rawEchoInput = ref('');
const testing = ref(false);
const testResult = ref<TestParseTemplateResult | null>(null);
const resultViewMode = ref<'table' | 'tree' | 'json'>('table');

const dialogVisible = ref(false);
const isEdit = ref(false);
const saving = ref(false);

// 保持底层原数据的副本，防止更新时字段被意外清空 (H1 修复)
const originalEditingTpl = ref<UserParseTemplateVO | null>(null);

const formData = ref<SaveParseTemplateRequest>({
  vendor: 'huawei',
  commandKey: '',
  engine: 'tree',
  pattern: '',
  multiline: false,
  description: '',
  enabled: true,
  aggregation: {},
  parseRules: {},
  fieldMapping: {},
});

const maxOutputLevel = ref<number>(0);
const treeRules = ref<TreeRule[]>([]);
const aggregateJsonStr = ref('');
const fieldMappingList = ref<{ src: string; dest: string }[]>([]);
const appliesToModelsInput = ref('');
const appliesToVersionsInput = ref('');

function getAppliesToText(appliesTo?: { models?: string[]; versions?: string[] } | null): string {
  if (!appliesTo) return '';
  const parts: string[] = [];
  if (appliesTo.models && appliesTo.models.length > 0) {
    parts.push(`款型: ${appliesTo.models.join(', ')}`);
  }
  if (appliesTo.versions && appliesTo.versions.length > 0) {
    parts.push(`版本: ${appliesTo.versions.join(', ')}`);
  }
  return parts.join(' | ');
}

// 边改边测状态
const liveTestInput = ref('');
const liveTesting = ref(false);
const liveTestResult = ref<TestParseTemplateResult | null>(null);

const resultColumns = computed(() => {
  if (!testResult.value || testResult.value.results.length === 0) return [];
  const keys = new Set<string>();
  testResult.value.results.forEach(row => {
    Object.keys(row).forEach(k => keys.add(k));
  });
  return Array.from(keys);
});

function getRowPrimarySummary(row: Record<string, any>): string {
  const keys = Object.keys(row);
  if (keys.length === 0) return '空数据行';
  const firstKey = keys[0];
  if (!firstKey) return '空数据行';
  return `${firstKey} = ${row[firstKey] || '-'}`;
}

function checkUnsupportedRegex(pat?: string): boolean {
  if (!pat) return false;
  return pat.includes('(?=') || pat.includes('(?!') || pat.includes('(?<=') || pat.includes('(?<!');
}

async function loadTemplates() {
  try {
    templates.value = await ParseTemplateAPI.listTemplates(selectedVendor.value);
    if (templates.value.length > 0 && templates.value[0] && (!selectedTpl.value || selectedTpl.value.vendor !== selectedVendor.value)) {
      selectTemplate(templates.value[0]);
    } else if (selectedTpl.value) {
      const found = templates.value.find(t => t.id === selectedTpl.value?.id);
      if (found) selectTemplate(found);
    }
  } catch (err: any) {
    ElMessage.error(`加载模板列表失败: ${err.message || err}`);
  }
}

function selectTemplate(tpl: UserParseTemplateVO) {
  selectedTpl.value = tpl;
}

function getEngineBadgeClass(engine: string) {
  switch (engine) {
    case 'tree':
      return 'bg-purple-500/10 text-purple-400 border border-purple-500/20';
    case 'aggregate':
      return 'bg-amber-500/10 text-amber-400 border border-amber-500/20';
    default:
      return 'bg-blue-500/10 text-blue-400 border border-blue-500/20';
  }
}

function openCreateDialog() {
  isEdit.value = false;
  originalEditingTpl.value = null;
  formData.value = {
    vendor: selectedVendor.value,
    commandKey: '',
    engine: 'tree',
    pattern: '',
    multiline: false,
    description: '',
    enabled: true,
    aggregation: {},
    parseRules: {},
    fieldMapping: {},
  };
  appliesToModelsInput.value = '';
  appliesToVersionsInput.value = '';
  maxOutputLevel.value = 0;
  treeRules.value = [
    { parseItem: 'slot', parentItem: '', isList: true, splitRegex: '(?m)^Slot\\s+(\\d+):', groupIndex: 1, isOutput: true, order: 1 },
    { parseItem: 'boardType', parentItem: 'slot', isList: false, parseRegex: 'Board Type\\s*:\\s*(\\S+)', groupIndex: 1, isOutput: true, defaultValue: 'N/A', order: 2 },
  ];
  aggregateJsonStr.value = '';
  fieldMappingList.value = [];
  liveTestInput.value = rawEchoInput.value;
  liveTestResult.value = null;
  dialogVisible.value = true;
}

function openEditDialog(tpl: UserParseTemplateVO) {
  isEdit.value = true;
  originalEditingTpl.value = JSON.parse(JSON.stringify(tpl));
  formData.value = {
    vendor: tpl.vendor,
    commandKey: tpl.commandKey,
    engine: tpl.engine,
    pattern: tpl.pattern || '',
    multiline: tpl.multiline || false,
    description: tpl.description || '',
    enabled: tpl.enabled,
    aggregation: tpl.aggregation || {},
    parseRules: tpl.parseRules || {},
    fieldMapping: tpl.fieldMapping || {},
  };
  appliesToModelsInput.value = tpl.appliesTo?.models ? tpl.appliesTo.models.join(', ') : '';
  appliesToVersionsInput.value = tpl.appliesTo?.versions ? tpl.appliesTo.versions.join(', ') : '';

  // 回填 tree 规则 (含 groupIndex)
  if (tpl.parseRules?.rules) {
    treeRules.value = tpl.parseRules.rules.map(r => ({
      ...r,
      groupIndex: r.groupIndex || 1,
    }));
    maxOutputLevel.value = tpl.parseRules.maxOutputLevel || 0;
  } else {
    treeRules.value = [];
    maxOutputLevel.value = 0;
  }

  // 回填 aggregate 规则
  if (tpl.aggregation && Object.keys(tpl.aggregation).length > 0) {
    aggregateJsonStr.value = JSON.stringify(tpl.aggregation, null, 2);
  } else {
    aggregateJsonStr.value = '';
  }

  // 回填 fieldMapping
  fieldMappingList.value = [];
  if (tpl.fieldMapping) {
    for (const [src, dest] of Object.entries(tpl.fieldMapping)) {
      if (dest) {
        fieldMappingList.value.push({ src, dest });
      }
    }
  }

  liveTestInput.value = rawEchoInput.value;
  liveTestResult.value = null;
  dialogVisible.value = true;
}

function addTreeRule() {
  treeRules.value.push({
    parseItem: '',
    parentItem: '',
    isList: false,
    parseRegex: '',
    splitRegex: '',
    groupIndex: 1,
    isOutput: true,
    defaultValue: '',
    order: treeRules.value.length + 1,
  });
}

function removeTreeRule(idx: number) {
  treeRules.value.splice(idx, 1);
}

function addFieldMappingRow() {
  fieldMappingList.value.push({ src: '', dest: '' });
}

function removeFieldMappingRow(idx: number) {
  fieldMappingList.value.splice(idx, 1);
}

// 组装完整的请求体（严格保留未编辑字段与格式转换，支持显式清空）
function buildRequestPayload(): SaveParseTemplateRequest {
  const req: SaveParseTemplateRequest = {
    ...formData.value,
  };

  // 处理引擎特定规则
  if (formData.value.engine === 'tree') {
    req.parseRules = {
      rules: treeRules.value,
      maxOutputLevel: maxOutputLevel.value > 0 ? maxOutputLevel.value : undefined,
    };
    if (originalEditingTpl.value?.aggregation) {
      req.aggregation = originalEditingTpl.value.aggregation;
    }
  } else if (formData.value.engine === 'aggregate') {
    if (aggregateJsonStr.value.trim()) {
      try {
        req.aggregation = JSON.parse(aggregateJsonStr.value);
      } catch (err: any) {
        throw new Error(`聚合 JSON 格式非法: ${err.message}`);
      }
    } else {
      // 用户显式清空了 aggregate 文本，传递空对象以触发后端清空
      req.aggregation = {};
    }
    if (originalEditingTpl.value?.parseRules) {
      req.parseRules = originalEditingTpl.value.parseRules;
    }
  } else if (formData.value.engine === 'regex') {
    req.pattern = formData.value.pattern;
    if (originalEditingTpl.value?.parseRules) {
      req.parseRules = originalEditingTpl.value.parseRules;
    }
    if (originalEditingTpl.value?.aggregation) {
      req.aggregation = originalEditingTpl.value.aggregation;
    }
  }

  // 转换 fieldMapping
  const mappingMap: Record<string, string> = {};
  for (const item of fieldMappingList.value) {
    if (item.src.trim() && item.dest.trim()) {
      mappingMap[item.src.trim()] = item.dest.trim();
    }
  }
  req.fieldMapping = mappingMap;

  // 转换 appliesTo
  const models = appliesToModelsInput.value
    .split(/[,，\s]+/)
    .map(s => s.trim())
    .filter(Boolean);
  const versions = appliesToVersionsInput.value
    .split(/[,，\s]+/)
    .map(s => s.trim())
    .filter(Boolean);
  if (models.length > 0 || versions.length > 0) {
    req.appliesTo = { models, versions };
  } else {
    req.appliesTo = undefined;
  }

  return req;
}

async function handleSave() {
  if (!formData.value.commandKey) {
    ElMessage.warning('请输入命令标识');
    return;
  }
  saving.value = true;
  try {
    const req = buildRequestPayload();
    if (isEdit.value && selectedTpl.value) {
      await ParseTemplateAPI.updateTemplate(selectedTpl.value.id, req);
      ElMessage.success('模板更新成功并已同步加载到解析器快照！');
    } else {
      await ParseTemplateAPI.createTemplate(req);
      ElMessage.success('模板创建成功并已热加载！');
    }
    dialogVisible.value = false;
    await loadTemplates();
  } catch (err: any) {
    ElMessage.error(`保存模板失败: ${err.message || err}`);
  } finally {
    saving.value = false;
  }
}

async function handleDelete(tpl: UserParseTemplateVO) {
  try {
    await ElMessageBox.confirm(`确定要删除模板 "${tpl.commandKey}" 吗？`, '删除确认', {
      type: 'warning',
    });
    await ParseTemplateAPI.deleteTemplate(tpl.id);
    ElMessage.success('模板已删除');
    if (selectedTpl.value?.id === tpl.id) {
      selectedTpl.value = null;
    }
    await loadTemplates();
  } catch {
    // 用户取消删除
  }
}

// 主界面测试：完整传递全部参数
async function runTest() {
  if (!selectedTpl.value) {
    ElMessage.warning('请先从左侧选择一个模板');
    return;
  }
  if (!rawEchoInput.value.trim()) {
    ElMessage.warning('请粘贴原始 CLI 回显内容');
    return;
  }
  testing.value = true;
  testResult.value = null;
  try {
    const res = await ParseTemplateAPI.testTemplate({
      vendor: selectedTpl.value.vendor,
      commandKey: selectedTpl.value.commandKey,
      engine: selectedTpl.value.engine,
      pattern: selectedTpl.value.pattern,
      multiline: selectedTpl.value.multiline,
      parseRules: selectedTpl.value.parseRules,
      aggregation: selectedTpl.value.aggregation,
      appliesTo: selectedTpl.value.appliesTo,
      fieldMapping: selectedTpl.value.fieldMapping,
      rawText: rawEchoInput.value,
    });
    testResult.value = res;
    if (!res.success) {
      ElMessage.error(`解析失败: ${res.error}`);
    } else {
      ElMessage.success(`解析成功，生成 ${res.count} 行数据`);
    }
  } catch (err: any) {
    ElMessage.error(`测试执行异常: ${err.message || err}`);
  } finally {
    testing.value = false;
  }
}

// 弹窗内边改边测
async function runLiveTest() {
  if (!liveTestInput.value.trim()) {
    ElMessage.warning('请先输入待测的回显文本');
    return;
  }
  liveTesting.value = true;
  liveTestResult.value = null;
  try {
    const req = buildRequestPayload();
    const res = await ParseTemplateAPI.testTemplate({
      vendor: req.vendor,
      commandKey: req.commandKey || 'test_key',
      engine: req.engine,
      pattern: req.pattern,
      multiline: req.multiline,
      parseRules: req.parseRules,
      aggregation: req.aggregation,
      appliesTo: req.appliesTo,
      fieldMapping: req.fieldMapping,
      rawText: liveTestInput.value,
    });
    liveTestResult.value = res;
    if (res.success) {
      ElMessage.success(`自测成功，得到 ${res.count} 行结果`);
    } else {
      ElMessage.warning(`自测失败: ${res.error}`);
    }
  } catch (err: any) {
    liveTestResult.value = {
      success: false,
      results: [],
      count: 0,
      error: err.message || String(err),
    };
  } finally {
    liveTesting.value = false;
  }
}

onMounted(() => {
  loadTemplates();
});
</script>
