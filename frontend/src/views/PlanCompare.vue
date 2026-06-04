<template>
  <div class="space-y-5 animate-slide-in h-full flex flex-col">
    <div class="flex-shrink-0">
      <h2 class="text-lg font-semibold text-text-primary">规划比对</h2>
      <p class="text-xs text-text-muted mt-1">
        导入固定模板 CSV，执行实际拓扑与规划链路的无向比对。
      </p>
    </div>

    <el-card shadow="never" class="flex-shrink-0">
      <template #header>
        <div class="flex items-center justify-between">
          <div class="text-sm font-medium">1. 导入规划文件</div>
          <el-button size="small" type="primary" plain :icon="Download" @click="downloadTemplate">
            下载导入模板
          </el-button>
        </div>
      </template>
      <div class="flex items-center gap-3">
        <el-input
          v-model="importPath"
          placeholder="输入 CSV 文件绝对路径"
          class="flex-1"
          clearable
        />
        <el-button
          type="primary"
          @click="importPlan"
          :disabled="!importPath"
          :loading="importing"
        >
          导入
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="flex-shrink-0">
      <template #header>
        <div class="text-sm font-medium">2. 选择任务与规划文件</div>
      </template>
      <div class="grid grid-cols-2 gap-4 mb-4">
        <el-select
          v-model="selectedRunId"
          placeholder="选择拓扑运行"
          class="w-full"
          clearable
        >
          <el-option
            v-for="run in topologyRuns"
            :key="run.runId"
            :label="`${run.taskName || run.runId.slice(0, 8)} - ${run.status} (${run.successUnits}/${run.totalUnits})`"
            :value="run.runId"
          />
        </el-select>
        <el-select
          v-model="selectedPlanID"
          placeholder="选择规划文件"
          class="w-full"
          clearable
        >
          <el-option
            v-for="plan in plans"
            :key="plan.id"
            :label="`${plan.fileName} (${plan.totalLinks})`"
            :value="plan.id"
          />
        </el-select>
      </div>
      <div class="flex gap-3">
        <el-button
          type="primary"
          @click="compareNow"
          :disabled="!selectedRunId || !selectedPlanID"
          :loading="comparing"
        >
          执行比对
        </el-button>
        <el-button
          @click="exportReport('json')"
          :disabled="!compareResult.reportId"
        >
          导出 JSON
        </el-button>
        <el-button
          @click="exportReport('csv')"
          :disabled="!compareResult.reportId"
        >
          导出 CSV
        </el-button>
        <el-button
          @click="exportReport('html')"
          :disabled="!compareResult.reportId"
        >
          导出 HTML
        </el-button>
      </div>
    </el-card>

    <div class="grid grid-cols-4 gap-4 flex-shrink-0">
      <el-card shadow="never" :body-style="{ padding: '16px' }">
        <div class="text-xs text-text-muted">规划链路</div>
        <div class="text-xl font-semibold mt-1">{{ compareResult.totalPlanned }}</div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '16px' }">
        <div class="text-xs text-text-muted">实际链路</div>
        <div class="text-xl font-semibold mt-1">{{ compareResult.totalActual }}</div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '16px' }">
        <div class="text-xs text-text-muted">匹配</div>
        <div class="text-xl font-semibold text-success mt-1">{{ compareResult.matched }}</div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '16px' }">
        <div class="text-xs text-text-muted">差异总数</div>
        <div class="text-xl font-semibold text-error mt-1">{{ totalDiff }}</div>
      </el-card>
    </div>

    <el-card shadow="never" class="flex-1 min-h-0 flex flex-col" :body-style="{ padding: '0', flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }">
      <template #header>
        <div class="text-sm font-medium">差异明细</div>
      </template>
      <el-table
        :data="allDiffItems"
        style="width: 100%; height: 100%"
        height="100%"
        empty-text="暂无差异"
      >
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="diffTagType(row.diffType)"
            >
              {{ row.diffType }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="A 端" min-width="180">
          <template #default="{ row }">
            <span class="font-mono text-text-primary">{{ row.aDeviceName }} {{ row.aIf }}</span>
          </template>
        </el-table-column>
        <el-table-column label="B 端" min-width="180">
          <template #default="{ row }">
            <span class="font-mono text-text-primary">{{ row.bDeviceName }} {{ row.bIf }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="200" />
      </el-table>
    </el-card>

    <div v-if="lastExportPath" class="text-xs text-text-muted flex-shrink-0 mt-2">
      报告已导出: {{ lastExportPath }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Download } from '@element-plus/icons-vue';
import {
  PlanCompareAPI,
  type CompareResult,
  type DiffItem,
  type PlanUploadView,
} from "../services/api";
import { useTaskexecStore } from "../stores/taskexecStore";

const taskexecStore = useTaskexecStore();

const importPath = ref("");
const importing = ref(false);
const comparing = ref(false);
const lastExportPath = ref("");

const selectedPlanID = ref("");
const selectedRunId = ref("");
const plans = ref<PlanUploadView[]>([]);

// 从统一运行时获取拓扑运行记录
const topologyRuns = computed(() => {
  return taskexecStore.runHistory.filter(
    (run) => run.runKind === "topology" && run.status !== "running"
  );
});

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

function diffTagType(diffType: string) {
  if (diffType.includes("missing") || diffType.includes("mismatch")) {
    return "danger";
  }
  if (diffType.includes("unexpected") || diffType.includes("one_side")) {
    return "warning";
  }
  return "info";
}

async function loadBaseData() {
  const [planList] = await Promise.all([
    PlanCompareAPI.listPlanFiles(50),
    taskexecStore.loadRunHistory(50), // 加载统一运行时历史
  ]);
  plans.value = planList || [];
}

async function importPlan() {
  if (!importPath.value || importing.value) return;
  importing.value = true;
  try {
    const result = await PlanCompareAPI.importPlanCSV(importPath.value);
    if (result?.planFileId) {
      selectedPlanID.value = result.planFileId;
    }
    await loadBaseData();
  } finally {
    importing.value = false;
  }
}

async function downloadTemplate() {
  const header = ['源设备IP', '源接口', '目标设备IP', '目标接口'];
  const csvContent = "\uFEFF" + header.join(',') + '\n';
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const fileName = '规划导入模板.csv';

  try {
    if ('showSaveFilePicker' in window) {
      const handle = await (window as any).showSaveFilePicker({
        suggestedName: fileName,
        types: [{
          description: 'CSV 文件',
          accept: { 'text/csv': ['.csv'] },
        }],
      });
      const writable = await handle.createWritable();
      await writable.write(blob);
      await writable.close();
    } else {
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.setAttribute('href', url);
      link.setAttribute('download', fileName);
      link.style.visibility = 'hidden';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    }
  } catch (err: any) {
    if (err.name !== 'AbortError') {
      console.error('下载模板失败:', err);
    }
  }
}

async function compareNow() {
  if (!selectedRunId.value || !selectedPlanID.value || comparing.value) return;
  comparing.value = true;
  try {
    const result = await PlanCompareAPI.compare(selectedRunId.value, selectedPlanID.value);
    if (result) {
      compareResult.value = result;
    }
  } finally {
    comparing.value = false;
  }
}

async function exportReport(format: "json" | "csv" | "html") {
  if (!compareResult.value.reportId) return;
  lastExportPath.value = await PlanCompareAPI.exportDiffReport(compareResult.value.reportId, format);
}

onMounted(() => {
  void loadBaseData();
});
</script>
