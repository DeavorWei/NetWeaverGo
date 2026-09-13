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
      <!-- 模板列表（组件化，方案 §4.3.6） -->
      <TemplateList
        :templates="templates"
        :selected-id="selectedTpl?.id"
        @refresh="loadTemplates"
        @select="selectTemplate"
        @edit="openEditDialog"
        @delete="handleDelete"
      />

      <!-- 右侧在线测试台（组件化，方案 §4.3.6） -->
      <EchoTestPane
        v-model:raw="rawEchoInput"
        v-model:view-mode="resultViewMode"
        :selected-tpl="selectedTpl"
        :result="testResult"
        :testing="testing"
        :columns="resultColumns"
        @run="runTest"
      />
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

        <!-- Tree 规则树配置区（组件化，方案 §4.3.6） -->
        <div v-if="formData.engine === 'tree'" class="mb-3">
          <RuleTableEditor
            v-model:max-level="maxOutputLevel"
            :rules="treeRules"
            @add="addTreeRule"
            @remove="removeTreeRule"
            @move="moveTreeRule"
          />
        </div>

        <!-- Regex 正则配置区（组件化） -->
        <div v-if="formData.engine === 'regex'" class="mb-3">
          <RegexPlayground
            v-model:pattern="formData.pattern"
            v-model:multiline="formData.multiline"
          />
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
// 解析模板工作台子组件（方案 §4.3.6 组件化）
import TemplateList from '@/components/parsetpl/TemplateList.vue';
import EchoTestPane from '@/components/parsetpl/EchoTestPane.vue';
import RuleTableEditor from '@/components/parsetpl/RuleTableEditor.vue';
import RegexPlayground from '@/components/parsetpl/RegexPlayground.vue';

const selectedVendor = ref('huawei');
const templates = ref<UserParseTemplateVO[]>([]);
const selectedTpl = ref<UserParseTemplateVO | null>(null);

const rawEchoInput = ref('');
const testing = ref(false);
const testResult = ref<TestParseTemplateResult | null>(null);
const resultViewMode = ref<'table' | 'tree' | 'json' | 'highlight'>('table');

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

// 上移/下移规则项：数组顺序即执行次序，重排后按新顺序回写 order 字段
function moveTreeRule(idx: number, delta: -1 | 1) {
  const list = treeRules.value;
  const target = idx + delta;
  if (idx < 0 || target < 0 || idx >= list.length || target >= list.length) return;
  const moving = list[idx];
  const displaced = list[target];
  if (!moving || !displaced) return;
  list[idx] = displaced;
  list[target] = moving;
  list.forEach((rule, i) => {
    rule.order = i + 1;
  });
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
