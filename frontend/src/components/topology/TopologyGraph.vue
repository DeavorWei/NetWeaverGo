<template>
  <div class="topology-graph-container">
    <div ref="cyContainer" class="cy-container"></div>

    <!-- 图例 -->
    <div
      class="absolute bottom-4 left-4 bg-bg-card border border-border rounded-lg p-3 space-y-2"
    >
      <div class="text-xs font-medium text-text-secondary">链路状态</div>
      <div class="flex items-center gap-2">
        <span class="w-4 h-0.5 bg-success"></span>
        <span class="text-xs text-text-muted">confirmed</span>
      </div>
      <div class="flex items-center gap-2">
        <span class="w-4 h-0.5 bg-warning"></span>
        <span class="text-xs text-text-muted">semi_confirmed</span>
      </div>
      <div class="flex items-center gap-2">
        <span
          class="w-4 h-0.5 bg-warning border-dashed border-t border-warning"
        ></span>
        <span class="text-xs text-text-muted">inferred</span>
      </div>
      <div class="flex items-center gap-2">
        <span class="w-4 h-0.5 bg-error"></span>
        <span class="text-xs text-text-muted">conflict</span>
      </div>
    </div>

    <!-- 控制按钮 -->
    <div class="absolute top-4 right-4 flex flex-col gap-2">
      <button
        @click="fitToScreen"
        class="p-2 bg-bg-card border border-border rounded-lg hover:bg-bg-hover"
        title="适应屏幕"
      >
        <svg
          class="w-4 h-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
        >
          <path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7" />
        </svg>
      </button>
      <button
        @click="resetLayout"
        class="p-2 bg-bg-card border border-border rounded-lg hover:bg-bg-hover"
        title="重新布局"
      >
        <svg
          class="w-4 h-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
        >
          <circle cx="12" cy="12" r="3" />
          <path
            d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"
          />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import cytoscape from "cytoscape";
import type { Core, NodeSingular, EdgeSingular } from "cytoscape";
// @ts-expect-error cytoscape-dagre 没有类型定义
import dagre from "cytoscape-dagre";

cytoscape.use(dagre);

export interface GraphNode {
  id: string;
  label: string;
  role?: string;
  site?: string;
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  sourceIf: string;
  targetIf: string;
  status: string;
  edgeType: string;
}

const props = defineProps<{
  nodes: GraphNode[];
  edges: GraphEdge[];
}>();

const emit = defineEmits<{
  nodeClick: [nodeId: string];
  edgeClick: [edgeId: string];
}>();

const cyContainer = ref<HTMLDivElement>();
let cy: Core | null = null;

const statusColors: Record<string, string> = {
  confirmed: "#22c55e",
  semi_confirmed: "#f59e0b",
  inferred: "#f59e0b",
  conflict: "#ef4444",
};

const roleColors: Record<string, string> = {
  core: "#3b82f6",
  aggregation: "#8b5cf6",
  access: "#06b6d4",
  firewall: "#ef4444",
  server: "#6b7280",
};

function initGraph() {
  if (!cyContainer.value) return;

  const elements: cytoscape.ElementDefinition[] = [
    ...props.nodes.map((n) => ({
      data: {
        id: n.id,
        label: n.label || n.id,
        role: n.role,
      },
    })),
    ...props.edges.map((e) => ({
      data: {
        id: e.id,
        source: e.source,
        target: e.target,
        status: e.status,
        edgeType: e.edgeType,
        sourceIf: e.sourceIf,
        targetIf: e.targetIf,
      },
    })),
  ];

  cy = cytoscape({
    container: cyContainer.value,
    elements,
    style: [
      {
        selector: "node",
        style: {
          "background-color": (ele: NodeSingular) =>
            roleColors[ele.data("role")] || "#3b82f6",
          label: "data(label)",
          width: 50,
          height: 50,
          "font-size": "11px",
          "text-valign": "bottom",
          "text-margin-y": 6,
          color: "#e5e7eb",
          "border-width": 2,
          "border-color": "#374151",
        },
      },
      {
        selector: "node:selected",
        style: {
          "border-color": "#3b82f6",
          "border-width": 3,
        },
      },
      {
        selector: "edge",
        style: {
          width: 2,
          "line-color": (ele: EdgeSingular) =>
            statusColors[ele.data("status")] || "#6b7280",
          "target-arrow-color": (ele: EdgeSingular) =>
            statusColors[ele.data("status")] || "#6b7280",
          "target-arrow-shape": "triangle",
          "curve-style": "bezier",
          label: (ele: EdgeSingular) =>
            `${ele.data("sourceIf")} ↔ ${ele.data("targetIf")}`,
          "font-size": "9px",
          "text-rotation": "autorotate",
          "text-margin-y": -10,
          color: "#9ca3af",
          "text-opacity": 0.8,
        },
      },
      {
        selector: 'edge[status="inferred"]',
        style: {
          "line-style": "dashed",
        },
      },
      {
        selector: "edge:selected",
        style: {
          width: 3,
          "line-color": "#3b82f6",
          "target-arrow-color": "#3b82f6",
        },
      },
    ],
    layout: {
      name: "dagre",
      rankDir: "TB",
      nodeSep: 100,
      edgeSep: 50,
      rankSep: 120,
      animate: true,
      animationDuration: 300,
    } as cytoscape.LayoutOptions,
    minZoom: 0.3,
    maxZoom: 2,
    wheelSensitivity: 0.3,
  });

  // 节点点击事件
  cy.on("tap", "node", (evt) => {
    const node = evt.target;
    emit("nodeClick", node.id());
  });

  // 边点击事件
  cy.on("tap", "edge", (evt) => {
    const edge = evt.target;
    emit("edgeClick", edge.id());
  });
}

function fitToScreen() {
  cy?.fit(undefined, 50);
}

function resetLayout() {
  cy?.layout({
    name: "dagre",
    rankDir: "TB",
    animate: true,
    animationDuration: 300,
  } as cytoscape.LayoutOptions).run();
}

onMounted(() => {
  initGraph();
});

onUnmounted(() => {
  cy?.destroy();
});

watch(
  () => [props.nodes, props.edges],
  () => {
    cy?.destroy();
    initGraph();
  },
  { deep: true },
);

defineExpose({
  fitToScreen,
  resetLayout,
});
</script>

<style scoped>
.topology-graph-container {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 500px;
  background: var(--bg-panel);
  border-radius: 0.5rem;
}

.cy-container {
  width: 100%;
  height: 100%;
  min-height: 500px;
}
</style>
