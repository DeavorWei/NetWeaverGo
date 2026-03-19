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
          v-model="selectedTaskID"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary min-w-[260px]"
        >
          <option value="">选择发现任务</option>
          <option v-for="task in tasks" :key="task.id" :value="task.id">
            {{ task.name || task.id }} ({{ task.status }})
          </option>
        </select>
        <button
          @click="refreshGraph"
          :disabled="!selectedTaskID || building"
          class="px-4 py-2 rounded-lg text-sm font-medium border border-border text-text-secondary hover:text-text-primary"
        >
          刷新图谱
        </button>
        <button
          @click="buildTopology"
          :disabled="!selectedTaskID || building"
          class="px-4 py-2 rounded-lg text-sm font-semibold transition-all"
          :class="
            !selectedTaskID || building
              ? 'bg-bg-panel border border-border text-text-muted cursor-not-allowed'
              : 'bg-accent text-white border border-accent/40 hover:bg-accent-glow'
          "
        >
          {{ building ? "构建中..." : "构建拓扑" }}
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
          <option v-for="role in roleOptions" :key="role" :value="role">{{ role }}</option>
        </select>
        <select
          v-model="siteFilter"
          class="px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        >
          <option value="all">全部站点</option>
          <option v-for="site in siteOptions" :key="site" :value="site">{{ site }}</option>
        </select>
      </div>
    </div>

    <div class="grid grid-cols-5 gap-3">
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">可见链路</div>
        <div class="text-xl font-semibold text-text-primary mt-1">{{ filteredEdges.length }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">confirmed</div>
        <div class="text-xl font-semibold text-success mt-1">{{ countByStatus("confirmed") }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">semi_confirmed</div>
        <div class="text-xl font-semibold text-warning mt-1">{{ countByStatus("semi_confirmed") }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">inferred</div>
        <div class="text-xl font-semibold text-warning mt-1">{{ countByStatus("inferred") }}</div>
      </div>
      <div class="bg-bg-card border border-border rounded-lg p-3">
        <div class="text-xs text-text-muted">conflict</div>
        <div class="text-xl font-semibold text-error mt-1">{{ countByStatus("conflict") }}</div>
      </div>
    </div>

    <div class="grid grid-cols-3 gap-4 items-start">
      <div class="col-span-2 bg-bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-4 py-3 border-b border-border bg-bg-panel flex items-center justify-between">
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
                <td class="px-3 py-2 font-mono text-text-muted">{{ edge.sourceIf }}</td>
                <td class="px-3 py-2 text-text-primary">
                  <button
                    class="hover:text-accent"
                    @click.stop="openDeviceDetail(edge.target)"
                  >
                    {{ displayNodeLabel(edge.target) }}
                  </button>
                </td>
                <td class="px-3 py-2 font-mono text-text-muted">{{ edge.targetIf }}</td>
                <td class="px-3 py-2 text-text-secondary">{{ edge.edgeType }}</td>
                <td class="px-3 py-2">
                  <span class="px-2 py-0.5 rounded text-xs" :class="statusClass(edge.status)">
                    {{ edge.status }}
                  </span>
                </td>
              </tr>
              <tr v-if="filteredEdges.length === 0">
                <td colspan="6" class="px-3 py-8 text-center text-text-muted">暂无匹配的链路</td>
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
              {{ edgeDetail.aDevice.label || edgeDetail.aDevice.id }}:{{ edgeDetail.aIf }}
              <span class="text-text-muted"> ↔ </span>
              {{ edgeDetail.bDevice.label || edgeDetail.bDevice.id }}:{{ edgeDetail.bIf }}
            </div>
            <div class="text-xs text-text-muted">
              {{ edgeDetail.edgeType }} / {{ edgeDetail.status }} / confidence={{ edgeDetail.confidence.toFixed(2) }}
            </div>
            <div class="space-y-1 max-h-[220px] overflow-auto scrollbar-custom">
              <div
                v-for="(ev, idx) in edgeDetail.evidence"
                :key="idx"
                class="text-xs bg-bg-panel border border-border rounded px-2 py-1"
              >
                <div class="text-text-primary">{{ ev.type }} | {{ ev.summary || "-" }}</div>
                <div class="text-text-muted font-mono">
                  device={{ ev.deviceId }} cmd={{ ev.command }} raw={{ ev.rawRefId }}
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
          <div v-if="loadingDeviceDetail" class="p-4 text-xs text-text-muted">加载中...</div>
          <div v-else-if="deviceDetail" class="p-4 space-y-2 text-xs">
            <div class="text-text-primary font-semibold">
              {{ deviceDetail.identity?.hostname || displayNodeLabel(selectedDeviceID) }}
            </div>
            <div class="text-text-muted">IP: {{ deviceDetail.deviceIp }}</div>
            <div class="text-text-muted">厂商: {{ deviceDetail.identity?.vendor || "-" }}</div>
            <div class="text-text-muted">型号: {{ deviceDetail.identity?.model || "-" }}</div>
            <div class="text-text-muted">版本: {{ deviceDetail.identity?.version || "-" }}</div>
            <div class="pt-2 border-t border-border text-text-muted">
              接口 {{ deviceDetail.interfaces?.length || 0 }} |
              LLDP {{ deviceDetail.lldpNeighbors?.length || 0 }} |
              聚合 {{ deviceDetail.aggregates?.length || 0 }}
            </div>
          </div>
          <div v-else class="p-4 text-xs text-text-muted">点击设备名称查看详情</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  DiscoveryAPI,
  type DiscoveryTaskView,
  type ParsedResult,
  type TopologyBuildResult,
  type TopologyEdgeDetailView,
  type TopologyGraphView,
} from "../services/api";

const tasks = ref<DiscoveryTaskView[]>([]);
const selectedTaskID = ref("");
const building = ref(false);

const keyword = ref("");
const statusFilter = ref("all");
const viewMode = ref("all");
const roleFilter = ref("all");
const siteFilter = ref("all");

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

const filteredEdges = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return (graph.value.edges || []).filter((edge) => {
    if (statusFilter.value !== "all" && edge.status !== statusFilter.value) {
      return false;
    }

    if (viewMode.value === "logical" && edge.edgeType === "physical") {
      return false;
    }
    if (viewMode.value === "physical" && edge.edgeType === "logical_aggregate") {
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

function applySummaryFromGraph() {
  const edges = graph.value.edges || [];
  summary.value = {
    taskId: selectedTaskID.value,
    totalEdges: edges.length,
    confirmedEdges: edges.filter((e) => e.status === "confirmed").length,
    semiConfirmedEdges: edges.filter((e) => e.status === "semi_confirmed").length,
    inferredEdges: edges.filter((e) => e.status === "inferred").length,
    conflictEdges: edges.filter((e) => e.status === "conflict").length,
    buildTime: summary.value.buildTime,
    errors: summary.value.errors || [],
  };
}

async function loadTasks() {
  tasks.value = (await DiscoveryAPI.listDiscoveryTasks(50)) || [];
}

async function refreshGraph() {
  if (!selectedTaskID.value) return;
  edgeDetail.value = null;
  deviceDetail.value = null;
  const g = await DiscoveryAPI.getTopologyGraph(selectedTaskID.value);
  graph.value = g || { taskId: selectedTaskID.value, nodes: [], edges: [] };
  applySummaryFromGraph();
}

async function buildTopology() {
  if (!selectedTaskID.value || building.value) return;
  building.value = true;
  edgeDetail.value = null;
  deviceDetail.value = null;
  try {
    const result = await DiscoveryAPI.buildTopology(selectedTaskID.value);
    if (result) {
      summary.value = result;
    }
    await refreshGraph();
  } finally {
    building.value = false;
  }
}

async function loadEdgeDetail(edgeID: string) {
  if (!selectedTaskID.value) return;
  edgeDetail.value = await DiscoveryAPI.getEdgeDetail(selectedTaskID.value, edgeID);
}

async function openDeviceDetail(deviceID: string) {
  selectedDeviceID.value = deviceID;
  loadingDeviceDetail.value = true;
  try {
    if (!selectedTaskID.value || deviceID.startsWith("server:") || deviceID.startsWith("unknown:")) {
      deviceDetail.value = {
        taskId: selectedTaskID.value,
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
    deviceDetail.value = await DiscoveryAPI.getDeviceTopologyDetail(selectedTaskID.value, deviceID);
  } finally {
    loadingDeviceDetail.value = false;
  }
}

watch(selectedTaskID, (value) => {
  edgeDetail.value = null;
  deviceDetail.value = null;
  if (value) {
    void refreshGraph();
  } else {
    graph.value = { taskId: "", nodes: [], edges: [] };
    applySummaryFromGraph();
  }
});

onMounted(() => {
  void loadTasks();
});
</script>

