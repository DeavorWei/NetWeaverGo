<template>
  <div class="space-y-5 animate-slide-in">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-text-primary">拓扑命令配置中心</h2>
        <p class="text-xs text-text-muted mt-1">
          统一维护厂商默认采集命令，支持字段级编辑、启停控制和超时管理。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="px-4 py-2 rounded-lg text-sm font-medium border border-border text-text-secondary hover:text-text-primary disabled:opacity-50"
          :disabled="loadingConfig"
          @click="refreshCurrentVendor"
        >
          刷新
        </button>
      </div>
    </div>

    <div class="bg-bg-card border border-border rounded-xl p-4 space-y-4">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <div class="space-y-2">
          <label class="text-sm font-medium text-text-primary">厂商</label>
          <select
            v-model="selectedVendor"
            class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
            :disabled="loadingVendors || loadingConfig"
          >
            <option value="" disabled>请选择厂商</option>
            <option v-for="vendor in vendors" :key="vendor" :value="vendor">
              {{ vendor }}
            </option>
          </select>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium text-text-primary">字段总数</label>
          <div class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary">
            {{ commands.length }}
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium text-text-primary">启用字段数</label>
          <div class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary">
            {{ enabledCount }}
          </div>
        </div>
      </div>

      <div
        v-if="invalidCount > 0"
        class="rounded-lg border border-error/30 bg-error/10 px-3 py-2 text-xs text-error"
      >
        存在 {{ invalidCount }} 条无效配置（启用但命令为空，或超时时间小于等于 0），请修正后再保存。
      </div>

      <div class="flex items-center justify-end gap-2">
        <button
          class="px-4 py-2 rounded-lg text-sm font-medium border border-warning/40 text-warning hover:bg-warning/10 disabled:opacity-50"
          :disabled="!selectedVendor || loadingConfig || resetting"
          @click="resetVendor"
        >
          {{ resetting ? "重置中..." : "重置为系统默认" }}
        </button>
        <button
          class="px-4 py-2 rounded-lg text-sm font-semibold bg-accent text-white hover:bg-accent-glow disabled:opacity-50"
          :disabled="!canSave"
          @click="saveVendor"
        >
          {{ saving ? "保存中..." : "保存配置" }}
        </button>
      </div>
    </div>

    <div class="bg-bg-card border border-border rounded-xl overflow-hidden">
      <div class="max-h-[62vh] overflow-auto scrollbar-custom">
        <table class="w-full text-sm">
          <thead class="bg-bg-panel text-text-secondary text-xs sticky top-0 z-10">
            <tr>
              <th class="text-left px-3 py-2 min-w-[140px]">字段</th>
              <th class="text-left px-3 py-2 min-w-[320px]">命令</th>
              <th class="text-left px-3 py-2 min-w-[120px]">超时(秒)</th>
              <th class="text-left px-3 py-2 min-w-[80px]">启用</th>
              <th class="text-left px-3 py-2 min-w-[160px]">备注</th>
              <th class="text-left px-3 py-2 min-w-[130px]">来源</th>
              <th class="text-left px-3 py-2 min-w-[80px]">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-if="loadingConfig">
              <td colspan="7" class="px-4 py-8 text-center text-text-muted">加载配置中...</td>
            </tr>
            <tr v-else-if="commands.length === 0">
              <td colspan="7" class="px-4 py-8 text-center text-text-muted">暂无可编辑字段</td>
            </tr>
            <tr v-for="row in commands" :key="row.fieldKey" class="hover:bg-bg-hover/30 align-top">
              <td class="px-3 py-3">
                <div class="font-medium text-text-primary">{{ row.displayName || row.fieldKey }}</div>
                <div class="text-xs text-text-muted font-mono mt-1">{{ row.fieldKey }}</div>
                <div class="text-[11px] text-text-muted mt-1">{{ row.description || "-" }}</div>
                <span
                  v-if="row.required"
                  class="inline-flex mt-2 px-2 py-0.5 rounded bg-warning/15 text-warning text-[11px]"
                >
                  必填字段
                </span>
              </td>

              <td class="px-3 py-3">
                <textarea
                  v-model="row.command"
                  rows="3"
                  class="w-full px-3 py-2 rounded-lg bg-terminal-bg text-terminal-text border border-border font-mono text-xs resize-y focus:outline-none focus:border-accent/50"
                  placeholder="请输入命令"
                  @input="markDirty"
                />
                <div class="text-[11px] text-text-muted mt-1">解析绑定: {{ row.parserBinding || "-" }}</div>
              </td>

              <td class="px-3 py-3">
                <input
                  v-model.number="row.timeoutSec"
                  type="number"
                  min="1"
                  class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
                  @input="markDirty"
                />
              </td>

              <td class="px-3 py-3">
                <label class="inline-flex items-center gap-2 cursor-pointer select-none">
                  <input
                    v-model="row.enabled"
                    type="checkbox"
                    class="w-4 h-4 rounded border-border"
                    @change="markDirty"
                  />
                  <span class="text-xs text-text-secondary">{{ row.enabled ? "启用" : "停用" }}</span>
                </label>
              </td>

              <td class="px-3 py-3">
                <input
                  v-model="row.notes"
                  type="text"
                  class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-xs text-text-primary"
                  placeholder="可选备注"
                  @input="markDirty"
                />
              </td>

              <td class="px-3 py-3">
                <span class="inline-flex px-2 py-1 rounded text-[11px] border" :class="sourceClass(row.source)">
                  {{ sourceLabel(row.source) }}
                </span>
              </td>

              <td class="px-3 py-3">
                <button
                  class="text-xs px-2 py-1 rounded border border-border text-text-secondary hover:text-text-primary"
                  @click="resetRow(row.fieldKey)"
                >
                  撤销
                </button>
              </td>
            </tr>
          </tbody>
        </table>
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
import { useToast } from "@/utils/useToast";

const toast = useToast();

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
      toast.warning("当前没有可用的拓扑厂商");
      return;
    }
    if (!vendors.value.includes(selectedVendor.value)) {
      selectedVendor.value = vendors.value[0] || "";
    }
  } catch (err: any) {
    console.error("加载厂商列表失败", err);
    toast.error(`加载厂商列表失败: ${err?.message || err}`);
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
    console.error("加载厂商配置失败", err);
    commands.value = [];
    baselineCommands.value = [];
    toast.error(`加载 ${vendor} 配置失败: ${err?.message || err}`);
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
    toast.success(`厂商 ${selectedVendor.value} 配置保存成功`);
  } catch (err: any) {
    console.error("保存厂商配置失败", err);
    toast.error(`保存失败: ${err?.message || err}`);
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
    toast.success(`已重置 ${selectedVendor.value} 为系统默认配置`);
  } catch (err: any) {
    console.error("重置厂商配置失败", err);
    toast.error(`重置失败: ${err?.message || err}`);
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

function sourceClass(source: string) {
  switch (source) {
    case "vendor_config":
      return "border-accent/30 text-accent bg-accent/10";
    case "profile_seed":
      return "border-info/30 text-info bg-info/10";
    default:
      return "border-border text-text-muted bg-bg-panel";
  }
}
</script>
