<template>
  <div class="space-y-5 animate-slide-in">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-text-primary">拓扑视图</h2>
        <p class="text-xs text-text-muted mt-1">
          支持搜索、状态过滤、逻辑/物理视图切换，并可查看链路证据与设备详情。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <select
          v-model="selectedRunId"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary min-w-[260px]"
        >
          <option value="">选择拓扑运行</option>
          <option
            v-for="run in topologyRuns"
            :key="run.runId"
            :value="run.runId"
          >
            {{ run.taskName || run.runId }} ({{
              StatusNames[run.status] || run.status
            }})
          </option>
        </select>
        <button
          @click="refreshGraph"
          :disabled="!selectedRunId || building"
          class="px-4 py-2 rounded-lg text-sm font-medium border border-border text-text-secondary hover:text-text-primary"
        >
          刷新图谱
        </button>
        <!-- 离线重放按钮 -->
        <button
          @click="openReplayDialog"
          :disabled="!selectedRunId"
          class="px-4 py-2 rounded-lg text-sm font-medium bg-accent text-white hover:bg-accent/90 disabled:opacity-50"
          title="从历史Raw文件重新解析构建拓扑"
        >
          <svg class="w-4 h-4 inline mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor">
            <path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          离线重放
        </button>
      </div>
    </div>

    <div class="bg-bg-card border border-border rounded-xl p-4">
      <div class="grid grid-cols-5 gap-3">
        <input
          v-model="keyword"
          placeholder="搜索设备/接口"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        />
        <select
          v-model="statusFilter"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="all">全部状态</option>
          <option value="confirmed">confirmed</option>
          <option value="semi_confirmed">semi_confirmed</option>
          <option value="inferred">inferred</option>
          <option value="conflict">conflict</option>
        </select>
        <select
          v-model="viewMode"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="all">全部链路</option>
          <option value="logical">逻辑视图</option>
          <option value="physical">物理视图</option>
        </select>
        <select
          v-model="roleFilter"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="all">全部角色</option>
          <option v-for="role in roleOptions" :key="role" :value="role">
            {{ role }}
          </option>
        </select>
        <select
          v-model="siteFilter"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="all">全部站点</option>
          <option v-for="site in siteOptions" :key="site" :value="site">
            {{ site }}
          </option>
        </select>
      </div>
    </div>

    <div class="grid grid-cols-5 gap-3">
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">可见链路</div>
        <div class="text-xl font-semibold text-text-primary mt-1">
          {{ filteredEdges.length }}
        </div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">confirmed</div>
        <div class="text-xl font-semibold text-success mt-1">
          {{ countByStatus("confirmed") }}
        </div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">semi_confirmed</div>
        <div class="text-xl font-semibold text-warning mt-1">
          {{ countByStatus("semi_confirmed") }}
        </div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">inferred</div>
        <div class="text-xl font-semibold text-warning mt-1">
          {{ countByStatus("inferred") }}
        </div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">conflict</div>
        <div class="text-xl font-semibold text-error mt-1">
          {{ countByStatus("conflict") }}
        </div>
      </div>
    </div>

    <!-- 视图切换 -->
    <div class="flex items-center gap-2">
      <button
        @click="viewType = 'graph'"
        :class="
          viewType === 'graph'
            ? 'bg-accent text-white'
            : 'bg-bg-panel text-text-secondary'
        "
        class="px-3 py-1.5 rounded-lg text-sm font-medium border border-border"
      >
        图形视图
      </button>
      <button
        @click="viewType = 'table'"
        :class="
          viewType === 'table'
            ? 'bg-accent text-white'
            : 'bg-bg-panel text-text-secondary'
        "
        class="px-3 py-1.5 rounded-lg text-sm font-medium border border-border"
      >
        表格视图
      </button>
    </div>

    <!-- 解析错误提示 -->
    <div
      v-if="summary.errors && summary.errors.length > 0"
      class="bg-error/10 border border-error/30 rounded-lg p-3"
    >
      <div class="flex items-center gap-2 text-sm text-error font-medium">
        <svg
          class="w-4 h-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
        >
          <circle cx="12" cy="12" r="10" stroke-width="2" />
          <line
            x1="12"
            y1="8"
            x2="12"
            y2="12"
            stroke-width="2"
            stroke-linecap="round"
          />
          <line
            x1="12"
            y1="16"
            x2="12.01"
            y2="16"
            stroke-width="2"
            stroke-linecap="round"
          />
        </svg>
        拓扑构建完成，但存在 {{ summary.errors.length }} 个数据问题
      </div>
      <div class="mt-2 space-y-1 max-h-[120px] overflow-auto scrollbar-custom">
        <div
          v-for="(err, idx) in summary.errors"
          :key="idx"
          class="text-xs text-error/80 font-mono bg-error/5 px-2 py-1 rounded"
        >
          {{ err }}
        </div>
      </div>
    </div>

    <div
      v-if="selectedRunId && graph.edges.length === 0"
      class="bg-warning/10 border border-warning/30 rounded-lg p-3 space-y-2"
    >
      <div class="text-sm text-warning font-medium">
        {{
          graph.nodes.length > 0
            ? `当前运行未发现任何拓扑链路，但已识别 ${graph.nodes.length} 个设备节点。图形视图会显示孤立节点，表格视图暂无链路记录。`
            : "当前运行未返回任何拓扑节点和链路，图形视图与表格视图都会为空。"
        }}
      </div>
      <ul class="text-xs text-text-muted list-disc pl-5 space-y-1">
        <li v-for="reason in emptyGraphReasons" :key="reason">{{ reason }}</li>
      </ul>
    </div>

    <!-- 图形视图 -->
    <div
      v-if="viewType === 'graph'"
      class="bg-bg-card border border-border rounded-xl overflow-hidden"
    >
      <div class="h-[60vh]">
        <TopologyGraph
          :nodes="graphNodes"
          :edges="graphEdges"
          @node-click="openDeviceDetail"
          @edge-click="loadEdgeDetail"
        />
      </div>
    </div>

    <!-- 表格视图 -->
    <div v-if="viewType === 'table'" class="grid grid-cols-3 gap-4 items-start">
      <div
        class="col-span-2 bg-bg-card border border-border rounded-xl overflow-hidden"
      >
        <div
          class="px-4 py-3 border-b border-border bg-bg-panel flex items-center justify-between"
        >
          <span class="text-sm font-medium text-text-primary">链路列表</span>
          <span class="text-xs text-text-muted"
            >节点 {{ graph.nodes.length }} / 边 {{ filteredEdges.length }}</span
          >
        </div>
        <div class="max-h-[58vh] overflow-auto scrollbar-custom">
          <table class="w-full text-sm">
            <thead class="bg-bg-panel text-text-secondary text-xs sticky top-0">
              <tr>
                <th class="text-left px-3 py-2">源设备</th>
                <th class="text-left px-3 py-2">源接口</th>
                <th class="text-left px-3 py-2">目标设备</th>
                <th class="text-left px-3 py-2">目标接口</th>
                <th class="text-left px-3 py-2">类型</th>
                <th class="text-left px-3 py-2">状态</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr
                v-for="edge in filteredEdges"
                :key="edge.id"
                class="hover:bg-bg-hover cursor-pointer"
                @click="loadEdgeDetail(edge.id)"
              >
                <td class="px-3 py-2 text-text-primary">
                  <button
                    class="hover:text-accent"
                    @click.stop="openDeviceDetail(edge.source)"
                  >
                    {{ displayNodeLabel(edge.source) }}
                  </button>
                </td>
                <td class="px-3 py-2 font-mono text-text-muted">
                  {{ edge.sourceIf }}
                </td>
                <td class="px-3 py-2 text-text-primary">
                  <button
                    class="hover:text-accent"
                    @click.stop="openDeviceDetail(edge.target)"
                  >
                    {{ displayNodeLabel(edge.target) }}
                  </button>
                </td>
                <td class="px-3 py-2 font-mono text-text-muted">
                  {{ edge.targetIf }}
                </td>
                <td class="px-3 py-2 text-text-secondary">
                  {{ edge.edgeType }}
                </td>
                <td class="px-3 py-2">
                  <span
                    class="px-2 py-0.5 rounded text-xs"
                    :class="statusClass(edge.status)"
                  >
                    {{ edge.status }}
                  </span>
                </td>
              </tr>
              <tr v-if="filteredEdges.length === 0">
                <td colspan="6" class="px-3 py-8 text-center text-text-muted">
                  暂无匹配的链路
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="space-y-4">
        <div class="bg-bg-card border border-border rounded-xl overflow-hidden">
          <div class="px-4 py-3 border-b border-border bg-bg-panel">
            <span class="text-sm font-medium text-text-primary">链路证据</span>
          </div>
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
        </div>

        <div class="bg-bg-card border border-border rounded-xl overflow-hidden">
          <div class="px-4 py-3 border-b border-border bg-bg-panel">
            <span class="text-sm font-medium text-text-primary">设备详情</span>
          </div>
          <div v-if="loadingDeviceDetail" class="p-4 text-xs text-text-muted">
            加载中...
          </div>
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
                版本: {{ deviceDetail.identity?.version || "-" }}
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
        </div>
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

function statusClass(status: string) {
  const map: Record<string, string> = {
    confirmed: "bg-success/20 text-success",
    semi_confirmed: "bg-warning/20 text-warning",
    inferred: "bg-warning/20 text-warning",
    conflict: "bg-error/20 text-error",
  };
  return map[status] || "bg-text-muted/20 text-text-muted";
}

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
  console.debug("[Topology] 开始刷新拓扑图", {
    runId: selectedRunId.value,
    status: selectedRun.value?.status,
    taskName: selectedRun.value?.taskName,
  });
  const g = await TaskExecutionAPI.getTopologyGraph(selectedRunId.value);
  graph.value = g || { taskId: selectedRunId.value, nodes: [], edges: [] };
  lastGraphLoadAt.value = startedAt.toLocaleString();
  applySummaryFromGraph();
  console.debug("[Topology] 拓扑图刷新完成", {
    runId: selectedRunId.value,
    nodeCount: graph.value.nodes?.length || 0,
    edgeCount: graph.value.edges?.length || 0,
    loadedAt: lastGraphLoadAt.value,
  });
  if ((graph.value.edges?.length || 0) === 0) {
    console.warn("[Topology] 当前运行未返回任何拓扑边", {
      runId: selectedRunId.value,
      nodeCount: graph.value.nodes?.length || 0,
      edgeCount: graph.value.edges?.length || 0,
      status: selectedRun.value?.status,
      reasons: emptyGraphReasons.value,
    });
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
  console.debug("[Topology] 切换拓扑运行", {
    runId: value,
    taskName: selectedRun.value?.taskName,
    status: selectedRun.value?.status,
  });
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
