<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <div class="flex items-center gap-4">
        <p class="text-sm text-text-muted">管理和执行已创建的任务绑定组合</p>
      </div>

      <!-- 操作按钮区域 -->
      <div class="flex gap-3">
        <el-button
          v-if="executionView.active && isRunning"
          type="danger"
          plain
          @click="stopExecution"
        >
          停止任务
        </el-button>
        <el-button
          :icon="Plus"
          @click="goToTaskCreate"
        >
          创建新任务
        </el-button>
        <!-- 执行详情视图时显示返回按钮，否则显示刷新按钮 -->
        <el-button
          v-if="shouldShowExecutionView"
          :icon="Back"
          @click="closeExecutionView"
        >
          返回任务列表
        </el-button>
        <el-button
          v-else
          :icon="RefreshRight"
          @click="refreshTaskList"
        >
          刷新
        </el-button>
      </div>
    </div>

    <!-- ==================== 任务执行内容 ==================== -->
    <div class="flex-1 min-h-0 flex flex-col gap-4">
      <!-- 搜索和筛选 -->
      <div class="flex items-center gap-4 flex-shrink-0">
        <el-input
          v-model="searchQuery"
          placeholder="搜索任务..."
          :prefix-icon="Search"
          class="w-64"
        />
        <el-select v-model="filterStatus" placeholder="全部状态" class="w-32" clearable>
          <el-option label="待执行" value="pending" />
          <el-option label="执行中" value="running" />
          <el-option label="已完成" value="completed" />
          <el-option label="部分成功" value="partial" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-select v-model="filterMode" placeholder="全部模式" class="w-48" clearable>
          <el-option label="模式A（命令组→设备组）" value="group" />
          <el-option label="模式B（IP绑定→独立命令）" value="binding" />
        </el-select>
      </div>

      <div class="flex items-center justify-between gap-3 flex-shrink-0 px-3 py-2 rounded-lg border border-border bg-bg-card/70 text-xs text-text-muted">
        <span>状态：tasks={{ tasks.length }} / filtered={{ filteredTasks.length }} / active={{ shouldShowExecutionView ? "true" : "false" }} / running={{ isRunning ? "true" : "false" }} / awaiting={{ awaitingSnapshot ? "true" : "false" }}</span>
        <span class="font-mono">run={{ executionView.runId || taskexecStore.currentRunId || "-" }}</span>
      </div>

      <!-- 执行视图（正在运行时显示） -->
      <template v-if="shouldShowExecutionView">
        <div class="flex-1 flex flex-col gap-4">
          <!-- 批量重置 SSH 密钥提示条 -->
          <el-alert
            v-if="sshKeyMismatchUnits.length > 0 && !hideSshMismatchBanner"
            type="warning"
            show-icon
            class="shrink-0"
            @close="hideSshMismatchBanner = true"
          >
            <template #title>
              <div class="flex items-center justify-between w-full">
                <span>发现 {{ sshKeyMismatchUnits.length }} 台设备主机密钥不匹配导致连接失败。</span>
                <el-button type="primary" size="small" @click="sshMismatchModal.show = true">
                  批量处理
                </el-button>
              </div>
            </template>
          </el-alert>

          <el-card shadow="never" :body-style="{ padding: '16px' }">
            <div class="flex items-start justify-between gap-4">
              <div class="space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-sm font-semibold text-text-primary">
                    {{ executionView.taskName || "任务执行" }}
                  </span>
                  <span
                    class="px-2.5 py-1 rounded-full text-xs border flex items-center gap-1.5"
                    :class="taskStatusBadge(executionRunStatus)"
                  >
                    <span
                      class="w-1.5 h-1.5 rounded-full"
                      :class="taskStatusDot(executionRunStatus)"
                    ></span>
                    {{ taskStatusLabel(executionRunStatus) }}
                  </span>
                  <el-tag v-if="executionView.taskType === 'topology'" size="small">
                    拓扑采集
                  </el-tag>
                </div>
                <p class="text-sm text-text-primary">
                  {{ executionStatusSummary }}
                </p>
                <p v-if="executionStatusDetail" class="text-xs" :class="executionStatusDetailClass">
                  {{ executionStatusDetail }}
                </p>
              </div>
              <el-button
                v-if="executionView.taskType === 'topology' && isExecutionTerminal"
                plain
                type="primary"
                @click="router.push('/topology')"
              >
                查看拓扑图谱
              </el-button>
            </div>
          </el-card>

          <!-- 进度条 -->
          <div class="flex-shrink-0 space-y-1.5">
            <div class="flex items-center justify-between text-xs text-text-muted">
              <span>{{ executionView.taskName || "任务执行" }} - 总体进度</span>
              <span class="font-mono">{{ progressPercent }}%</span>
            </div>
            <el-progress :percentage="progressPercent" :status="progressPercent === 100 ? 'success' : ''" :show-text="false" />
          </div>

          <!-- Stage 进度展示 -->
          <div v-if="executionStages.length > 0" class="flex-shrink-0">
            <StageProgress :stages="executionStages" :units="executionUnits" />
          </div>

          <!-- 拓扑采集计划证据 -->
          <el-card
            v-if="executionView.taskType === 'topology'"
            shadow="never"
            :body-style="{ padding: '16px' }"
          >
            <div class="flex items-center justify-between gap-3 mb-3">
              <div>
                <h4 class="text-sm font-semibold">拓扑采集计划证据</h4>
                <p class="text-xs text-text-muted mt-1">展示字段启停、命令来源与厂商来源，支持运行后复盘。</p>
              </div>
              <div class="text-xs text-text-muted text-right">
                <div>设备计划: {{ topologyCollectionPlanRows.length }}</div>
                <div>启用字段: {{ topologyPlanEnabledCount }} / 禁用字段: {{ topologyPlanDisabledCount }}</div>
              </div>
            </div>

            <div v-if="topologyPlanLoading" class="text-xs text-text-muted">正在加载采集计划快照...</div>
            <el-alert v-else-if="topologyPlanError" type="error" :closable="false" :title="`加载采集计划失败：${topologyPlanError}`" />
            <el-empty v-else-if="topologyCollectionPlanRows.length === 0" description="暂无采集计划快照，待设备采集阶段产物生成后自动展示。" :image-size="60" />
            <div v-else class="space-y-3 max-h-64 overflow-auto scrollbar-custom pr-1">
              <div
                v-for="plan in topologyCollectionPlanRows"
                :key="`${plan.artifactKey || '-'}:${plan.deviceIp}:${String(plan.generatedAt || '-')}`"
                class="rounded-lg border border-border bg-bg-panel/40 px-3 py-2 space-y-2"
              >
                <div class="flex items-center justify-between gap-3 text-xs">
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="font-mono font-medium">{{ plan.deviceIp }}</span>
                    <el-tag size="small" type="info">厂商: {{ plan.resolvedVendor || "-" }}</el-tag>
                    <span class="text-text-muted">来源: {{ vendorSourceLabel(plan.vendorSource) }}</span>
                  </div>
                  <span class="text-text-muted">{{ formatDate(String(plan.generatedAt || "")) }}</span>
                </div>
                <div class="text-xs text-text-muted">字段: 启用 {{ enabledCommandCount(plan) }} / 禁用 {{ disabledCommandCount(plan) }}</div>
                <div class="flex flex-wrap gap-1.5">
                  <span
                    v-for="cmd in (plan.commands || []).slice(0, 6)"
                    :key="`${cmd.fieldKey}:${cmd.commandSource}`"
                    class="px-1.5 py-0.5 rounded border text-[11px]"
                    :class="cmd.enabled ? 'border-success/30 bg-success/10 text-success' : 'border-border bg-bg-card text-text-muted'"
                  >
                    {{ cmd.fieldKey }} · {{ commandSourceLabel(cmd.commandSource) }}
                  </span>
                  <span v-if="(plan.commands || []).length > 6" class="px-1.5 py-0.5 rounded border border-border text-[11px] text-text-muted">
                    +{{ (plan.commands || []).length - 6 }}
                  </span>
                </div>
              </div>
            </div>
          </el-card>

          <!-- 设备卡片网格 -->
          <div class="flex-1 overflow-auto scrollbar-custom min-h-0 relative" ref="devicesContainer">
            <div v-if="deviceCardUnits.length === 0 && (isRunning || awaitingSnapshot)" class="flex flex-col items-center justify-center h-48 text-text-muted gap-3">
              <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
              <p class="text-sm">正在初始化任务...</p>
            </div>
            <div v-else>
              <div v-if="deviceCardUnits.length > 50" class="text-xs text-text-muted mb-2 px-1">
                共 {{ deviceCardUnits.length }} 台设备，显示前 {{ visibleUnitCount }} 台活跃设备
                <el-button v-if="deviceCardUnits.length > visibleUnitCount" link type="primary" class="ml-2" @click="showAllDevices = !showAllDevices">
                  {{ showAllDevices ? "收起" : "显示全部" }}
                </el-button>
              </div>

              <div class="grid grid-cols-3 gap-4">
                <div
                  v-for="unit in visibleUnits"
                  :key="unit.targetKey"
                  class="bg-bg-card border rounded-xl overflow-hidden shadow-card transition-all duration-300"
                  :class="statusBorder(unit.status)"
                >
                  <div class="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-panel">
                    <span class="font-mono text-sm font-semibold">{{ unit.targetKey }}</span>
                    <span class="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full border" :class="statusBadge(unit.status)">
                      <span class="w-1.5 h-1.5 rounded-full" :class="statusDot(unit.status)"></span>
                      {{ statusLabel(unit.status) }}
                    </span>
                  </div>
                  <div class="px-4 py-2 border-b border-border bg-bg-card/50 space-y-1">
                    <div class="flex items-center justify-between gap-3 text-xs">
                      <span class="text-text-muted">步骤进度</span>
                      <span class="font-mono text-text-primary">{{ unit.doneSteps }}/{{ unit.totalSteps }}</span>
                    </div>
                    <div v-if="unit.errorMessage" class="text-xs text-error break-all">
                      {{ unit.errorMessage }}
                    </div>
                  </div>
                  <VirtualLogTerminal :logs="unit.logs || []" :total-count="unit.logCount || 0" :truncated="unit.truncated || false" :device-ip="unit.targetKey" />
                </div>
              </div>

              <div v-if="!showAllDevices && deviceCardUnits.length > visibleUnitCount" class="text-center py-4 text-text-muted text-sm">
                还有 {{ deviceCardUnits.length - visibleUnitCount }} 台设备已完成或等待中
                <el-button link type="primary" class="ml-2" @click="showAllDevices = true">显示全部</el-button>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- 任务列表视图 -->
      <template v-else>
        <div class="flex-1 overflow-auto scrollbar-custom min-h-0 space-y-4">
          <div v-if="loading" class="flex items-center justify-center h-48">
            <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
          </div>
          <el-empty v-else-if="loadError" :description="`任务列表加载失败: ${loadError}`" :image-size="80">
            <el-button @click="loadTasks('retry')">重试加载</el-button>
          </el-empty>
          <el-empty v-else-if="filteredTasks.length === 0" description="暂无任务，请前往「任务创建」页面创建" :image-size="80" />
          <div v-else class="grid grid-cols-2 gap-4">
            <el-card
              v-for="task in filteredTasks"
              :key="task.id"
              shadow="hover"
              :body-style="{ padding: '0px' }"
              class="border border-border rounded-xl group/card"
            >
              <!-- 卡片头部 -->
              <div class="flex items-start justify-between px-4 py-3 border-b border-border bg-bg-panel">
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="text-sm font-semibold truncate">{{ task.name }}</h3>
                    <el-tag size="small" :type="isTopologyTask(task) ? '' : 'info'">{{ isTopologyTask(task) ? "拓扑采集" : "普通任务" }}</el-tag>
                    <el-tag size="small" :type="task.mode === 'group' ? 'success' : 'warning'">{{ task.mode === "group" ? "模式A" : "模式B" }}</el-tag>
                    <span
                      class="flex-shrink-0 flex items-center gap-1 text-xs px-2 py-0.5 rounded-full border font-medium"
                      :class="taskStatusBadge(task.latestRunStatus || task.status)"
                    >
                      <span class="w-1.5 h-1.5 rounded-full" :class="taskStatusDot(task.latestRunStatus || task.status)"></span>
                      {{ taskStatusLabel(task.latestRunStatus || task.status) }}
                    </span>
                  </div>
                  <p class="text-xs text-text-muted line-clamp-1 mt-1">{{ task.description || "暂无描述" }}</p>
                </div>
                <!-- 操作按钮 -->
                <div class="flex items-center gap-1 ml-2">
                  <el-tooltip content="查看详情" placement="top">
                    <el-button :icon="Document" circle size="small" @click="openTaskDetail(task)" />
                  </el-tooltip>
                  <el-tooltip :content="!task.canEdit ? '任务存在活跃运行，不可编辑' : isTopologyTask(task) ? '编辑拓扑任务（支持字段级覆盖）' : '编辑任务'" placement="top">
                    <el-button :icon="Edit" circle size="small" :disabled="!task.canEdit" @click="openTaskEdit(task)" />
                  </el-tooltip>
                  <el-tooltip content="查看执行历史" placement="top">
                    <el-button :icon="TopRight" circle size="small" @click="showExecutionHistory(task)" />
                  </el-tooltip>
                  <el-tooltip content="执行" placement="top">
                    <el-button :icon="VideoPlay" type="primary" plain circle size="small" :disabled="isRunning || awaitingSnapshot" @click="executeTask(task)" />
                  </el-tooltip>
                  <el-tooltip content="删除" placement="top">
                    <el-button :icon="Delete" type="danger" plain circle size="small" @click="confirmDelete(task)" />
                  </el-tooltip>
                </div>
              </div>

              <!-- 绑定详情 -->
              <div class="px-4 py-3">
                <template v-if="isTopologyTask(task)">
                  <div class="flex items-center gap-2 text-xs">
                    <el-tag size="small" type="info">厂商: {{ topologyVendorLabel(task) }}</el-tag>
                    <span class="text-text-secondary">{{ topologyDeviceCount(task) }} 台设备</span>
                  </div>
                </template>
                <template v-else-if="task.mode === 'group'">
                  <div v-for="(item, idx) in task.items || []" :key="idx" class="flex items-center gap-2 text-xs">
                    <el-tag size="small" type="info" class="max-w-[200px] truncate">命令组: {{ String(item.commandGroupId || "-").substring(0, 8) }}...</el-tag>
                    <el-icon class="text-text-muted"><Right /></el-icon>
                    <span class="text-text-secondary">{{ Array.isArray(item.deviceIDs) ? item.deviceIDs.length : 0 }} 台设备</span>
                  </div>
                </template>
                <template v-else>
                  <div class="space-y-1">
                    <div v-for="(item, idx) in (task.items || []).slice(0, 3)" :key="idx" class="flex items-center gap-2 text-xs">
                      <span class="font-mono text-text-secondary">{{ Array.isArray(item.deviceIDs) && item.deviceIDs.length > 0 ? item.deviceIDs[0] : "-" }}</span>
                      <el-icon class="text-text-muted"><Right /></el-icon>
                      <span class="text-text-muted truncate">{{ Array.isArray(item.commands) ? item.commands.length : 0 }} 条命令</span>
                    </div>
                    <div v-if="(task.items || []).length > 3" class="text-xs text-text-muted">
                      +{{ (task.items || []).length - 3 }} 台设备...
                    </div>
                  </div>
                </template>
              </div>

              <!-- 标签和时间 -->
              <div class="px-4 py-2 border-t border-border bg-bg-secondary/30 text-xs text-text-muted flex items-center justify-between">
                <div class="flex items-center gap-1.5 overflow-hidden">
                  <el-tag v-for="tag in (task.tags || []).slice(0, 3)" :key="tag" size="small" type="info">{{ tag }}</el-tag>
                </div>
                <span class="flex-shrink-0 ml-2">{{ formatDate(task.updatedAt) }}</span>
              </div>
            </el-card>
          </div>
        </div>
      </template>
    </div>

    <!-- SSH 密钥重置冲突弹窗 -->
    <el-dialog
      v-model="sshMismatchModal.show"
      title="主机密钥不匹配"
      width="500px"
      :close-on-click-modal="false"
      class="rounded-xl overflow-hidden"
    >
      <div class="space-y-4">
        <el-alert
          type="warning"
          show-icon
          :closable="false"
          title="检测到以下设备的 SSH 主机密钥发生改变，可能是由于设备重装或网络变更导致。"
        />
        <div class="bg-bg-panel border border-border rounded-lg p-3 max-h-48 overflow-y-auto scrollbar-custom space-y-1.5">
          <div
            v-for="unit in sshKeyMismatchUnits"
            :key="unit.targetKey"
            class="flex items-center gap-2 text-sm"
          >
            <span class="font-mono text-text-primary">{{ unit.targetKey }}</span>
            <el-tag size="small" type="danger">指纹冲突</el-tag>
          </div>
        </div>
        <p class="text-sm text-text-muted">
          您可以选择批量清理这些设备的已知指纹记录，重置后下次连接会自动接受新密钥。
        </p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3 mt-4">
          <el-button @click="sshMismatchModal.show = false">取消</el-button>
          <el-button
            type="warning"
            plain
            :loading="sshMismatchModal.processing"
            @click="handleResetSSHAndRetry(false)"
          >
            仅重置密钥
          </el-button>
          <el-button
            type="primary"
            :loading="sshMismatchModal.processing"
            @click="handleResetSSHAndRetry(true)"
          >
            重置并重试任务
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 执行历史抽屉 -->
    <ExecutionHistoryDrawer
      v-model="historyDrawer.show"
      :task-group-id="historyDrawer.taskGroupId"
      :task-group-name="historyDrawer.taskGroupName"
    />

    <TaskDetailModal
      v-model="detailModal.show"
      :loading="detailModal.loading"
      :detail="detailModal.detail"
      @edit="editFromDetail"
    />

    <TaskEditModal
      v-model="editModal.show"
      :task="editModal.task"
      :loading="editModal.loading"
      :saving="editModal.saving"
      :all-devices="editReferences.devices"
      :command-groups="editReferences.commandGroups"
      @save="saveTaskEdit"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Search, Plus, Back, RefreshRight, Document, Edit, TopRight, VideoPlay, Delete, Right } from "@element-plus/icons-vue";
import {
  CommandGroupAPI,
  DeviceAPI,
  TaskExecutionAPI,
  TaskGroupAPI,
} from "../services/api";
import type {
  CommandGroup,
  DeviceAsset,
  TaskGroup,
  TaskGroupDetailViewModel,
  TaskGroupListView,
  TopologyCollectionPlanArtifact,
} from "../services/api";
import { useTaskexecStore } from "../stores/taskexecStore";
import VirtualLogTerminal from "../components/task/VirtualLogTerminal.vue";
import ExecutionHistoryDrawer from "../components/task/ExecutionHistoryDrawer.vue";
import TaskDetailModal from "../components/task/TaskDetailModal.vue";
import TaskEditModal from "../components/task/TaskEditModal.vue";
import StageProgress from "../components/task/StageProgress.vue";
import type { StageSnapshot, UnitSnapshot } from "../types/taskexec";
import { getLogger } from '@/utils/logger'

// 创建模块专用logger，用于调试日志写入frontend.log
const log = getLogger().createModuleLogger('TaskExecution')
// 保留logger变量以兼容其他代码
const logger = getLogger()

const router = useRouter();
const route = useRoute();
const taskexecStore = useTaskexecStore();

// ================== 任务执行状态 ==================
const loading = ref(false);
const loadError = ref("");
const tasks = ref<TaskGroupListView[]>([]);
const searchQuery = ref("");
const filterStatus = ref("");
const filterMode = ref("");

// 执行视图状态 (阶段3: 统一执行框架 - 使用runId驱动)
const executionView = ref({
  active: false,
  taskId: 0 as number,
  runId: "" as string, // 统一运行时runId
  taskName: "",
  taskType: "normal" as "normal" | "topology",
});
const awaitingSnapshot = ref(false);
let snapshotTimeoutTimer: ReturnType<typeof setTimeout> | null = null;
const SNAPSHOT_TIMEOUT = 10000;
let snapshotPollTimer: ReturnType<typeof setInterval> | null = null;
const SNAPSHOT_POLL_INTERVAL = 1000;

// 删除弹窗 (Removed)

// 执行历史抽屉
const historyDrawer = ref({
  show: false,
  taskGroupId: "",
  taskGroupName: "",
});

// 任务详情弹窗
const detailModal = ref({
  show: false,
  loading: false,
  detail: null as TaskGroupDetailViewModel | null,
});

// 任务编辑弹窗
const editModal = ref({
  show: false,
  loading: false,
  saving: false,
  task: null as TaskGroup | null,
});

const editReferences = ref({
  devices: [] as DeviceAsset[],
  commandGroups: [] as CommandGroup[],
});

const topologyCollectionPlanRows = ref<TopologyCollectionPlanArtifact[]>([]);
const topologyPlanLoading = ref(false);
const topologyPlanError = ref("");
const topologyPlanLastRevision = ref(-1);

// 虚拟滚动优化状态
const showAllDevices = ref(false);
const VISIBLE_DEVICE_LIMIT = 30;

// SSH 主机密钥不匹配处理状态
const hideSshMismatchBanner = ref(false);
const sshMismatchModal = ref({
  show: false,
  processing: false
});

function triggerToast(msg: string, type: "success" | "error" = "success") {
  if (type === "success") {
    ElMessage.success(msg);
  } else {
    ElMessage.error(msg);
  }
}

// ================== 计算属性 - 从 Store 获取 ==================
const executionSnapshot = computed(() => taskexecStore.currentSnapshot);
const isRunning = computed(() => taskexecStore.isRunning);
const shouldShowExecutionView = computed(() => {
  return (
    executionView.value.active &&
    (awaitingSnapshot.value || isRunning.value || !!executionSnapshot.value)
  );
});
const progressPercent = computed(() => {
  const snapshot = executionSnapshot.value;
  if (!snapshot) {
    return 0;
  }

  const runProgress = snapshot.progress ?? 0;
  if (Array.isArray(snapshot.stages) && snapshot.stages.length > 0) {
    const total = snapshot.stages.reduce(
      (sum, stage) => sum + (stage.progress || 0),
      0,
    );
    const stageProgress = Math.round(total / snapshot.stages.length);
    return Math.max(runProgress, stageProgress);
  }

  return runProgress;
});
// ================== 统一运行时 Stage/Unit 数据 (新增) ==================
const executionStages = computed<StageSnapshot[]>(() => {
  return (executionSnapshot.value as any)?.stages || [];
});

const executionUnits = computed<UnitSnapshot[]>(() => {
  return (executionSnapshot.value as any)?.units || [];
});

const executionRunStatus = computed(() => {
  const snapshot = executionSnapshot.value;
  if (snapshot?.status) {
    return snapshot.status;
  }
  if (awaitingSnapshot.value) {
    return "pending";
  }
  return isRunning.value ? "running" : "pending";
});

function isDeviceExecutionUnit(unit: UnitSnapshot): boolean {
  // 基本条件：必须是设备类型单元且有有效目标
  if (
    normalizeString(unit.kind) !== "device" ||
    normalizeString(unit.targetType) !== "device_ip" ||
    normalizeString(unit.targetKey) === ""
  ) {
    return false;
  }

  // 拓扑任务特殊处理：只显示 device_collect 阶段的设备单元
  const taskType = executionView.value.taskType;
  log.debug(`[isDeviceExecutionUnit] taskType=${taskType}, unit.stageId=${unit.stageId}, unit.targetKey=${unit.targetKey}, executionStages count=${executionStages.value.length}`);

  if (taskType === "topology") {
    const stage = executionStages.value.find((s) => s.id === unit.stageId);
    log.debug(`[isDeviceExecutionUnit] Topology task - stage found=${!!stage}, stage.kind=${stage?.kind}, stage.id=${stage?.id}`);

    if (stage && normalizeString(stage.kind) !== "device_collect") {
      log.debug(`[isDeviceExecutionUnit] Filtering out unit - stage.kind=${stage.kind}, unit.targetKey=${unit.targetKey}`);
      return false;
    }

    if (!stage) {
      log.debug(`[isDeviceExecutionUnit] WARNING: No stage found for unit.stageId=${unit.stageId}, Available stage IDs=${executionStages.value.map(s => s.id).join(",")}`);
    }
  } else {
    log.debug(`[isDeviceExecutionUnit] Non-topology task, skipping stage filter`);
  }

  return true;
}

function deviceUnitPriority(
  unit: UnitSnapshot,
  stageOrderMap: Map<string, number>,
): number {
  const statusPriority: Record<string, number> = {
    running: 700,
    failed: 600,
    partial: 550,
    cancelled: 500,
    pending: 300,
    completed: 200,
  };
  const stageOrder = stageOrderMap.get(unit.stageId) ?? 0;
  const progress = Number(unit.progress || 0);
  const doneSteps = Number(unit.doneSteps || 0);
  return (
    (statusPriority[unit.status] || 0) * 1000000 +
    stageOrder * 10000 +
    progress * 100 +
    doneSteps
  );
}

const deviceCardUnits = computed<UnitSnapshot[]>(() => {
  const taskType = executionView.value.taskType;
  const stages = executionStages.value;
  const units = executionUnits.value;

  log.debug(`[deviceCardUnits] computed START - taskType=${taskType}, stages=${stages.length}, units=${units.length}`);

  if (stages.length > 0) {
    const stageDetails = stages.map((s, idx) => `[${idx}] id=${s.id}, kind=${s.kind}, name=${s.name}`).join("; ");
    log.debug(`[deviceCardUnits] Stage details: ${stageDetails}`);
  }

  if (units.length > 0) {
    const unitDetails = units.slice(0, 5).map((u, idx) => `[${idx}] id=${u.id}, stageId=${u.stageId}, kind=${u.kind}, targetKey=${u.targetKey}`).join("; ");
    log.debug(`[deviceCardUnits] Unit details (first 5): ${unitDetails}`);
  }

  const stageOrderMap = new Map<string, number>(
    stages.map((stage) => [stage.id, stage.order]),
  );
  const groups = new Map<string, UnitSnapshot[]>();

  let filteredCount = 0;
  let passedCount = 0;
  for (const unit of units) {
    if (!isDeviceExecutionUnit(unit)) {
      filteredCount++;
      continue;
    }
    passedCount++;
  }

  log.debug(`[deviceCardUnits] computed END - filtered out=${filteredCount}, passed=${passedCount}`);
  
  // Reset iteration
  for (const unit of executionUnits.value) {
    if (!isDeviceExecutionUnit(unit)) {
      continue;
    }
    const key = normalizeString(unit.targetKey);
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)!.push(unit);
  }

  const projected = Array.from(groups.values())
    .map((group) => {
      const sorted = [...group].sort((left, right) => {
        const priorityDiff =
          deviceUnitPriority(right, stageOrderMap) -
          deviceUnitPriority(left, stageOrderMap);
        if (priorityDiff !== 0) {
          return priorityDiff;
        }
        return normalizeString(left.id).localeCompare(
          normalizeString(right.id),
        );
      });
      return sorted.length > 0 ? sorted[0] : null;
    })
    .filter((unit): unit is UnitSnapshot => unit !== null);

  return projected.sort((left, right) => {
    return normalizeString(left.targetKey).localeCompare(
      normalizeString(right.targetKey),
    );
  });
});

const sshKeyMismatchUnits = computed(() => {
  return deviceCardUnits.value.filter((unit) =>
    unit.status === "failed" &&
    (normalizeString(unit.errorMessage).includes("knownhosts: key mismatch") ||
     normalizeString(unit.errorMessage).includes("主机密钥不匹配"))
  );
});

const failedExecutionUnitCount = computed(() => {
  return deviceCardUnits.value.filter((unit) =>
    ["failed", "partial", "cancelled"].includes(unit.status),
  ).length;
});

const firstExecutionError = computed(() => {
  return (
    deviceCardUnits.value.find((unit) => normalizeString(unit.errorMessage))
      ?.errorMessage ??
    executionUnits.value.find((unit) => normalizeString(unit.errorMessage))
      ?.errorMessage ??
    ""
  );
});

const isExecutionTerminal = computed(() => {
  return ["completed", "partial", "failed", "cancelled", "aborted"].includes(
    executionRunStatus.value,
  );
});

const executionStatusSummary = computed(() => {
  const isTopology = executionView.value.taskType === "topology";
  switch (executionRunStatus.value) {
    case "pending":
      return isTopology
        ? "拓扑采集任务正在初始化，等待首个快照返回。"
        : "任务正在初始化，等待首个快照返回。";
    case "running":
      return isTopology
        ? "拓扑采集正在执行，页面将实时刷新采集、解析与构建进度。"
        : "任务正在执行，页面将实时刷新设备日志与阶段进度。";
    case "completed":
      return isTopology
        ? "拓扑采集已完成，所有阶段执行成功。"
        : "任务执行完成，所有设备均已成功结束。";
    case "partial":
      return isTopology
        ? "拓扑采集部分完成，存在失败设备或失败阶段。"
        : "任务部分完成，存在失败设备或未完全成功的单元。";
    case "failed":
      return isTopology
        ? "拓扑采集失败，未能完成必要执行阶段。"
        : "任务执行失败。";
    case "aborted":
      return isTopology
        ? "拓扑采集已中止，关键阶段失败导致后续阶段跳过。"
        : "任务已中止。";
    case "cancelled":
      return isTopology ? "拓扑采集已取消。" : "任务已取消。";
    default:
      return isTopology ? "拓扑采集状态未知。" : "任务状态未知。";
  }
});

const executionStatusDetail = computed(() => {
  if (
    executionRunStatus.value === "running" ||
    executionRunStatus.value === "pending"
  ) {
    return executionView.value.taskType === "topology"
      ? "执行链路：设备采集 → 数据解析 → 拓扑构建。任一设备失败都会在下方设备卡片中显示。"
      : "执行链路会按快照持续更新，下方设备卡片与阶段条目会实时反映最新状态。";
  }

  const failedCount = failedExecutionUnitCount.value;
  const firstError = normalizeString(firstExecutionError.value);
  switch (executionRunStatus.value) {
    case "completed":
      return executionView.value.taskType === "topology"
        ? "可以直接前往拓扑图谱页面查看本次采集结果。"
        : "可以返回任务列表查看执行结果。";
    case "partial":
      return failedCount > 0
        ? `共有 ${failedCount} 个执行单元未成功完成，请优先检查失败设备的错误信息。`
        : "部分阶段未完全成功，请检查阶段与设备详情。";
    case "failed":
    case "aborted":
      return firstError || "请检查失败设备的日志和错误原因。";
    case "cancelled":
      return "任务已停止，未完成的阶段不会继续执行。";
    default:
      return "";
  }
});

const executionStatusDetailClass = computed(() => {
  switch (executionRunStatus.value) {
    case "partial":
      return "text-warning";
    case "failed":
    case "aborted":
      return "text-error";
    case "completed":
      return "text-success";
    default:
      return "text-text-muted";
  }
});

const topologyPlanEnabledCount = computed(() =>
  topologyCollectionPlanRows.value.reduce(
    (sum, plan) => sum + enabledCommandCount(plan),
    0,
  ),
);

const topologyPlanDisabledCount = computed(() =>
  topologyCollectionPlanRows.value.reduce(
    (sum, plan) => sum + disabledCommandCount(plan),
    0,
  ),
);

// ================== 虚拟滚动优化计算属性 ==================
const visibleUnitCount = computed(() =>
  showAllDevices.value ? deviceCardUnits.value.length : VISIBLE_DEVICE_LIMIT,
);

const visibleUnits = computed(() => {
  const units = deviceCardUnits.value;

  if (showAllDevices.value) {
    return units;
  }

  const activeStatuses = ["running", "failed", "partial", "pending"];
  const active = units.filter((unit: UnitSnapshot) =>
    activeStatuses.includes(unit.status),
  );
  const inactive = units.filter(
    (unit: UnitSnapshot) => !activeStatuses.includes(unit.status),
  );

  if (active.length >= VISIBLE_DEVICE_LIMIT) {
    return active.slice(0, VISIBLE_DEVICE_LIMIT);
  }

  return [...active, ...inactive].slice(0, VISIBLE_DEVICE_LIMIT);
});

// ================== 生命周期 ==================
onMounted(() => {
  void syncExecutionView();
  void taskexecStore.loadRunHistory(50);
  void loadTasks("mounted");
});

onUnmounted(() => {
  clearSnapshotTimeout();
  stopSnapshotPolling();
});

watch(isRunning, (running, wasRunning) => {
  if (!running && wasRunning && executionView.value.active) {
    stopSnapshotPolling();
    void taskexecStore.loadRunHistory(50);
    void loadTasks("run-finished");
  }
});

watch(
  () => route.query.refresh,
  (refreshToken, previousToken) => {
    if (!refreshToken || refreshToken === previousToken) {
      return;
    }
    logger.debug(
      `检测到路由刷新信号，重新加载任务列表，token=${refreshToken}`,
      'TaskExecution',
    );
    void loadTasks("route-refresh");
  },
);

function normalizeString(value: unknown, fallback: string = ""): string {
  return typeof value === "string" ? value.trim() : fallback;
}

function normalizeStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeNumberArray(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item) && item > 0);
}

function normalizeTaskItem(item: any) {
  return {
    commandGroupId: normalizeString(item?.commandGroupId),
    commands: normalizeStringArray(item?.commands),
    deviceIDs: normalizeNumberArray(item?.deviceIDs),
  };
}

function normalizeTaskListEntry(task: any): TaskGroupListView {
  const normalized = {
    ...(task || {}),
    id: Number(task?.id || 0),
    name: normalizeString(task?.name, "未命名任务"),
    description: normalizeString(task?.description, "暂无描述"),
    deviceGroup: normalizeString(task?.deviceGroup),
    commandGroup: normalizeString(task?.commandGroup),
    taskType: normalizeString(task?.taskType, "normal"),
    topologyVendor: normalizeString(task?.topologyVendor),
    mode: normalizeString(task?.mode, "group"),
    status: normalizeString(task?.status, "pending"),
    latestRunId: normalizeString(task?.latestRunId),
    latestRunStatus: normalizeString(task?.latestRunStatus),
    latestRunStartedAt: normalizeString(task?.latestRunStartedAt),
    latestRunFinishedAt: normalizeString(task?.latestRunFinishedAt),
    activeRunCount: Number(task?.activeRunCount || 0),
    canEdit: task?.canEdit !== false,
    tags: normalizeStringArray(task?.tags),
    items: Array.isArray(task?.items)
      ? task.items.map((item: any) => normalizeTaskItem(item))
      : [],
    createdAt: normalizeString(task?.createdAt),
    updatedAt: normalizeString(task?.updatedAt || task?.createdAt),
    enableRawLog: Boolean(task?.enableRawLog),
  };
  return normalized as TaskGroupListView;
}

function matchesTaskQuery(task: TaskGroupListView, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) {
    return true;
  }
  const name = normalizeString(task.name).toLowerCase();
  const description = normalizeString(task.description).toLowerCase();
  const tags = normalizeStringArray(task.tags);
  return (
    name.includes(q) ||
    description.includes(q) ||
    tags.some((tag) => tag.toLowerCase().includes(q))
  );
}

// ================== 过滤逻辑（任务列表） ==================
const filteredTasks = computed(() => {
  let result = tasks.value.filter((task) => task && task.id > 0);
  if (searchQuery.value) {
    result = result.filter((task) => matchesTaskQuery(task, searchQuery.value));
  }
  if (filterStatus.value) {
    result = result.filter(
      (task) => normalizeString(task.status, "pending") === filterStatus.value,
    );
  }
  if (filterMode.value) {
    result = result.filter(
      (task) => normalizeString(task.mode, "group") === filterMode.value,
    );
  }
  return result;
});

function refreshTaskList() {
  void loadTasks("manual-refresh");
}

function resetExecutionViewState(reason: string) {
  logger.debug(
    `重置执行视图状态，reason=${reason}, active=${executionView.value.active}, runId=${executionView.value.runId}`,
    'TaskExecution',
  );
  executionView.value.active = false;
  executionView.value.runId = "";
  executionView.value.taskName = "";
  executionView.value.taskType = "normal";
  awaitingSnapshot.value = false;
  clearSnapshotTimeout();
  stopSnapshotPolling();
  taskexecStore.setCurrentRunId(null);
  hideSshMismatchBanner.value = false;
  topologyCollectionPlanRows.value = [];
  topologyPlanError.value = "";
  topologyPlanLoading.value = false;
  topologyPlanLastRevision.value = -1;
}

// 加载任务列表
async function loadTasks(reason: string = "manual") {
  loading.value = true;
  loadError.value = "";
  try {
    logger.debug(`开始加载任务列表，reason=${reason}`, 'TaskExecution');
    const result = await TaskGroupAPI.listTaskGroups();
    if (!Array.isArray(result)) {
      throw new Error("任务列表接口返回非数组数据");
    }
    tasks.value = result
      .filter(Boolean)
      .map((item) => normalizeTaskListEntry(item));
    logger.debug(
      `任务列表加载完成，count=${tasks.value.length}, reason=${reason}`,
      'TaskExecution',
    );
  } catch (err: any) {
    const message = err?.message || String(err);
    logger.error('加载任务列表失败', 'TaskExecution', err);
    loadError.value = message;
    tasks.value = [];
  } finally {
    loading.value = false;
  }
}

async function syncExecutionView() {
  try {
    const running = await TaskExecutionAPI.listRunningTasks();
    logger.debug(`同步执行视图: running=${running.length}`, 'TaskExecution');

    if (!running.length) {
      // 没有运行中的任务，检查当前任务是否已完成
      const currentRunId =
        executionView.value.runId || taskexecStore.currentRunId;

      if (currentRunId) {
        // 主动刷新当前 runId 的快照，确认是否终态
        const snapshot = await taskexecStore.refreshSnapshot(currentRunId);
        if (snapshot) {
          // 检查刷新后的快照是否终态
          const terminalStatuses = [
            "completed",
            "partial",
            "failed",
            "cancelled",
            "aborted",
          ];
          if (terminalStatuses.includes(snapshot.status)) {
            // 任务已完成（终态），保留在执行详情页面
            // 用户可以查看执行结果、日志和拓扑采集证据
            // 由用户主动点击"返回任务列表"按钮退出
            logger.debug(
              `任务已完成，保留执行视图以查看结果，status=${snapshot.status}`,
              'TaskExecution',
            );
            // 更新当前快照数据，确保UI显示最新状态
            taskexecStore.updateSnapshot(snapshot.runId, snapshot);
            taskexecStore.setCurrentRunId(snapshot.runId);
            executionView.value.active = true;
            return;
          }
        }
      }

      // 没有当前 runId 或快照刷新失败或非终态，重置执行视图
      resetExecutionViewState("no-running-snapshots");
      return;
    }

    for (const snapshot of running) {
      taskexecStore.updateSnapshot(snapshot.runId, snapshot);
    }

    const currentRunId =
      taskexecStore.currentRunId &&
      running.some((item) => item.runId === taskexecStore.currentRunId)
        ? taskexecStore.currentRunId
        : running[0].runId;
    const snapshot =
      running.find((item) => item.runId === currentRunId) ?? running[0];

    taskexecStore.setCurrentRunId(snapshot.runId);
    executionView.value.active = true;
    executionView.value.runId = snapshot.runId;
    executionView.value.taskName = snapshot.taskName || "任务执行";
    executionView.value.taskType =
      snapshot.runKind === "topology" ? "topology" : "normal";
    logger.debug(
      `已切换到执行视图，runId=${snapshot.runId}, taskName=${snapshot.taskName}, runKind=${snapshot.runKind}`,
      'TaskExecution',
    );
  } catch (err) {
    logger.error('同步执行视图失败', 'TaskExecution', err);
    resetExecutionViewState("sync-error");
  }
}

function startSnapshotPolling() {
  if (snapshotPollTimer) return;
  snapshotPollTimer = setInterval(() => {
    if (!executionView.value.active) {
      stopSnapshotPolling();
      return;
    }
    // 任务完成后停止轮询（终态不再变化）
    if (!awaitingSnapshot.value && isExecutionTerminal.value) {
      stopSnapshotPolling();
      return;
    }
    if (!awaitingSnapshot.value && !isRunning.value) {
      stopSnapshotPolling();
      return;
    }
    void syncExecutionView();
  }, SNAPSHOT_POLL_INTERVAL);
}

function stopSnapshotPolling() {
  if (snapshotPollTimer) {
    clearInterval(snapshotPollTimer);
    snapshotPollTimer = null;
  }
}

async function handleSnapshotTimeout() {
  if (!awaitingSnapshot.value) return;

  const runId = executionView.value.runId || taskexecStore.currentRunId || "";
  const snapshot = runId ? await taskexecStore.refreshSnapshot(runId) : null;
  if (snapshot) {
    awaitingSnapshot.value = false;
    clearSnapshotTimeout();
    executionView.value.active = true;
    executionView.value.runId = snapshot.runId;
    executionView.value.taskName =
      snapshot?.taskName || executionView.value.taskName || "任务执行";
    startSnapshotPolling();
    return;
  }

  logger.warn('快照超时，重置UI状态', 'TaskExecution');
  awaitingSnapshot.value = false;
  triggerToast("任务执行超时，请检查设备连接配置", "error");
  executionView.value.active = false;
  stopSnapshotPolling();
}

function startSnapshotTimeout() {
  if (snapshotTimeoutTimer) {
    clearTimeout(snapshotTimeoutTimer);
  }
  snapshotTimeoutTimer = setTimeout(() => {
    void handleSnapshotTimeout();
  }, SNAPSHOT_TIMEOUT);
}

function clearSnapshotTimeout() {
  if (snapshotTimeoutTimer) {
    clearTimeout(snapshotTimeoutTimer);
    snapshotTimeoutTimer = null;
  }
}

// 执行任务 (阶段3: 统一执行框架 - 统一使用runId驱动)
async function executeTask(task: TaskGroupListView) {
  if (isRunning.value || awaitingSnapshot.value) return;

  executionView.value = {
    active: true,
    taskId: task.id,
    runId: "",
    taskName: task.name,
    taskType: isTopologyTask(task) ? "topology" : "normal",
  };

  taskexecStore.clearEventLogs();
  taskexecStore.setCurrentRunId(null);
  awaitingSnapshot.value = true;
  topologyCollectionPlanRows.value = [];
  topologyPlanError.value = "";
  topologyPlanLastRevision.value = -1;

  startSnapshotTimeout();
  startSnapshotPolling();

  try {
    const runId = await TaskGroupAPI.startTaskGroup(task.id);
    executionView.value.runId = runId;
    taskexecStore.setCurrentRunId(runId);
    await taskexecStore.refreshSnapshot(runId);
    await taskexecStore.loadRunHistory(50);
    await loadTasks("task-started");
  } catch (err: any) {
    logger.error('执行任务失败', 'TaskExecution', err);
    triggerToast(`执行失败: ${err?.message || err}`, "error");
    executionView.value.active = false;
    clearSnapshotTimeout();
    stopSnapshotPolling();
  }
}

async function loadTopologyCollectionPlans(runId: string) {
  const normalizedRunID = normalizeString(runId);
  if (executionView.value.taskType !== "topology" || normalizedRunID === "") {
    topologyCollectionPlanRows.value = [];
    topologyPlanError.value = "";
    topologyPlanLoading.value = false;
    return;
  }

  topologyPlanLoading.value = true;
  topologyPlanError.value = "";
  try {
    const plans =
      await TaskExecutionAPI.getTopologyCollectionPlans(normalizedRunID);
    topologyCollectionPlanRows.value = Array.isArray(plans)
      ? plans.filter(Boolean)
      : [];
  } catch (err: any) {
    topologyPlanError.value = err?.message || String(err);
    topologyCollectionPlanRows.value = [];
  } finally {
    topologyPlanLoading.value = false;
  }
}

watch(
  () => executionView.value.runId,
  (runId, previousRunId) => {
    if (runId === previousRunId) {
      return;
    }
    topologyPlanLastRevision.value = -1;
    if (executionView.value.taskType === "topology" && normalizeString(runId)) {
      void loadTopologyCollectionPlans(runId);
    }
  },
);

watch(
  () => executionView.value.taskType,
  (taskType) => {
    if (taskType !== "topology") {
      topologyCollectionPlanRows.value = [];
      topologyPlanError.value = "";
      topologyPlanLoading.value = false;
      topologyPlanLastRevision.value = -1;
      return;
    }
    if (normalizeString(executionView.value.runId)) {
      void loadTopologyCollectionPlans(executionView.value.runId);
    }
  },
);

// 监听快照变化
watch(executionSnapshot, (snapshot) => {
  if (snapshot) {
    log.debug(`[executionSnapshot watch] triggered - runKind=${snapshot.runKind}, runId=${snapshot.runId}, stages=${snapshot.stages?.length || 0}, units=${snapshot.units?.length || 0}`);

    if (snapshot.stages && snapshot.stages.length > 0) {
      const stageDetails = snapshot.stages.map((s, idx) => `[${idx}] id=${s.id}, kind=${s.kind}, name=${s.name}`).join("; ");
      log.debug(`[executionSnapshot watch] Stage details: ${stageDetails}`);
    }

    awaitingSnapshot.value = false;
    clearSnapshotTimeout();
    executionView.value.active = true;
    executionView.value.runId = snapshot.runId;
    const newTaskType = snapshot.runKind === "topology" ? "topology" : "normal";
    log.debug(`[executionSnapshot watch] Setting taskType=${newTaskType} (was: ${executionView.value.taskType})`);
    executionView.value.taskType = newTaskType;
    if (snapshot.taskName) {
      executionView.value.taskName = snapshot.taskName;
    }

    if (snapshot.runKind === "topology") {
      const revision = Number(snapshot.revision || 0);
      if (revision !== topologyPlanLastRevision.value) {
        topologyPlanLastRevision.value = revision;
        void loadTopologyCollectionPlans(snapshot.runId);
      }
    }
  }
});

// 删除任务
function confirmDelete(task: TaskGroupListView) {
  ElMessageBox.confirm(
    `确定要删除任务「${task.name}」吗？此操作不可撤销。`,
    '确认删除',
    {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(() => {
    doDelete(task.id);
  }).catch(() => {});
}

async function doDelete(taskId: number) {
  try {
    await TaskGroupAPI.deleteTaskGroup(taskId);
    triggerToast("任务已删除", "success");
    void loadTasks();
  } catch (err: any) {
    triggerToast(`删除失败: ${err?.message || err}`, "error");
  }
}

// 关闭执行视图：仅解绑当前 run，不删除快照缓存
function closeExecutionView() {
  if (isRunning.value || awaitingSnapshot.value) return;
  logger.debug('用户关闭执行视图', 'TaskExecution');
  resetExecutionViewState("close-execution-view");
  void loadTasks("close-execution-view");
}

// 停止执行任务 (阶段3: 使用统一运行时的CancelTask)
async function handleResetSSHAndRetry(retry: boolean) {
  sshMismatchModal.value.processing = true;
  try {
    const mismatchUnits = sshKeyMismatchUnits.value;
    if (mismatchUnits.length === 0) {
      return;
    }
    const failedIps = mismatchUnits.map((u) => normalizeString(u.targetKey));

    // 1. 获取设备列表并匹配 ID
    const allDevices = await DeviceAPI.listDevices();
    const targetDevices = allDevices.filter((d) => failedIps.includes(d.ip));

    // 2. 批量重置 SSH 密钥
    if (targetDevices.length > 0) {
      const resetPromises = targetDevices.map((dev) =>
        DeviceAPI.resetDeviceSSHHostKey(dev.id)
      );
      await Promise.all(resetPromises);
      triggerToast(`成功重置了 ${targetDevices.length} 台设备的主机密钥`);
    }

    sshMismatchModal.value.show = false;
    hideSshMismatchBanner.value = true;

    // 3. 如果需要重试
    if (retry) {
      if (isRunning.value) {
        await stopExecution();
      }
      const currentTask = tasks.value.find(
        (t) => t.id === executionView.value.taskId
      );
      if (currentTask) {
        // executeTask expect task as param
        await executeTask(currentTask);
      } else {
        triggerToast("未找到当前任务配置，无法重试", "error");
      }
    }
  } catch (err: any) {
    const msg = err?.message || String(err);
    triggerToast(`处理失败: ${msg}`, "error");
  } finally {
    sshMismatchModal.value.processing = false;
  }
}

async function stopExecution() {
  if (!confirm("确定要停止当前执行任务吗？")) {
    return;
  }

  if (executionView.value.runId) {
    try {
      await taskexecStore.cancelTask(executionView.value.runId);
      triggerToast("已发送停止信号");
      return;
    } catch (err: any) {
      triggerToast(`停止失败: ${err?.message || err}`, "error");
    }
  }
}

// 导航
function goToTaskCreate() {
  router.push("/tasks");
}

function isTopologyTask(task: TaskGroupListView | TaskGroup) {
  return ((task as any).taskType || "normal") === "topology";
}

function topologyVendorLabel(task: TaskGroupListView | TaskGroup) {
  const vendor = ((task as any).topologyVendor || "").trim();
  return vendor === "" ? "自动识别" : vendor;
}

function topologyDeviceCount(task: TaskGroupListView | TaskGroup) {
  const set = new Set<number>();
  for (const item of task.items || []) {
    for (const id of item.deviceIDs || []) {
      set.add(id);
    }
  }
  return set.size;
}

function enabledCommandCount(plan: TopologyCollectionPlanArtifact): number {
  if (!Array.isArray(plan?.commands)) {
    return 0;
  }
  return plan.commands.filter((cmd: any) => Boolean(cmd?.enabled)).length;
}

function disabledCommandCount(plan: TopologyCollectionPlanArtifact): number {
  if (!Array.isArray(plan?.commands)) {
    return 0;
  }
  return plan.commands.filter((cmd: any) => !Boolean(cmd?.enabled)).length;
}

function commandSourceLabel(source: string): string {
  const map: Record<string, string> = {
    task_override: "任务覆盖",
    vendor_config: "厂商配置",
    profile_seed: "画像种子",
    builtin_seed: "内置种子",
    disabled: "禁用",
  };
  const key = normalizeString(source);
  return map[key] ?? (key || "未知来源");
}

function vendorSourceLabel(source: string): string {
  const map: Record<string, string> = {
    task: "任务显式",
    inventory: "资产信息",
    detect: "自动探测",
    fallback_default: "默认回退",
  };
  const key = normalizeString(source);
  return map[key] ?? (key || "未知来源");
}

// 查看执行历史
function showExecutionHistory(task: TaskGroupListView) {
  historyDrawer.value = {
    show: true,
    taskGroupId: String(task.id),
    taskGroupName: task.name,
  };
}

async function ensureEditReferences() {
  if (
    editReferences.value.devices.length > 0 &&
    editReferences.value.commandGroups.length > 0
  ) {
    return;
  }

  const [devices, commandGroups] = await Promise.all([
    DeviceAPI.listDevices(),
    CommandGroupAPI.listCommandGroups(),
  ]);

  editReferences.value = {
    devices: devices || [],
    commandGroups: commandGroups || [],
  };
}

async function openTaskDetail(task: TaskGroupListView) {
  detailModal.value.show = true;
  detailModal.value.loading = true;
  detailModal.value.detail = null;

  try {
    detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(task.id);
  } catch (err: any) {
    detailModal.value.show = false;
    triggerToast(`加载任务详情失败: ${err?.message || err}`, "error");
  } finally {
    detailModal.value.loading = false;
  }
}

async function openTaskEdit(task: TaskGroupListView) {
  if (!task.canEdit) {
    triggerToast("任务存在活跃运行，不可编辑", "error");
    return;
  }

  editModal.value.show = true;
  editModal.value.loading = true;
  editModal.value.task = null;

  try {
    const [freshTask] = await Promise.all([
      TaskGroupAPI.getTaskGroup(task.id),
      ensureEditReferences(),
    ]);
    editModal.value.task = {
      ...(freshTask as any),
      status: task.latestRunStatus || task.status || "pending",
    };
  } catch (err: any) {
    editModal.value.show = false;
    triggerToast(`加载编辑数据失败: ${err?.message || err}`, "error");
  } finally {
    editModal.value.loading = false;
  }
}

async function editFromDetail() {
  const currentTask = detailModal.value.detail?.task;
  if (!currentTask) return;
  detailModal.value.show = false;
  await openTaskEdit({
    ...currentTask,
    status: detailModal.value.detail?.latestRunStatus || "pending",
    latestRunId: detailModal.value.detail?.latestRunId || "",
    latestRunStatus: detailModal.value.detail?.latestRunStatus || "pending",
    latestRunStartedAt: "",
    latestRunFinishedAt: "",
    activeRunCount: detailModal.value.detail?.activeRunCount || 0,
    canEdit: detailModal.value.detail?.canEdit ?? true,
  });
}

async function saveTaskEdit(payload: TaskGroup) {
  if (!editModal.value.task) return;

  editModal.value.saving = true;
  try {
    const updated = await TaskGroupAPI.updateTaskGroup(
      editModal.value.task.id,
      payload,
    );
    editModal.value.task = updated || payload;
    editModal.value.show = false;
    triggerToast("任务更新成功", "success");
    await loadTasks();

    if (
      detailModal.value.show &&
      detailModal.value.detail?.task.id === payload.id
    ) {
      detailModal.value.loading = true;
      detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(
        payload.id,
      );
      detailModal.value.loading = false;
    }
  } catch (err: any) {
    triggerToast(`保存失败: ${err?.message || err}`, "error");
    await loadTasks();

    if (
      detailModal.value.show &&
      detailModal.value.detail?.task.id === payload.id
    ) {
      try {
        detailModal.value.detail = await TaskGroupAPI.getTaskGroupDetail(
          payload.id,
        );
      } catch (detailErr) {
        logger.error('刷新任务详情失败', 'TaskExecution', detailErr);
      }
    }
  } finally {
    editModal.value.saving = false;
  }
}

// ================== 状态样式 ==================
function statusBorder(s: string) {
  switch (s) {
    case "running":
      return "border-accent/50";
    case "completed":
      return "border-success/50";
    case "failed":
    case "partial":
    case "cancelled":
      return "border-error/50";
    case "pending":
      return "border-warning/40";
    default:
      return "border-border";
  }
}
function statusBadge(s: string) {
  switch (s) {
    case "running":
      return "bg-accent/10 border-accent/30 text-accent";
    case "completed":
      return "bg-success/10 border-success/30 text-success";
    case "failed":
    case "partial":
    case "cancelled":
      return "bg-error/10 border-error/30 text-error";
    case "pending":
      return "bg-warning/10 border-warning/30 text-warning";
    default:
      return "bg-bg-panel border-border text-text-muted";
  }
}
function statusDot(s: string) {
  switch (s) {
    case "running":
      return "bg-accent animate-pulse";
    case "completed":
      return "bg-success";
    case "failed":
    case "partial":
    case "cancelled":
      return "bg-error";
    case "pending":
      return "bg-warning animate-pulse";
    default:
      return "bg-text-muted";
  }
}
function statusLabel(s: string) {
  const map: Record<string, string> = {
    pending: "等待中",
    running: "执行中",
    completed: "成功",
    partial: "部分完成",
    failed: "失败",
    cancelled: "已终止",
  };
  return map[s] ?? s;
}
// 任务状态样式
function taskStatusBadge(s: string) {
  switch (s) {
    case "pending":
      return "bg-bg-panel border-border text-text-muted";
    case "running":
      return "bg-accent/10 border-accent/30 text-accent";
    case "completed":
      return "bg-success/10 border-success/30 text-success";
    case "partial":
      return "bg-warning/10 border-warning/30 text-warning";
    case "failed":
    case "aborted":
      return "bg-error/10 border-error/30 text-error";
    case "cancelled":
      return "bg-bg-panel border-border text-text-muted";
    default:
      return "bg-bg-panel border-border text-text-muted";
  }
}
function taskStatusDot(s: string) {
  switch (s) {
    case "pending":
      return "bg-text-muted";
    case "running":
      return "bg-accent animate-pulse";
    case "completed":
      return "bg-success";
    case "partial":
      return "bg-warning";
    case "failed":
    case "aborted":
      return "bg-error";
    case "cancelled":
      return "bg-text-muted";
    default:
      return "bg-text-muted";
  }
}
function taskStatusLabel(s: string) {
  const map: Record<string, string> = {
    pending: "待执行",
    running: "执行中",
    completed: "已完成",
    partial: "部分成功",
    failed: "失败",
    cancelled: "已取消",
    aborted: "已中止",
  };
  return map[s] ?? s;
}

function formatDate(dateStr: string) {
  if (!dateStr) return "-";
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}
</script>

