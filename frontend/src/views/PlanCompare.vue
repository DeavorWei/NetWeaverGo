<template>
  <div class="space-y-5 animate-slide-in">
    <div>
      <h2 class="text-lg font-semibold text-text-primary">规划比对</h2>
      <p class="text-xs text-text-muted mt-1">
        导入固定模板 Excel，执行实际拓扑与规划链路的无向比对。
      </p>
    </div>

    <div class="bg-bg-card border border-border rounded-xl p-4 space-y-3">
      <div class="text-sm font-medium text-text-primary">1. 导入规划文件</div>
      <div class="flex items-center gap-2">
        <input
          v-model="importPath"
          placeholder="输入 Excel 文件绝对路径"
          class="flex-1 px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        />
        <button
          @click="importPlan"
          :disabled="!importPath || importing"
          class="px-4 py-2 rounded-lg text-sm font-semibold transition-all"
          :class="
            !importPath || importing
              ? 'bg-bg-panel border border-border text-text-muted cursor-not-allowed'
              : 'bg-accent text-white border border-accent/40 hover:bg-accent-glow'
          "
        >
          {{ importing ? "导入中..." : "导入" }}
        </button>
      </div>
    </div>

    <div class="bg-bg-card border border-border rounded-xl p-4 space-y-3">
      <div class="text-sm font-medium text-text-primary">2. 选择任务与规划文件</div>
      <div class="grid grid-cols-2 gap-3">
        <select
          v-model="selectedTaskID"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="">选择发现任务</option>
          <option v-for="task in tasks" :key="task.id" :value="task.id">
            {{ task.name || task.id }}
          </option>
        </select>
        <select
          v-model="selectedPlanID"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="">选择规划文件</option>
          <option v-for="plan in plans" :key="plan.id" :value="plan.id">
            {{ plan.fileName }} ({{ plan.totalLinks }})
          </option>
        </select>
      </div>
      <div class="flex gap-2">
        <button
          @click="compareNow"
          :disabled="!selectedTaskID || !selectedPlanID || comparing"
          class="px-4 py-2 rounded-lg text-sm font-semibold transition-all"
          :class="
            !selectedTaskID || !selectedPlanID || comparing
              ? 'bg-bg-panel border border-border text-text-muted cursor-not-allowed'
              : 'bg-accent text-white border border-accent/40 hover:bg-accent-glow'
          "
        >
          {{ comparing ? "比对中..." : "执行比对" }}
        </button>
        <button
          @click="exportReport('json')"
          :disabled="!compareResult.reportId"
          class="px-4 py-2 rounded-lg text-sm border border-border text-text-secondary hover:text-text-primary"
        >
          导出 JSON
        </button>
        <button
          @click="exportReport('csv')"
          :disabled="!compareResult.reportId"
          class="px-4 py-2 rounded-lg text-sm border border-border text-text-secondary hover:text-text-primary"
        >
          导出 CSV
        </button>
        <button
          @click="exportReport('excel')"
          :disabled="!compareResult.reportId"
          class="px-4 py-2 rounded-lg text-sm border border-border text-text-secondary hover:text-text-primary"
        >
          导出 Excel
        </button>
        <button
          @click="exportReport('html')"
          :disabled="!compareResult.reportId"
          class="px-4 py-2 rounded-lg text-sm border border-border text-text-secondary hover:text-text-primary"
        >
          导出 HTML
        </button>
      </div>
    </div>

    <div class="grid grid-cols-4 gap-3">
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">规划链路</div>
        <div class="text-xl font-semibold text-text-primary mt-1">{{ compareResult.totalPlanned }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">实际链路</div>
        <div class="text-xl font-semibold text-text-primary mt-1">{{ compareResult.totalActual }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">匹配</div>
        <div class="text-xl font-semibold text-success mt-1">{{ compareResult.matched }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">差异总数</div>
        <div class="text-xl font-semibold text-error mt-1">{{ totalDiff }}</div>
      </div>
    </div>

    <div class="bg-bg-card border border-border rounded-xl overflow-hidden">
      <div class="px-4 py-3 border-b border-border bg-bg-panel text-sm font-medium text-text-primary">
        差异明细
      </div>
      <div class="max-h-[48vh] overflow-auto scrollbar-custom">
        <table class="w-full text-sm">
          <thead class="bg-bg-panel text-text-secondary text-xs sticky top-0">
            <tr>
              <th class="text-left px-3 py-2">类型</th>
              <th class="text-left px-3 py-2">A 端</th>
              <th class="text-left px-3 py-2">B 端</th>
              <th class="text-left px-3 py-2">原因</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="item in allDiffItems" :key="`${item.diffType}-${item.id}`">
              <td class="px-3 py-2">
                <span class="px-2 py-0.5 rounded text-xs" :class="diffClass(item.diffType)">
                  {{ item.diffType }}
                </span>
              </td>
              <td class="px-3 py-2 font-mono text-text-primary">
                {{ item.aDeviceName }} {{ item.aIf }}
              </td>
              <td class="px-3 py-2 font-mono text-text-primary">
                {{ item.bDeviceName }} {{ item.bIf }}
              </td>
              <td class="px-3 py-2 text-text-muted">{{ item.reason }}</td>
            </tr>
            <tr v-if="allDiffItems.length === 0">
              <td colspan="4" class="px-3 py-8 text-center text-text-muted">暂无差异</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="lastExportPath" class="text-xs text-text-muted">
      报告已导出: {{ lastExportPath }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  DiscoveryAPI,
  PlanCompareAPI,
  type CompareResult,
  type DiffItem,
  type DiscoveryTaskView,
  type PlanUploadView,
} from "../services/api";

const importPath = ref("");
const importing = ref(false);
const comparing = ref(false);
const lastExportPath = ref("");

const selectedTaskID = ref("");
const selectedPlanID = ref("");
const tasks = ref<DiscoveryTaskView[]>([]);
const plans = ref<PlanUploadView[]>([]);

const compareResult = ref<CompareResult>({
  reportId: "",
  totalPlanned: 0,
  totalActual: 0,
  matched: 0,
  missingLinks: [],
  unexpectedLinks: [],
  inconsistentItems: [],
});

const allDiffItems = computed<DiffItem[]>(() => {
  return [
    ...(compareResult.value.missingLinks || []),
    ...(compareResult.value.unexpectedLinks || []),
    ...(compareResult.value.inconsistentItems || []),
  ];
});

const totalDiff = computed(() => allDiffItems.value.length);

function diffClass(diffType: string) {
  if (diffType.includes("missing") || diffType.includes("mismatch")) {
    return "bg-error/20 text-error";
  }
  if (diffType.includes("unexpected") || diffType.includes("one_side")) {
    return "bg-warning/20 text-warning";
  }
  return "bg-text-muted/20 text-text-muted";
}

async function loadBaseData() {
  const [taskList, planList] = await Promise.all([
    DiscoveryAPI.listDiscoveryTasks(50),
    PlanCompareAPI.listPlanFiles(50),
  ]);
  tasks.value = taskList || [];
  plans.value = planList || [];
}

async function importPlan() {
  if (!importPath.value || importing.value) return;
  importing.value = true;
  try {
    const result = await PlanCompareAPI.importPlanExcel(importPath.value);
    if (result?.planFileId) {
      selectedPlanID.value = result.planFileId;
    }
    await loadBaseData();
  } finally {
    importing.value = false;
  }
}

async function compareNow() {
  if (!selectedTaskID.value || !selectedPlanID.value || comparing.value) return;
  comparing.value = true;
  try {
    const result = await PlanCompareAPI.compare(selectedTaskID.value, selectedPlanID.value);
    if (result) {
      compareResult.value = result;
    }
  } finally {
    comparing.value = false;
  }
}

async function exportReport(format: "json" | "csv" | "excel" | "html") {
  if (!compareResult.value.reportId) return;
  lastExportPath.value = await PlanCompareAPI.exportDiffReport(compareResult.value.reportId, format);
}

onMounted(() => {
  void loadBaseData();
});
</script>
