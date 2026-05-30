<template>
  <div class="space-y-5 animate-slide-in h-full flex flex-col">
    <div class="flex items-center justify-between gap-3 flex-shrink-0">
      <div>
        <h2 class="text-lg font-semibold text-text-primary">拓扑命令配置中心</h2>
        <p class="text-xs text-text-muted mt-1">
          统一维护厂商默认采集命令，支持字段级编辑、启停控制和超时管理。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <el-button
          :icon="RefreshRight"
          :disabled="loadingConfig"
          @click="refreshCurrentVendor"
        >
          刷新
        </el-button>
      </div>
    </div>

    <el-card shadow="never" :body-style="{ padding: '16px' }" class="flex-shrink-0">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div class="space-y-2">
          <label class="text-sm font-medium text-text-primary">厂商</label>
          <el-select
            v-model="selectedVendor"
            placeholder="请选择厂商"
            class="w-full"
            :disabled="loadingVendors || loadingConfig"
          >
            <el-option v-for="vendor in vendors" :key="vendor" :label="vendor" :value="vendor" />
          </el-select>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium text-text-primary">字段总数</label>
          <div class="px-3 py-1.5 rounded-lg bg-bg-panel border border-border text-sm text-text-primary">
            {{ commands.length }}
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium text-text-primary">启用字段数</label>
          <div class="px-3 py-1.5 rounded-lg bg-bg-panel border border-border text-sm text-text-primary">
            {{ enabledCount }}
          </div>
        </div>
      </div>

      <el-alert
        v-if="invalidCount > 0"
        type="error"
        :closable="false"
        class="mt-4"
        show-icon
        :title="`存在 ${invalidCount} 条无效配置（启用但命令为空，或超时时间小于等于 0），请修正后再保存。`"
      />

      <div class="flex items-center justify-end gap-2 mt-4">
        <el-button
          type="warning"
          plain
          :disabled="!selectedVendor || loadingConfig || resetting"
          @click="resetVendor"
        >
          {{ resetting ? "重置中..." : "重置为系统默认" }}
        </el-button>
        <el-button
          type="primary"
          :disabled="!canSave"
          :loading="saving"
          @click="saveVendor"
        >
          保存配置
        </el-button>
      </div>
    </el-card>

    <div class="flex-1 min-h-0 bg-bg-card border border-border rounded-xl overflow-hidden relative">
      <el-table
        :data="commands"
        style="width: 100%; height: 100%"
        height="100%"
        v-loading="loadingConfig"
        empty-text="暂无可编辑字段"
      >
        <el-table-column label="字段" min-width="160">
          <template #default="{ row }">
            <div class="font-medium text-text-primary">{{ row.displayName || row.fieldKey }}</div>
            <div class="text-xs text-text-muted font-mono mt-1">{{ row.fieldKey }}</div>
            <div class="text-[11px] text-text-muted mt-1">{{ row.description || "-" }}</div>
            <el-tag
              v-if="row.required"
              type="warning"
              size="small"
              class="mt-1"
            >
              必填字段
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="命令" min-width="320">
          <template #default="{ row }">
            <el-input
              v-model="row.command"
              type="textarea"
              :rows="3"
              placeholder="请输入命令"
              @input="markDirty"
              class="font-mono text-xs"
            />
            <div class="text-[11px] text-text-muted mt-1">解析绑定: {{ row.parserBinding || "-" }}</div>
          </template>
        </el-table-column>

        <el-table-column label="超时(秒)" width="120">
          <template #default="{ row }">
            <el-input-number
              v-model="row.timeoutSec"
              :min="1"
              size="small"
              class="w-full"
              controls-position="right"
              @change="markDirty"
            />
          </template>
        </el-table-column>

        <el-table-column label="启用" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              inline-prompt
              active-text="启"
              inactive-text="停"
              @change="markDirty"
            />
          </template>
        </el-table-column>

        <el-table-column label="备注" min-width="160">
          <template #default="{ row }">
            <el-input
              v-model="row.notes"
              placeholder="可选备注"
              size="small"
              @input="markDirty"
            />
          </template>
        </el-table-column>

        <el-table-column label="来源" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="sourceType(row.source)">
              {{ sourceLabel(row.source) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              size="small"
              @click="resetRow(row.fieldKey)"
            >
              撤销
            </el-button>
          </template>
        </el-table-column>
      </el-table>
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
import { RefreshRight } from "@element-plus/icons-vue";
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
    vendors.value = (values || [])
      .map((item: string) => String(item || "").trim())
      .filter((item: string) => item.length > 0);
    if (vendors.value.length === 0) {
      selectedVendor.value = "";
      commands.value = [];
      baselineCommands.value = [];
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
  } catch (err: any) {
    logger.error('加载厂商配置失败', 'TopologyCommandConfig', err);
    commands.value = [];
    baselineCommands.value = [];
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
