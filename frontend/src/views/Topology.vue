<template>
  <div class="space-y-5 animate-slide-in h-full flex flex-col">
    <div class="flex items-center justify-between gap-3 flex-shrink-0">
      <div>
        <h2 class="text-lg font-semibold text-text-primary">拓扑视图</h2>
        <p class="text-xs text-text-muted mt-1">
          支持搜索、状态过滤、逻辑/物理视图切换，并可查看链路证据与设备详情。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <el-select v-model="selectedRunId" placeholder="选择拓扑运行" style="width: 300px" clearable>
          <el-option
            v-for="run in topologyRuns"
            :key="run.runId"
            :label="`${run.taskName || run.runId} (${StatusNames[run.status] || run.status})`"
            :value="run.runId"
          />
        </el-select>
        <el-button
          :icon="RefreshRight"
          @click="refreshGraph"
          :disabled="!selectedRunId || building"
        >
          刷新图谱
        </el-button>
        <!-- 离线重放按钮 -->
        <el-button
          type="primary"
          :icon="VideoPlay"
          @click="openReplayDialog"
          :disabled="!selectedRunId"
          title="从历史Raw文件重新解析构建拓扑"
        >
          离线重放
        </el-button>
      </div>
    </div>

    <el-card shadow="never" :body-style="{ padding: '16px' }" class="flex-shrink-0">
      <div class="grid grid-cols-5 gap-3">
        <el-input
          v-model="keyword"
          placeholder="搜索设备/接口"
          :prefix-icon="Search"
          clearable
        />
        <el-select v-model="statusFilter" placeholder="全部状态">
          <el-option label="全部状态" value="all" />
          <el-option label="confirmed" value="confirmed" />
          <el-option label="semi_confirmed" value="semi_confirmed" />
          <el-option label="inferred" value="inferred" />
          <el-option label="conflict" value="conflict" />
        </el-select>
        <el-select v-model="viewMode" placeholder="全部链路">
          <el-option label="全部链路" value="all" />
          <el-option label="逻辑视图" value="logical" />
          <el-option label="物理视图" value="physical" />
        </el-select>
        <el-select v-model="roleFilter" placeholder="全部角色">
          <el-option label="全部角色" value="all" />
          <el-option v-for="role in roleOptions" :key="role" :label="role" :value="role" />
        </el-select>
        <el-select v-model="siteFilter" placeholder="全部站点">
          <el-option label="全部站点" value="all" />
          <el-option v-for="site in siteOptions" :key="site" :label="site" :value="site" />
        </el-select>
      </div>
    </el-card>

    <div class="grid grid-cols-5 gap-3 flex-shrink-0">
      <el-card shadow="never" :body-style="{ padding: '12px' }">
        <div class="text-xs text-text-muted">可见链路</div>
        <div class="text-xl font-semibold mt-1">
          {{ filteredEdges.length }}
        </div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '12px' }">
        <div class="text-xs text-text-muted">confirmed</div>
        <div class="text-xl font-semibold text-success mt-1">
          {{ countByStatus("confirmed") }}
        </div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '12px' }">
        <div class="text-xs text-text-muted">semi_confirmed</div>
        <div class="text-xl font-semibold text-warning mt-1">
          {{ countByStatus("semi_confirmed") }}
        </div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '12px' }">
        <div class="text-xs text-text-muted">inferred</div>
        <div class="text-xl font-semibold text-warning mt-1">
          {{ countByStatus("inferred") }}
        </div>
      </el-card>
      <el-card shadow="never" :body-style="{ padding: '12px' }">
        <div class="text-xs text-text-muted">conflict</div>
        <div class="text-xl font-semibold text-error mt-1">
          {{ countByStatus("conflict") }}
        </div>
      </el-card>
    </div>

    <!-- 视图切换 -->
    <div class="flex items-center gap-2 flex-shrink-0">
      <el-radio-group v-model="viewType">
        <el-radio-button label="graph">图形视图</el-radio-button>
        <el-radio-button label="table">表格视图</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 解析错误提示 -->
    <el-alert
      v-if="summary.errors && summary.errors.length > 0"
      type="error"
      :closable="false"
      class="flex-shrink-0"
      :title="`拓扑构建完成，但存在 ${summary.errors.length} 个数据问题`"
    >
      <div class="mt-2 space-y-1 max-h-[120px] overflow-auto scrollbar-custom">
        <div
          v-for="(err, idx) in summary.errors"
          :key="idx"
          class="text-xs font-mono bg-error/10 px-2 py-1 rounded text-error"
        >
          {{ err }}
        </div>
      </div>
    </el-alert>

    <el-alert
      v-if="selectedRunId && graph.edges.length === 0"
      type="warning"
      :closable="false"
      class="flex-shrink-0"
    >
      <div class="text-sm font-medium">
        {{
          graph.nodes.length > 0
            ? `当前运行未发现任何拓扑链路，但已识别 ${graph.nodes.length} 个设备节点。图形视图会显示孤立节点，表格视图暂无链路记录。`
            : "当前运行未返回任何拓扑节点和链路，图形视图与表格视图都会为空。"
        }}
      </div>
      <ul class="text-xs list-disc pl-5 space-y-1 mt-2">
        <li v-for="reason in emptyGraphReasons" :key="reason">{{ reason }}</li>
      </ul>
    </el-alert>

    <!-- 图形视图 -->
    <div
      v-if="viewType === 'graph'"
      class="flex-1 min-h-0 bg-bg-card border border-border rounded-xl overflow-hidden relative"
    >
      <div class="absolute inset-0">
        <TopologyGraph
          :nodes="graphNodes"
          :edges="graphEdges"
          @node-click="openDeviceDetail"
          @edge-click="loadEdgeDetail"
        />
      </div>
    </div>

    <!-- 表格视图 -->
    <div v-if="viewType === 'table'" class="flex-1 min-h-0 grid grid-cols-3 gap-4 items-start overflow-hidden">
      <el-card class="col-span-2 h-full flex flex-col" shadow="never" :body-style="{ padding: '0', flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">链路列表</span>
            <span class="text-xs text-text-muted">节点 {{ graph.nodes.length }} / 边 {{ filteredEdges.length }}</span>
          </div>
        </template>
        <el-table
          :data="filteredEdges"
          style="width: 100%; height: 100%"
          height="100%"
          highlight-current-row
          @row-click="(row: any) => loadEdgeDetail(row.id)"
        >
          <el-table-column label="源设备" min-width="120">
            <template #default="{ row }">
              <el-button link type="primary" @click.stop="openDeviceDetail(row.source)">
                {{ displayNodeLabel(row.source) }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sourceIf" label="源接口" min-width="100" />
          <el-table-column label="目标设备" min-width="120">
            <template #default="{ row }">
              <el-button link type="primary" @click.stop="openDeviceDetail(row.target)">
                {{ displayNodeLabel(row.target) }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="targetIf" label="目标接口" min-width="100" />
          <el-table-column prop="edgeType" label="类型" width="100" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'confirmed' ? 'success' : (row.status === 'conflict' ? 'danger' : 'warning')">
                {{ row.status }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <div class="space-y-4 h-full overflow-auto pr-2 scrollbar-custom">
        <el-card shadow="never" :body-style="{ padding: '0' }">
          <template #header>
            <span class="text-sm font-medium">链路证据</span>
          </template>
          <div v-if="edgeDetail" class="p-4 space-y-2 text-sm">
            <div class="text-text-primary">
              {{ edgeDetail.aDevice.label || edgeDetail.aDevice.id }}:{{
                edgeDetail.aIf
              }}
              <span class="text-text-muted"> ↔ </span>
              {{ edgeDetail.bDevice.label || edgeDetail.bDevice.id }}:{{
                edgeDetail.bIf
              }}
            </div>
            <div class="text-xs text-text-muted">
              {{ edgeDetail.edgeType }} / {{ edgeDetail.status }} /
              confidence={{ edgeDetail.confidence.toFixed(2) }}
            </div>
            <div class="space-y-1 max-h-[220px] overflow-auto scrollbar-custom">
              <div
                v-for="(ev, idx) in edgeDetail.evidence"
                :key="idx"
                class="text-xs bg-bg-panel border border-border rounded px-2 py-1"
              >
                <div class="text-text-primary">
                  {{ ev.type }} | {{ ev.summary || "-" }}
                </div>
                <div class="text-text-muted font-mono">
                  device={{ ev.deviceId }} cmd={{ ev.command }} raw={{
                    ev.rawRefId
                  }}
                </div>
                <!-- 互联详情：展示远端设备信息 -->
                <div
                  v-if="ev.remoteName || ev.remoteIf || ev.remoteMac || ev.remoteIp"
                  class="text-text-muted font-mono mt-1 pt-1 border-t border-border"
                >
                  <span v-if="ev.remoteName" class="mr-2">远端设备: {{ ev.remoteName }}</span>
                  <span v-if="ev.remoteIf" class="mr-2">接口: {{ ev.remoteIf }}</span>
                  <span v-if="ev.remoteMac" class="mr-2">MAC: {{ ev.remoteMac }}</span>
                  <span v-if="ev.remoteIp">IP: {{ ev.remoteIp }}</span>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="p-4 text-xs text-text-muted">点击链路查看证据</div>
        </el-card>

        <el-card shadow="never" :body-style="{ padding: '0' }">
          <template #header>
            <span class="text-sm font-medium">设备详情</span>
          </template>
          <div v-if="loadingDeviceDetail" class="p-4 text-xs text-text-muted" v-loading="true"></div>
          <div v-else-if="deviceDetail" class="p-4 space-y-2 text-xs">
            <div class="text-text-primary font-semibold">
              {{
                deviceDetail.identity?.hostname ||
                displayNodeLabel(selectedDeviceID)
              }}
            </div>
            <div class="text-text-muted">IP: {{ deviceDetail.deviceIp }}</div>
            <!-- 推断节点显示MAC信息 -->
            <div v-if="isInferredNode(nodeByID.get(selectedDeviceID))" class="space-y-1">
              <div class="text-warning font-medium">推断节点</div>
              <div v-if="nodeByID.get(selectedDeviceID)?.macAddress" class="text-text-muted">
                MAC: {{ nodeByID.get(selectedDeviceID)?.macAddress }}
              </div>
              <div v-if="(nodeByID.get(selectedDeviceID)?.macAddresses?.length ?? 0) > 1" class="text-text-muted">
                多MAC: {{ nodeByID.get(selectedDeviceID)?.macAddresses?.join(', ') }}
              </div>
              <div class="text-text-muted text-xs italic">
                此设备通过FDB/ARP推断，未直接采集
              </div>
            </div>
            <template v-else>
              <div class="text-text-muted">
                厂商: {{ deviceDetail.identity?.vendor || "-" }}
              </div>
              <div class="text-text-muted">
                型号: {{ deviceDetail.identity?.model || "-" }}
              </div>
              <div class="text-text-muted">
                主机名: {{ deviceDetail.identity?.hostname || "-" }}
              </div>
              <div class="pt-2 border-t border-border text-text-muted">
                接口 {{ deviceDetail.interfaces?.length || 0 }} | LLDP
                {{ deviceDetail.lldpNeighbors?.length || 0 }} | 聚合
                {{ deviceDetail.aggregates?.length || 0 }}
              </div>
            </template>
          </div>
          <div v-else class="p-4 text-xs text-text-muted">
            点击设备名称查看详情
          </div>
        </el-card>
      </div>
    </div>
  </div>

  <!-- 离线重放对话框 -->
  <ReplayDialog
    :visible="showReplayDialog"
    :original-run-id="selectedRunId"
    :run-info="replayRunInfo"
    @close="showReplayDialog = false"
    @complete="handleReplayComplete"
  />

  <!-- 设备详情弹窗 -->
  <TopologyDeviceDetailModal
    v-model:show="showDeviceModal"
    :loading="loadingDeviceDetail"
    :device-detail="deviceDetail"
    :node-info="nodeByID.get(selectedDeviceID)"
    :device-id="selectedDeviceID"
  />

  <!-- 链路详情弹窗 -->
  <TopologyEdgeDetailModal
    v-model:show="showEdgeModal"
    :edge-detail="edgeDetail"
    @device-click="openDeviceDetail"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Search, RefreshRight, VideoPlay } from '@element-plus/icons-vue';
import {
  TaskExecutionAPI,
  type ParsedResult,
  type TopologyBuildResult,
  type TopologyEdgeDetailView,
  type TopologyGraphView,
} from "../services/api";
import { useTaskexecStore } from "../stores/taskexecStore";
import { StatusNames, type ReplayableRunInfo } from "../types/taskexec";
import TopologyGraph, { type GraphNode } from "../components/topology/TopologyGraph.vue";
import ReplayDialog from "../components/topology/ReplayDialog.vue";
import TopologyDeviceDetailModal from "../components/topology/TopologyDeviceDetailModal.vue";
import TopologyEdgeDetailModal from "../components/topology/TopologyEdgeDetailModal.vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

// 阶段4: 统一执行框架 - 使用runId替代taskId
const taskexecStore = useTaskexecStore();
const selectedRunId = ref("");
const lastGraphLoadAt = ref<string>("");

// 离线重放对话框状态
const showReplayDialog = ref(false);
const replayRunInfo = ref<ReplayableRunInfo | null>(null);

// 计算属性：筛选拓扑类型的运行
const topologyRuns = computed(() => {
  return taskexecStore.runHistory.filter((r) => r.runKind === "topology");
});
const selectedRun = computed(
  () =>
    topologyRuns.value.find((run) => run.runId === selectedRunId.value) || null,
);
const building = ref(false);

const keyword = ref("");
const statusFilter = ref("all");
const viewMode = ref("all");
const roleFilter = ref("all");
const siteFilter = ref("all");
const viewType = ref<"graph" | "table">("graph");

const summary = ref<TopologyBuildResult>({
  taskId: "",
  totalEdges: 0,
  confirmedEdges: 0,
  semiConfirmedEdges: 0,
  inferredEdges: 0,
  conflictEdges: 0,
  buildTime: 0,
  errors: [],
});

const graph = ref<TopologyGraphView>({
  taskId: "",
  nodes: [],
  edges: [],
});

const edgeDetail = ref<TopologyEdgeDetailView | null>(null);
const deviceDetail = ref<ParsedResult | null>(null);
const selectedDeviceID = ref("");
const loadingDeviceDetail = ref(false);

// 图形视图弹窗状态
const showDeviceModal = ref(false);
const showEdgeModal = ref(false);

const nodeByID = computed(() => {
  const map = new Map<string, (typeof graph.value.nodes)[number]>();
  for (const node of graph.value.nodes || []) {
    map.set(node.id, node);
  }
  return map;
});

const roleOptions = computed(() => {
  const values = new Set<string>();
  for (const node of graph.value.nodes || []) {
    if (node.role) {
      values.add(node.role);
    }
  }
  return Array.from(values).sort();
});

const siteOptions = computed(() => {
  const values = new Set<string>();
  for (const node of graph.value.nodes || []) {
    if (node.site) {
      values.add(node.site);
    }
  }
  return Array.from(values).sort();
});

// 转换数据格式供图形组件使用
const graphNodes = computed(() =>
  (graph.value.nodes || []).map((n) => ({
    id: n.id,
    label: n.label || n.id,
    role: n.role,
    site: n.site,
    ip: n.ip,
    nodeType: n.nodeType,
    macAddress: n.macAddress,
    macAddresses: n.macAddresses,
    vendor: n.vendor,
  })),
);

const graphEdges = computed(() =>
  (graph.value.edges || []).map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    sourceIf: e.sourceIf,
    targetIf: e.targetIf,
    status: e.status,
    edgeType: e.edgeType,
  })),
);

const filteredEdges = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return (graph.value.edges || []).filter((edge) => {
    if (statusFilter.value !== "all" && edge.status !== statusFilter.value) {
      return false;
    }

    if (viewMode.value === "logical" && edge.edgeType === "physical") {
      return false;
    }
    if (
      viewMode.value === "physical" &&
      edge.edgeType === "logical_aggregate"
    ) {
      return false;
    }

    const sourceNode = nodeByID.value.get(edge.source);
    const targetNode = nodeByID.value.get(edge.target);

    if (roleFilter.value !== "all") {
      const sourceRole = sourceNode?.role || "";
      const targetRole = targetNode?.role || "";
      if (sourceRole !== roleFilter.value && targetRole !== roleFilter.value) {
        return false;
      }
    }

    if (siteFilter.value !== "all") {
      const sourceSite = sourceNode?.site || "";
      const targetSite = targetNode?.site || "";
      if (sourceSite !== siteFilter.value && targetSite !== siteFilter.value) {
        return false;
      }
    }

    if (!kw) {
      return true;
    }

    const text = [
      edge.source,
      edge.target,
      edge.sourceIf,
      edge.targetIf,
      edge.edgeType,
      edge.status,
      sourceNode?.label || "",
      targetNode?.label || "",
    ]
      .join(" ")
      .toLowerCase();
    return text.includes(kw);
  });
});

const emptyGraphReasons = computed(() => {
  if (!selectedRunId.value) {
    return ["请先选择一个拓扑运行记录"];
  }

  const reasons: string[] = [];
  const nodeCount = graph.value.nodes?.length || 0;
  const edgeCount = graph.value.edges?.length || 0;

  reasons.push(`当前运行ID: ${selectedRunId.value}`);
  reasons.push(`当前返回节点数: ${nodeCount}`);
  reasons.push(`当前返回链路数: ${edgeCount}`);

  if (selectedRun.value) {
    reasons.push(
      `运行状态: ${StatusNames[selectedRun.value.status] || selectedRun.value.status}`,
    );
  }

  if (nodeCount > 0) {
    reasons.push(
      "后端已经返回设备节点，说明前端渲染链路并未失败；当前问题是未发现可用链路证据",
    );
    if (nodeCount === 1) {
      reasons.push(
        "单设备采集且没有 LLDP 邻居时，仅显示一个孤立设备节点属于预期表现",
      );
    } else {
      reasons.push(
        "如果本次本应存在链路，请继续检查 LLDP/FDB/ARP 解析结果与构图规则是否生成了边",
      );
    }
  } else {
    reasons.push(
      "后端返回的拓扑节点和链路都为 0，问题通常发生在采集、解析或构图阶段，而不是前端渲染阶段",
    );
  }

  reasons.push(
    "请重点查看后端 verbose 日志中的“拓扑采集设备画像解析 / 解析汇总 / 拓扑构建结果 / 查询拓扑图无边结果”等关键字",
  );
  reasons.push(
    "华为设备必须采集到可被 TextFSM 模板识别的 LLDP verbose 输出，否则无法生成任何链路",
  );

  if (lastGraphLoadAt.value) {
    reasons.push(`最近一次刷新时间: ${lastGraphLoadAt.value}`);
  }

  return reasons;
});


function countByStatus(status: string) {
  return filteredEdges.value.filter((edge) => edge.status === status).length;
}

function displayNodeLabel(nodeID: string) {
  const node = nodeByID.value.get(nodeID);
  return node?.label || nodeID;
}

// 判断是否为推断节点
function isInferredNode(node: GraphNode | undefined): boolean {
  return node?.nodeType === 'inferred' || node?.nodeType === 'unknown';
}

function applySummaryFromGraph() {
  const edges = graph.value.edges || [];
  summary.value = {
    taskId: selectedRunId.value,
    totalEdges: edges.length,
    confirmedEdges: edges.filter((e) => e.status === "confirmed").length,
    semiConfirmedEdges: edges.filter((e) => e.status === "semi_confirmed")
      .length,
    inferredEdges: edges.filter((e) => e.status === "inferred").length,
    conflictEdges: edges.filter((e) => e.status === "conflict").length,
    buildTime: summary.value.buildTime,
    errors: summary.value.errors || [],
  };
}

// 阶段4: 加载拓扑运行列表（从统一运行时）
async function loadRuns() {
  await taskexecStore.loadRunHistory(50);
}

async function loadTasks() {
  await loadRuns();
}

async function refreshGraph() {
  if (!selectedRunId.value) return;
  edgeDetail.value = null;
  deviceDetail.value = null;
  const startedAt = new Date();
  logger.debug(
    `开始刷新拓扑图，runId=${selectedRunId.value}, status=${selectedRun.value?.status}`,
    'Topology',
  );
  const g = await TaskExecutionAPI.getTopologyGraph(selectedRunId.value);
  graph.value = g || { taskId: selectedRunId.value, nodes: [], edges: [] };
  lastGraphLoadAt.value = startedAt.toLocaleString();
  applySummaryFromGraph();
  logger.debug(
    `拓扑图刷新完成，nodes=${graph.value.nodes?.length || 0}, edges=${graph.value.edges?.length || 0}`,
    'Topology',
  );
  if ((graph.value.edges?.length || 0) === 0) {
    logger.warn(
      `当前运行未返回任何拓扑边，runId=${selectedRunId.value}, reasons=${emptyGraphReasons.value.join(', ')}`,
      'Topology',
    );
  }
}

async function loadEdgeDetail(edgeID: string) {
  if (!selectedRunId.value) return;
  showEdgeModal.value = true;
  edgeDetail.value = await TaskExecutionAPI.getTopologyEdgeDetail(
    selectedRunId.value,
    edgeID,
  );
}

async function openDeviceDetail(deviceID: string) {
  selectedDeviceID.value = deviceID;
  loadingDeviceDetail.value = true;
  showDeviceModal.value = true;
  try {
    if (
      !selectedRunId.value ||
      deviceID.startsWith("server:") ||
      deviceID.startsWith("unknown:")
    ) {
      deviceDetail.value = {
        taskId: selectedRunId.value,
        deviceIp: deviceID,
        parsedAt: new Date() as any,
        identity: {
          vendor: "inferred",
          hostname: displayNodeLabel(deviceID),
        } as any,
        interfaces: [],
        lldpNeighbors: [],
        fdbEntries: [],
        arpEntries: [],
        aggregates: [],
      } as ParsedResult;
      return;
    }
    deviceDetail.value = await TaskExecutionAPI.getTopologyDeviceDetail(
      selectedRunId.value,
      deviceID,
    );
  } finally {
    loadingDeviceDetail.value = false;
  }
}

watch(selectedRunId, (value) => {
  logger.debug(
    `切换拓扑运行，runId=${value}, taskName=${selectedRun.value?.taskName}`,
    'Topology',
  );
  edgeDetail.value = null;
  deviceDetail.value = null;
  if (value) {
    void refreshGraph();
  } else {
    graph.value = { taskId: "", nodes: [], edges: [] };
    lastGraphLoadAt.value = "";
    applySummaryFromGraph();
  }
});

onMounted(() => {
  void loadTasks();
});

// 离线重放功能
function openReplayDialog() {
  if (!selectedRunId.value) return;
  
  // 构建可重放运行信息
  const run = selectedRun.value;
  if (run) {
    replayRunInfo.value = {
      runId: run.runId,
      taskName: run.taskName || run.runId,
      status: run.status,
      runKind: run.runKind,
      deviceCount: 0, // 将由对话框加载
      rawFileCount: 0, // 将由对话框加载
      createdAt: run.startedAt || new Date().toISOString(),
      hasRawFiles: true // 假设已选择运行都有Raw文件
    };
  }
  showReplayDialog.value = true;
}

function handleReplayComplete(result: { replayRunId: string }) {
  showReplayDialog.value = false;
  // 切换到新的重放运行ID
  selectedRunId.value = result.replayRunId;
  // 刷新历史记录
  void loadTasks();
}
</script>
