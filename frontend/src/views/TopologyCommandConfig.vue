<template>
  <div class="space-y-5 animate-slide-in h-full flex flex-col">
    <!-- 顶部 Header 区域 -->
    <div class="flex items-center justify-between gap-3 flex-shrink-0">
      <div>
        <h2 class="text-lg font-bold text-text-primary tracking-wide">拓扑命令配置中心</h2>
        <p class="text-xs text-text-muted mt-1">
          统一维护厂商默认采集命令，支持字段级编辑、启停控制和超时管理。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <el-button
          :icon="RefreshRight"
          :disabled="loadingConfig"
          @click="refreshCurrentVendor"
          class="!rounded-lg"
        >
          刷新
        </el-button>
        <el-button
          type="warning"
          plain
          :disabled="!selectedVendor || loadingConfig || resetting"
          @click="resetVendor"
          class="!rounded-lg"
        >
          {{ resetting ? "重置中..." : "重置为默认" }}
        </el-button>
        <el-button
          type="primary"
          :disabled="!canSave"
          :loading="saving"
          @click="saveVendor"
          class="!rounded-lg shadow-sm"
        >
          保存配置
        </el-button>
      </div>
    </div>

    <!-- 异常状态提示 -->
    <el-alert
      v-if="invalidCount > 0"
      type="error"
      :closable="false"
      class="flex-shrink-0 !rounded-lg"
      show-icon
      :title="`存在 ${invalidCount} 条无效配置（启用但命令为空，或超时时间小于等于 0），请修正后再保存。`"
    />

    <!-- 两栏主体区域 -->
    <div class="flex-1 flex flex-col md:flex-row gap-5 min-h-0">
      <!-- 左栏 Master: 厂商与字段导航列表 -->
      <div class="w-full md:w-80 flex-shrink-0 flex flex-col h-full bg-bg-card border border-border rounded-2xl overflow-hidden shadow-sm">
        
        <!-- 厂商选择与统计看板 -->
        <div class="p-4 border-b border-border space-y-4 bg-bg-secondary/40">
          <div class="space-y-1.5">
            <span class="text-[11px] font-semibold text-text-muted tracking-wider uppercase">厂商设备系列</span>
            <el-select
              v-model="selectedVendor"
              placeholder="请选择厂商"
              class="w-full"
              :disabled="loadingVendors || loadingConfig"
            >
              <el-option v-for="vendor in vendors" :key="vendor" :label="vendor" :value="vendor" />
            </el-select>
          </div>
          
          <!-- 统计指标小磁贴 -->
          <div class="grid grid-cols-2 gap-2" v-if="selectedVendor">
            <div class="bg-bg-panel border border-border/80 rounded-xl p-2.5 text-center transition-all hover:border-border">
              <span class="block text-lg font-bold text-text-primary">{{ commands.length }}</span>
              <span class="text-[10px] text-text-muted uppercase tracking-wider">总字段数</span>
            </div>
            <div class="bg-bg-panel border border-border/80 rounded-xl p-2.5 text-center transition-all hover:border-border">
              <span class="block text-lg font-bold text-success">{{ enabledCount }}</span>
              <span class="text-[10px] text-text-muted uppercase tracking-wider">已启用数</span>
            </div>
          </div>
        </div>

        <!-- 字段快速检索 -->
        <div class="px-4 py-3 border-b border-border bg-bg-secondary/20" v-if="selectedVendor">
          <el-input
            v-model="searchQuery"
            placeholder="搜索字段或 Key..."
            clearable
            size="default"
          >
            <template #prefix>
              <el-icon class="text-text-muted"><Search /></el-icon>
            </template>
          </el-input>
        </div>

        <!-- 字段导航列表 -->
        <div class="flex-1 overflow-y-auto p-3 space-y-2 bg-bg-secondary/10" v-loading="loadingConfig">
          <div
            v-for="item in filteredCommands"
            :key="item.fieldKey"
            @click="selectedFieldKey = item.fieldKey"
            class="group relative p-3 border rounded-xl cursor-pointer transition-all duration-200 select-none flex flex-col gap-1.5"
            :class="[
              selectedFieldKey === item.fieldKey
                ? 'bg-accent/10 border-accent shadow-sm'
                : 'bg-bg-panel border-border/60 hover:bg-bg-hover hover:border-border hover:translate-x-0.5'
            ]"
          >
            <!-- 字段显示名称 & 启用状态指示点 -->
            <div class="flex items-center justify-between gap-2">
              <span 
                class="font-bold text-sm text-text-primary line-clamp-1 transition-colors"
                :class="{'text-accent': selectedFieldKey === item.fieldKey}"
              >
                {{ item.displayName || item.fieldKey }}
              </span>
              <span 
                class="w-2 h-2 rounded-full flex-shrink-0 transition-all"
                :class="[item.enabled ? 'bg-success shadow-[0_0_5px_var(--color-success)]' : 'bg-text-muted/30']"
                :title="item.enabled ? '已启用' : '已停用'"
              ></span>
            </div>

            <!-- 字段 Key -->
            <span class="text-[11px] text-text-muted font-mono tracking-wide break-all">
              {{ item.fieldKey }}
            </span>

            <!-- 状态标签 -->
            <div class="flex flex-wrap items-center gap-1.5 mt-0.5">
              <el-tag
                v-if="item.required"
                type="warning"
                effect="light"
                size="small"
                class="!px-1.5 !h-4.5 !line-height-4.5 !rounded-md"
              >
                必填
              </el-tag>
              <el-tag
                v-if="isRowDirty(item.fieldKey)"
                type="danger"
                effect="light"
                size="small"
                class="!px-1.5 !h-4.5 !line-height-4.5 !rounded-md"
              >
                已修改
              </el-tag>
            </div>
          </div>
          
          <div v-if="filteredCommands.length === 0" class="text-center py-8 text-xs text-text-muted">
            {{ selectedVendor ? '未找到匹配字段' : '请先选择厂商' }}
          </div>
        </div>
      </div>

      <!-- 右栏 Detail: 详细配置面板 -->
      <div class="flex-1 flex flex-col min-h-0 bg-bg-card border border-border rounded-2xl overflow-hidden shadow-sm">
        <div v-if="currentField" class="flex-1 flex flex-col min-h-0">
          
          <!-- 选中字段头部描述 -->
          <div class="p-5 border-b border-border bg-bg-secondary/40 flex items-start justify-between gap-4">
            <div class="space-y-1">
              <div class="flex items-center gap-2.5">
                <h3 class="text-base font-bold text-text-primary">{{ currentField.displayName || currentField.fieldKey }}</h3>
                <span class="text-xs font-mono px-2 py-0.5 rounded bg-bg-panel border border-border text-text-secondary">{{ currentField.fieldKey }}</span>
              </div>
              <p class="text-xs text-text-muted leading-relaxed max-w-2xl">
                {{ currentField.description || "暂无该字段的详细描述说明。" }}
              </p>
            </div>
            <div class="flex items-center gap-2 flex-shrink-0">
              <el-tag v-if="currentField.required" type="warning" effect="dark" size="default" class="!rounded-md">
                必填字段
              </el-tag>
              <el-button
                v-if="isRowDirty(currentField.fieldKey)"
                type="info"
                plain
                size="small"
                :icon="RefreshLeft"
                @click="resetRow(currentField.fieldKey)"
                class="!rounded-lg"
              >
                撤销修改
              </el-button>
            </div>
          </div>

          <!-- 主编辑区域 -->
          <div class="flex-1 overflow-y-auto p-6 space-y-6">
            <!-- 采集命令配置 -->
            <div class="space-y-2 flex flex-col">
              <div class="flex items-center justify-between">
                <label class="text-xs font-semibold text-text-muted uppercase tracking-wider flex items-center gap-1.5">
                  <el-icon><Cpu /></el-icon>
                  采集命令 (CLI Command)
                </label>
                <span class="text-[11px] text-text-muted font-mono bg-bg-panel px-2 py-0.5 rounded border border-border/60">
                  解析绑定: {{ currentField.parserBinding || "无" }}
                </span>
              </div>
              
              <!-- 代码风格输入区域 -->
              <div class="border border-border/80 focus-within:border-accent focus-within:shadow-[0_0_0_3px_var(--color-accent-bg)] rounded-xl bg-bg-panel overflow-hidden transition-all flex flex-col h-44">
                <div class="flex-1 relative">
                  <textarea
                    v-model="currentField.command"
                    placeholder="请输入用于采集该字段数据的设备 CLI 命令，例如: display version"
                    @input="markDirty"
                    class="premium-textarea h-full"
                  ></textarea>
                </div>
              </div>
            </div>

            <!-- 参数属性 Grid 区域 -->
            <div class="bg-bg-secondary/20 border border-border/60 rounded-xl p-5 space-y-5">
              <h4 class="text-xs font-semibold text-text-muted uppercase tracking-wider mb-2">配置参数选项</h4>
              
              <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
                <!-- 超时秒数 -->
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <label class="text-xs font-medium text-text-primary">采集超时时间 (秒)</label>
                    <el-tooltip content="设置此命令执行的最大等待秒数，超时将自动终止" placement="top">
                      <el-icon class="text-text-muted cursor-pointer"><QuestionFilled /></el-icon>
                    </el-tooltip>
                  </div>
                  <el-input-number
                    v-model="currentField.timeoutSec"
                    :min="1"
                    :max="3600"
                    class="w-full"
                    controls-position="right"
                    @change="markDirty"
                  />
                </div>

                <!-- 启用控制 -->
                <div class="space-y-2">
                  <label class="text-xs font-medium text-text-primary block">启用状态</label>
                  <div class="flex items-center gap-3 py-1 px-3 bg-bg-panel border border-border/60 rounded-lg h-[32px]">
                    <el-switch
                      v-model="currentField.enabled"
                      inline-prompt
                      active-text="启用"
                      inactive-text="停用"
                      @change="markDirty"
                    />
                    <span class="text-xs text-text-muted">
                      {{ currentField.enabled ? '在任务采集时执行此命令' : '跳过此字段采集' }}
                    </span>
                  </div>
                </div>

                <!-- 备注信息 -->
                <div class="space-y-2 md:col-span-2">
                  <label class="text-xs font-medium text-text-primary">字段备注</label>
                  <el-input
                    v-model="currentField.notes"
                    placeholder="填写关于此配置的补充说明..."
                    @input="markDirty"
                    clearable
                  />
                </div>

                <!-- 来源信息 -->
                <div class="space-y-2 md:col-span-2">
                  <label class="text-xs font-medium text-text-primary block">配置来源</label>
                  <div class="flex items-center gap-2 p-3 bg-bg-panel border border-border/60 rounded-lg">
                    <el-tag size="small" :type="sourceType(currentField.source)" effect="dark" class="!rounded-md">
                      {{ sourceLabel(currentField.source) }}
                    </el-tag>
                    <span class="text-[11px] text-text-muted">
                      {{ getSourceDesc(currentField.source) }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <!-- 空白缺省状态 -->
        <div v-else class="flex-1 flex flex-col items-center justify-center p-8 text-center bg-bg-secondary/10">
          <el-empty :description="selectedVendor ? '请在左侧列表中选择一个字段以开始编辑' : '请先选择一个厂商'" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  TopologyCommandConfigAPI,
  type TopologyVendorCommandItemView,
  type TopologyVendorCommandSaveRequest,
} from "@/services/api";
import { ElMessage } from "element-plus";
import { RefreshRight, Search, Cpu, QuestionFilled, RefreshLeft } from "@element-plus/icons-vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const vendors = ref<string[]>([]);
const selectedVendor = ref("");
const commands = ref<TopologyVendorCommandItemView[]>([]);
const baselineCommands = ref<TopologyVendorCommandItemView[]>([]);

const loadingVendors = ref(false);
const loadingConfig = ref(false);
const saving = ref(false);
const resetting = ref(false);

// 新置：选中字段的 Key
const selectedFieldKey = ref("");
// 新置：快速检索过滤词
const searchQuery = ref("");

// 计算属性：当前选中字段的完整对象
const currentField = computed(() => {
  return commands.value.find(item => item.fieldKey === selectedFieldKey.value);
});

// 计算属性：过滤后的字段列表
const filteredCommands = computed(() => {
  const query = searchQuery.value.trim().toLowerCase();
  if (!query) return commands.value;
  return commands.value.filter((item) => {
    const keyMatch = String(item.fieldKey || "").toLowerCase().includes(query);
    const nameMatch = String(item.displayName || "").toLowerCase().includes(query);
    const descMatch = String(item.description || "").toLowerCase().includes(query);
    return keyMatch || nameMatch || descMatch;
  });
});

const enabledCount = computed(() => commands.value.filter((item) => item.enabled).length);

const invalidCount = computed(
  () =>
    commands.value.filter(
      (item) =>
        (item.enabled && String(item.command || "").trim() === "") ||
        Number(item.timeoutSec || 0) <= 0,
    ).length,
);

const isDirty = computed(
  () => serializeCommands(commands.value) !== serializeCommands(baselineCommands.value),
);

const canSave = computed(
  () =>
    !!selectedVendor.value &&
    !loadingConfig.value &&
    !saving.value &&
    invalidCount.value === 0 &&
    isDirty.value,
);

watch(
  selectedVendor,
  async (vendor) => {
    if (!vendor) {
      commands.value = [];
      baselineCommands.value = [];
      selectedFieldKey.value = "";
      return;
    }
    await loadVendorConfig(vendor);
  },
  { immediate: false },
);

onMounted(async () => {
  await loadVendors();
});

async function loadVendors() {
  loadingVendors.value = true;
  try {
    const values = await TopologyCommandConfigAPI.getSupportedTopologyVendors();
    const rawVendors = (values || [])
      .map((item: string) => String(item || "").trim())
      .filter((item: string) => item.length > 0);
    
    // 排序：huawei 置顶，其他按字母顺序升序排序
    vendors.value = [...rawVendors].sort((a, b) => {
      const aLower = a.toLowerCase();
      const bLower = b.toLowerCase();
      if (aLower === "huawei") return -1;
      if (bLower === "huawei") return 1;
      return a.localeCompare(b);
    });

    if (vendors.value.length === 0) {
      selectedVendor.value = "";
      commands.value = [];
      baselineCommands.value = [];
      selectedFieldKey.value = "";
      ElMessage.warning("当前没有可用的拓扑厂商");
      return;
    }
    if (!vendors.value.includes(selectedVendor.value)) {
      selectedVendor.value = vendors.value[0] || "";
    }
  } catch (err: any) {
    logger.error('加载厂商列表失败', 'TopologyCommandConfig', err);
    ElMessage.error(`加载厂商列表失败: ${err?.message || err}`);
  } finally {
    loadingVendors.value = false;
  }
}

async function loadVendorConfig(vendor: string) {
  loadingConfig.value = true;
  try {
    const set = await TopologyCommandConfigAPI.getVendorCommandConfig(vendor);
    const normalized = normalizeCommands(set?.commands || []);
    commands.value = normalized;
    baselineCommands.value = cloneCommands(normalized);

    // 默认选择第一个字段
    if (normalized.length > 0) {
      if (!normalized.some(item => item.fieldKey === selectedFieldKey.value)) {
        selectedFieldKey.value = normalized[0]?.fieldKey || "";
      }
    } else {
      selectedFieldKey.value = "";
    }
  } catch (err: any) {
    logger.error('加载厂商配置失败', 'TopologyCommandConfig', err);
    commands.value = [];
    baselineCommands.value = [];
    selectedFieldKey.value = "";
    ElMessage.error(`加载 ${vendor} 配置失败: ${err?.message || err}`);
  } finally {
    loadingConfig.value = false;
  }
}

async function refreshCurrentVendor() {
  if (!selectedVendor.value) return;
  await loadVendorConfig(selectedVendor.value);
}

async function saveVendor() {
  if (!selectedVendor.value || invalidCount.value > 0) {
    return;
  }
  saving.value = true;
  try {
    const request: TopologyVendorCommandSaveRequest = {
      vendor: selectedVendor.value,
      commands: commands.value.map((item) => ({
        ...item,
        command: String(item.command || ""),
        timeoutSec: Number(item.timeoutSec || 0),
        enabled: Boolean(item.enabled),
        notes: String(item.notes || ""),
      })),
    };
    const saved = await TopologyCommandConfigAPI.saveVendorCommandConfig(request);
    const normalized = normalizeCommands(saved?.commands || []);
    commands.value = normalized;
    baselineCommands.value = cloneCommands(normalized);
    ElMessage.success(`厂商 ${selectedVendor.value} 配置保存成功`);
  } catch (err: any) {
    logger.error('保存厂商配置失败', 'TopologyCommandConfig', err);
    ElMessage.error(`保存失败: ${err?.message || err}`);
  } finally {
    saving.value = false;
  }
}

async function resetVendor() {
  if (!selectedVendor.value) return;
  resetting.value = true;
  try {
    const reset = await TopologyCommandConfigAPI.resetVendorCommandConfig(selectedVendor.value);
    const normalized = normalizeCommands(reset?.commands || []);
    commands.value = normalized;
    baselineCommands.value = cloneCommands(normalized);
    
    // 重置后确保选中有效字段
    if (normalized.length > 0) {
      if (!normalized.some(item => item.fieldKey === selectedFieldKey.value)) {
        selectedFieldKey.value = normalized[0]?.fieldKey || "";
      }
    } else {
      selectedFieldKey.value = "";
    }

    ElMessage.success(`已重置 ${selectedVendor.value} 为系统默认配置`);
  } catch (err: any) {
    logger.error('重置厂商配置失败', 'TopologyCommandConfig', err);
    ElMessage.error(`重置失败: ${err?.message || err}`);
  } finally {
    resetting.value = false;
  }
}

function resetRow(fieldKey: string) {
  const currentIndex = commands.value.findIndex((item) => item.fieldKey === fieldKey);
  const baseline = baselineCommands.value.find((item) => item.fieldKey === fieldKey);
  if (currentIndex < 0 || !baseline) {
    return;
  }
  const next = cloneCommands(commands.value);
  next[currentIndex] = { ...baseline };
  commands.value = next;
}

function markDirty() {
  commands.value = [...commands.value];
}

// 辅助：判断特定行是否被修改
function isRowDirty(fieldKey: string) {
  const current = commands.value.find(item => item.fieldKey === fieldKey);
  const baseline = baselineCommands.value.find(item => item.fieldKey === fieldKey);
  if (!current || !baseline) return false;
  return (
    current.command !== baseline.command ||
    current.timeoutSec !== baseline.timeoutSec ||
    current.enabled !== baseline.enabled ||
    current.notes !== baseline.notes
  );
}

// 辅助：获取来源的具体解释说明
function getSourceDesc(source: string) {
  switch (source) {
    case "vendor_config":
      return "此命令源于厂商自定义配置文件，具有最高采集执行优先级。";
    case "profile_seed":
      return "这是画像种子中的基础采集指令，常用于特征提取阶段。";
    case "field_default":
      return "这是该字段在全局系统的硬编码兜底指令，未指定厂商规则时生效。";
    default:
      return "此命令来自非标准渠道或处于未初始化的缺省状态。";
  }
}

function normalizeCommands(items: TopologyVendorCommandItemView[]): TopologyVendorCommandItemView[] {
  return (items || []).map((item) => ({
    ...item,
    fieldKey: String(item.fieldKey || "").trim(),
    displayName: String(item.displayName || ""),
    parserBinding: String(item.parserBinding || ""),
    description: String(item.description || ""),
    required: Boolean(item.required),
    command: String(item.command || ""),
    timeoutSec: Number(item.timeoutSec || 0) > 0 ? Number(item.timeoutSec) : 30,
    enabled: Boolean(item.enabled),
    notes: String(item.notes || ""),
    source: String(item.source || ""),
  }));
}

function cloneCommands(items: TopologyVendorCommandItemView[]) {
  return (items || []).map((item) => ({ ...item }));
}

function serializeCommands(items: TopologyVendorCommandItemView[]) {
  return JSON.stringify(
    [...(items || [])]
      .map((item) => ({
        fieldKey: String(item.fieldKey || ""),
        command: String(item.command || "").trim(),
        timeoutSec: Number(item.timeoutSec || 0),
        enabled: Boolean(item.enabled),
        notes: String(item.notes || "").trim(),
      }))
      .sort((a, b) => a.fieldKey.localeCompare(b.fieldKey)),
  );
}

function sourceLabel(source: string) {
  switch (source) {
    case "vendor_config":
      return "厂商配置";
    case "profile_seed":
      return "画像种子";
    case "field_default":
      return "字段默认";
    default:
      return source || "未知";
  }
}

function sourceType(source: string) {
  switch (source) {
    case "vendor_config": return "success";
    case "profile_seed": return "info";
    default: return "info";
  }
}
</script>
