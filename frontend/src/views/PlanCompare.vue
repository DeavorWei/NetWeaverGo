<template>
  <div class="h-full flex gap-5 animate-slide-in">
    <!-- 左侧面板 -->
    <div class="w-[380px] flex-shrink-0 flex flex-col gap-4 overflow-y-auto pr-1">
      <div class="mb-2">
        <h2 class="text-xl font-semibold text-text-primary">规划比对</h2>
        <p class="text-sm text-text-muted mt-1">导入固定模板 CSV，执行实际拓扑与规划链路的比对。</p>
      </div>

      <!-- 1. 数据准备 -->
      <el-card shadow="never" class="border-border">
        <template #header>
          <div class="flex items-center justify-between">
            <div class="text-sm font-medium flex items-center gap-2">
              <el-icon><Document /></el-icon>
              1. 数据准备
            </div>
          </div>
        </template>
        <div class="flex flex-col gap-3">
          <div class="text-xs text-text-muted">请先下载模板，填写后一键导入系统：</div>
          <div class="flex items-center gap-2">
            <el-button @click="downloadTemplate" :icon="Download" class="flex-1 !ml-0">
              下载模板
            </el-button>
            <el-button type="primary" @click="selectAndImportCSV" :icon="FolderOpened" :loading="importing" class="flex-1 !ml-0">
              导入数据
            </el-button>
            <el-button @click="showManageDialog = true" :icon="Setting" class="flex-1 !ml-0">
              管理文件
            </el-button>
          </div>
        </div>
      </el-card>

      <!-- 2. 比对参数 -->
      <el-card shadow="never" class="border-border">
        <template #header>
          <div class="text-sm font-medium flex items-center gap-2">
            <el-icon><Filter /></el-icon>
            2. 比对参数
          </div>
        </template>
        <div class="flex flex-col gap-4">
          <div>
            <div class="text-xs text-text-muted mb-1">选择拓扑运行任务（实际数据）：</div>
            <el-select
              v-model="selectedRunId"
              placeholder="请选择拓扑运行记录"
              class="w-full"
              clearable
            >
              <el-option
                v-for="run in topologyRuns"
                :key="run.runId"
                :label="`[${formatTime(run.startedAt)}] ${run.taskName || run.runId.slice(0, 8)} - ${run.status} (${run.successUnits}/${run.totalUnits})`"
                :value="run.runId"
              />
            </el-select>
          </div>
          <div>
            <div class="text-xs text-text-muted mb-1">选择规划文件（期望数据）：</div>
            <el-select
              v-model="selectedPlanID"
              placeholder="请选择规划文件"
              class="w-full"
              clearable
            >
              <el-option
                v-for="plan in plans"
                :key="plan.id"
                :label="`[${formatTime(plan.importedAt)}] ${formatFileName(plan.fileName)} (${plan.totalLinks})`"
                :value="plan.id"
              />
            </el-select>
          </div>
        </div>
      </el-card>

      <!-- 3. 执行与导出 -->
      <el-card shadow="never" class="border-border bg-fill-lighter">
        <div class="flex flex-col gap-4">
          <el-button
            type="primary"
            size="large"
            class="w-full shadow-md font-medium"
            :icon="DataLine"
            @click="compareNow"
            :disabled="!selectedRunId || !selectedPlanID"
            :loading="comparing"
          >
            开始比对
          </el-button>

          <div class="flex items-center justify-between gap-3">
            <span class="text-xs text-text-muted">比对完成后可导出报告</span>
            <el-dropdown trigger="click" @command="exportReport" :disabled="!compareResult.reportId">
              <el-button :disabled="!compareResult.reportId" plain>
                导出报告<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="html">导出为 HTML</el-dropdown-item>
                  <el-dropdown-item command="csv">导出为 CSV</el-dropdown-item>
                  <el-dropdown-item command="json">导出为 JSON</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
          <div v-if="lastExportPath" class="text-xs text-success break-all">
            已导出: {{ lastExportPath }}
          </div>
        </div>
      </el-card>
    </div>

    <!-- 右侧结果展示区 -->
    <div class="flex-1 min-w-0 flex flex-col gap-4">
      <!-- 统计指标 -->
      <div class="grid grid-cols-4 gap-4 flex-shrink-0">
        <el-card shadow="never" :body-style="{ padding: '20px' }" class="border-border relative overflow-hidden">
          <div class="absolute right-[-10px] top-[-10px] opacity-5 text-6xl"><el-icon><Document /></el-icon></div>
          <div class="text-sm text-text-muted mb-2">规划链路</div>
          <div class="text-3xl font-bold text-text-primary">{{ compareResult.totalPlanned }}</div>
        </el-card>
        <el-card shadow="never" :body-style="{ padding: '20px' }" class="border-border relative overflow-hidden">
          <div class="absolute right-[-10px] top-[-10px] opacity-5 text-6xl"><el-icon><Connection /></el-icon></div>
          <div class="text-sm text-text-muted mb-2">实际链路</div>
          <div class="text-3xl font-bold text-text-primary">{{ compareResult.totalActual }}</div>
        </el-card>
        <el-card shadow="never" :body-style="{ padding: '20px' }" class="border-border border-l-4 border-l-success relative overflow-hidden">
          <div class="absolute right-[-10px] top-[-10px] opacity-5 text-6xl text-success"><el-icon><Check /></el-icon></div>
          <div class="text-sm text-text-muted mb-2">完全匹配</div>
          <div class="text-3xl font-bold text-success">{{ compareResult.matched }}</div>
        </el-card>
        <el-card shadow="never" :body-style="{ padding: '20px' }" class="border-border border-l-4 border-l-error relative overflow-hidden">
          <div class="absolute right-[-10px] top-[-10px] opacity-5 text-6xl text-error"><el-icon><Warning /></el-icon></div>
          <div class="text-sm text-text-muted mb-2">差异总数</div>
          <div class="text-3xl font-bold text-error">{{ totalDiff }}</div>
        </el-card>
      </div>

      <!-- 差异明细表格 -->
      <el-card shadow="never" class="flex-1 min-h-0 flex flex-col border-border" :body-style="{ padding: '0', flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }">
        <template #header>
          <div class="text-sm font-medium flex items-center gap-2">
            <el-icon><List /></el-icon>
            差异明细
          </div>
        </template>
        <div v-if="allDiffItems.length === 0 && !compareResult.reportId" class="h-full flex flex-col items-center justify-center text-text-muted p-10">
          <el-icon class="text-6xl mb-4 opacity-20"><DataAnalysis /></el-icon>
          <p>请在左侧配置参数并执行比对，结果将显示在这里</p>
        </div>
        <el-table
          v-else
          :data="allDiffItems"
          style="width: 100%; height: 100%"
          height="100%"
          empty-text="完美匹配，暂无差异 🎉"
        >
          <el-table-column label="差异类型" width="160">
            <template #default="{ row }">
              <el-tag
                size="small"
                effect="light"
                :type="diffTagType(row.diffType)"
              >
                {{ row.diffType }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="源设备IP" min-width="140">
            <template #default="{ row }">
              <div class="font-mono text-sm text-text-primary bg-fill-light px-2 py-1 rounded inline-block break-all whitespace-normal">
                {{ formatDeviceIP(row.aMgmtIp, row.aDeviceName) }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="源接口" min-width="140">
            <template #default="{ row }">
              <div class="font-mono text-sm text-text-primary bg-fill-light px-2 py-1 rounded inline-block break-all whitespace-normal">
                {{ row.aIf }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="目标设备IP" min-width="140">
            <template #default="{ row }">
              <div class="font-mono text-sm text-text-primary bg-fill-light px-2 py-1 rounded inline-block break-all whitespace-normal">
                {{ formatDeviceIP(row.bMgmtIp, row.bDeviceName) }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="目标接口" min-width="140">
            <template #default="{ row }">
              <div class="font-mono text-sm text-text-primary bg-fill-light px-2 py-1 rounded inline-block break-all whitespace-normal">
                {{ row.bIf }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="原因说明" min-width="250">
            <template #default="{ row }">
              <div class="whitespace-pre-wrap">{{ row.reason }}</div>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>

    <!-- 管理弹窗 -->
    <el-dialog v-model="showManageDialog" title="规划文件管理" width="800px">
      <div class="mb-4">
        <el-button type="danger" :disabled="selectedFiles.length === 0" @click="batchDelete">批量删除</el-button>
      </div>
      <el-table
        :data="plans"
        style="width: 100%"
        @selection-change="handleSelectionChange"
        max-height="400"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="fileName" label="文件名" min-width="200" />
        <el-table-column prop="totalLinks" label="链路数" width="100" />
        <el-table-column prop="importedAt" label="导入时间" width="180">
          <template #default="{ row }">
            {{ new Date(row.importedAt).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="previewPlan(row)">预览</el-button>
            <el-button link type="danger" size="small" @click="deletePlan(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 预览弹窗 -->
    <el-dialog v-model="showPreviewDialog" :title="`预览: ${previewTitle}`" width="900px">
      <el-table :data="previewData" style="width: 100%" height="400" v-loading="previewLoading">
        <el-table-column prop="aMgmtIp" label="源设备IP" min-width="140" />
        <el-table-column prop="aIf" label="源接口" min-width="120" />
        <el-table-column prop="bMgmtIp" label="目标设备IP" min-width="140" />
        <el-table-column prop="bIf" label="目标接口" min-width="120" />
        <el-table-column prop="linkType" label="类型" width="100" />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Download, Setting, Document, Filter, DataLine, ArrowDown, Connection, Check, Warning, List, DataAnalysis, FolderOpened } from '@element-plus/icons-vue';
import { Dialogs } from '@wailsio/runtime';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  PlanCompareAPI,
  type CompareResult,
  type DiffItem,
  type PlanUploadView,
} from "../services/api";
import { useTaskexecStore } from "../stores/taskexecStore";

const taskexecStore = useTaskexecStore();

function formatTime(timeStr?: string) {
  if (!timeStr) return "";
  const d = new Date(timeStr);
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function formatFileName(fileName: string) {
  if (!fileName) return "";
  return fileName.replace(/^\d{8}_\d{6}_/, '');
}

const importPath = ref("");
const importing = ref(false);
const comparing = ref(false);
const lastExportPath = ref("");

const selectedPlanID = ref("");
const selectedRunId = ref("");
const plans = ref<PlanUploadView[]>([]);

const showManageDialog = ref(false);
const showPreviewDialog = ref(false);
const selectedFiles = ref<PlanUploadView[]>([]);
const previewData = ref<any[]>([]);
const previewTitle = ref("");
const previewLoading = ref(false);

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

function formatDeviceIP(ip?: string, name?: string) {
  let val = ip || name || '';
  // 移除结尾的空格和多余的 .
  val = val.replace(/[\s.]+$/, '');
  // 移除可能存在的设备类型前缀 (如 endpoint:192.168.1.1 -> 192.168.1.1)
  const match = val.match(/^[a-zA-Z]+:(.*)$/);
  if (match) {
    val = match[1] || "";
  }
  return val;
}

async function loadBaseData() {
  const [planList] = await Promise.all([
    PlanCompareAPI.listPlanFiles(50),
    taskexecStore.loadRunHistory(50), // 加载统一运行时历史
  ]);
  plans.value = planList || [];
}

async function selectAndImportCSV() {
  if (importing.value) return;
  try {
    const file = await Dialogs.OpenFile({
      Title: "选择规划文件 (CSV)",
      Filters: [{ DisplayName: "CSV Files (*.csv)", Pattern: "*.csv" }],
      CanChooseFiles: true,
      CanChooseDirectories: false,
      AllowsMultipleSelection: false,
    });
    if (file) {
      importPath.value = Array.isArray(file) ? file[0] : file;
      if (importPath.value) {
        await importPlan();
      }
    }
  } catch (err: any) {
    console.error('导入数据异常:', err);
    ElMessage.error(err.message || String(err));
  }
}

async function importPlan() {
  if (!importPath.value || importing.value) return;
  importing.value = true;
  try {
    const result = await PlanCompareAPI.importPlanCSV(importPath.value);
    if (result?.planFileId) {
      selectedPlanID.value = result.planFileId;
      ElMessage.success(`导入成功，共 ${result.totalLinks} 条链路`);
    }
    await loadBaseData();
  } catch (error: any) {
    console.error('导入规划文件失败:', error);
    ElMessage.error(error?.message || '导入失败，请检查文件路径和格式');
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
      ElMessage.success(`比对完成，匹配 ${result.matched} 条链路`);
    }
  } catch (error: any) {
    console.error('比对失败:', error);
    ElMessage.error(error?.message || '比对失败，请重试');
  } finally {
    comparing.value = false;
  }
}

async function exportReport(format: "json" | "csv" | "html") {
  if (!compareResult.value.reportId) return;
  
  const defaultExt = format === 'html' ? 'html' : format;
  try {
    const savePath = await Dialogs.SaveFile({
      Title: "导出报告",
      Filename: `规划比对差异_${compareResult.value.reportId.slice(5, 13)}.${defaultExt}`,
      Filters: [{ DisplayName: `${format.toUpperCase()} 文件`, Pattern: `*.${defaultExt}` }],
    });
    
    if (!savePath) return; // 用户取消
    
    lastExportPath.value = await PlanCompareAPI.exportDiffReport(compareResult.value.reportId, format, savePath);
    ElMessage.success("报告导出成功");
  } catch (err: any) {
    const errMsg = err.message || String(err);
    if (errMsg === 'cancel' || errMsg.includes('cancelled by user')) {
      ElMessage.info("用户已取消");
    } else {
      console.error('导出报告失败:', err);
      ElMessage.error(errMsg || "导出报告失败");
    }
  }
}

function handleSelectionChange(val: PlanUploadView[]) {
  selectedFiles.value = val;
}

async function batchDelete() {
  if (selectedFiles.value.length === 0) return;
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${selectedFiles.value.length} 个文件吗？`, '提示', { type: 'warning' });
    const ids = selectedFiles.value.map(f => f.id);
    await PlanCompareAPI.deletePlanFiles(ids);
    ElMessage.success('批量删除成功');
    if (ids.includes(selectedPlanID.value)) {
      selectedPlanID.value = "";
    }
    await loadBaseData();
  } catch (e) {
    if (e !== 'cancel') {
      console.error('删除失败:', e);
      ElMessage.error('批量删除失败');
    }
  }
}

async function deletePlan(row: PlanUploadView) {
  try {
    await ElMessageBox.confirm(`确定删除文件 ${row.fileName} 吗？`, '提示', { type: 'warning' });
    await PlanCompareAPI.deletePlanFiles([row.id]);
    ElMessage.success('删除成功');
    if (selectedPlanID.value === row.id) {
      selectedPlanID.value = "";
    }
    await loadBaseData();
  } catch (e) {
    if (e !== 'cancel') {
      console.error('删除失败:', e);
      ElMessage.error('删除失败');
    }
  }
}

async function previewPlan(row: PlanUploadView) {
  previewTitle.value = row.fileName;
  showPreviewDialog.value = true;
  previewLoading.value = true;
  try {
    previewData.value = await PlanCompareAPI.getPlanFilePreview(row.id, 100);
  } catch (e) {
    console.error('预览失败:', e);
    ElMessage.error('获取预览数据失败');
  } finally {
    previewLoading.value = false;
  }
}

onMounted(() => {
  void loadBaseData();
});
</script>
