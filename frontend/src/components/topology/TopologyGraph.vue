<template>
  <div class="topology-graph-container" @contextmenu.prevent>
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
      <!-- 布局选择器 -->
      <select
        v-model="currentLayout"
        @change="onLayoutChange"
        class="p-2 bg-bg-card border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-primary shadow-sm hover:bg-bg-hover cursor-pointer"
        title="切换布局方式"
      >
        <option value="dagre">层级排版</option>
        <option value="cose">物理引力</option>
        <option value="concentric">同心圆</option>
        <option value="circle">环形排列</option>
        <option value="grid">网格对齐</option>
      </select>

      <button
        v-if="rootNodeId"
        @click="clearRoot"
        class="flex items-center gap-2 px-3 py-2 bg-bg-card border border-border rounded-lg hover:bg-bg-hover text-primary transition-colors text-sm font-medium"
        title="取消根节点 (恢复默认布局)"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4M16 17l5-5-5-5M21 12H9" />
        </svg>
        <span>取消根节点</span>
      </button>
      <button
        @click="fitToScreen"
        class="flex items-center gap-2 px-3 py-2 bg-bg-card border border-border rounded-lg hover:bg-bg-hover text-text-primary transition-colors text-sm font-medium"
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
        <span>适应屏幕</span>
      </button>
      <button
        @click="resetLayout"
        class="flex items-center gap-2 px-3 py-2 bg-bg-card border border-border rounded-lg hover:bg-bg-hover text-text-primary transition-colors text-sm font-medium"
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
        <span>重新布局</span>
      </button>
    </div>

    <!-- 缩放控制 -->
    <div class="absolute bottom-4 right-4 flex items-center bg-bg-card border border-border rounded-lg shadow-sm overflow-hidden">
      <button @click="zoomOut" class="px-3 py-1.5 hover:bg-bg-hover text-text-primary transition-colors border-r border-border" title="缩小 (20%)">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4" /></svg>
      </button>
      <div class="flex items-center px-2">
        <input 
          type="number" 
          v-model="zoomPercentInput" 
          @change="onZoomInputChange"
          @keyup.enter="onZoomInputChange"
          class="w-12 text-center bg-transparent focus:outline-none text-sm text-text-primary"
          style="-moz-appearance: textfield;"
          min="30" max="150"
        />
        <span class="text-sm text-text-muted select-none">%</span>
      </div>
      <button @click="zoomIn" class="px-3 py-1.5 hover:bg-bg-hover text-text-primary transition-colors border-l border-border" title="放大 (20%)">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
      </button>
    </div>

    <!-- 右键菜单 -->
    <div
      v-if="contextMenu.visible"
      class="fixed z-50 bg-bg-card border border-border rounded-md shadow-lg py-1 min-w-[120px]"
      :style="{ top: `${contextMenu.y}px`, left: `${contextMenu.x}px` }"
    >
      <div
        class="px-4 py-2 text-sm text-text-primary hover:bg-bg-hover cursor-pointer transition-colors flex items-center gap-2"
        @click="setAsRoot"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
        </svg>
        设为根节点
      </div>
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

// 获取当前主题相关的颜色配置
// 直接从 CSS 变量读取，确保与实际主题一致
function getThemeColors() {
  if (typeof window === 'undefined') {
    return {
      nodeLabel: '#e5e7eb',
      edgeLabelBg: '#111827',
      edgeLabel: '#9ca3af',
      nodeBorder: '#374151',
    };
  }
  
  const style = getComputedStyle(document.documentElement);
  
  // 从 CSS 变量读取颜色
  const textPrimary = style.getPropertyValue('--color-text-primary').trim();
  const textMuted = style.getPropertyValue('--color-text-muted').trim();
  const borderDefault = style.getPropertyValue('--color-border-default').trim();
  const bgSecondary = style.getPropertyValue('--color-bg-secondary').trim();
  
  return {
    // 节点标签颜色：使用主文字颜色
    nodeLabel: textPrimary || '#1f2937',
    // 边标签背景色：使用次背景色
    edgeLabelBg: bgSecondary || '#f3f4f6',
    // 边标签文字颜色：使用弱化文字颜色
    edgeLabel: textMuted || '#4b5563',
    // 节点边框颜色
    nodeBorder: borderDefault || '#d1d5db',
  };
}

export interface GraphNode {
  id: string;
  label: string;
  role?: string;
  site?: string;
  ip?: string;
  nodeType?: string;      // managed, unmanaged, inferred, unknown
  macAddress?: string;    // 推断节点的主MAC地址
  macAddresses?: string[];// 推断节点的所有MAC地址
  vendor?: string;
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

const currentLayout = ref("dagre");
const rootNodeId = ref<string | null>(null);
const zoomPercentInput = ref(100);
let isZoomingFromInput = false;
const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  nodeId: "",
});

function hideContextMenu() {
  contextMenu.value.visible = false;
}

// 监听全局点击以关闭菜单
onMounted(() => {
  window.addEventListener("click", hideContextMenu);
});
onUnmounted(() => {
  window.removeEventListener("click", hideContextMenu);
});

// 从 CSS 变量读取状态颜色
function getStatusColors(): Record<string, string> {
  if (typeof window === 'undefined') {
    return {
      confirmed: '#22c55e',
      semi_confirmed: '#f59e0b',
      inferred: '#f59e0b',
      conflict: '#ef4444',
    };
  }
  
  const style = getComputedStyle(document.documentElement);
  return {
    confirmed: style.getPropertyValue('--color-topology-status-confirmed').trim() || '#22c55e',
    semi_confirmed: style.getPropertyValue('--color-topology-status-semi-confirmed').trim() || '#f59e0b',
    inferred: style.getPropertyValue('--color-topology-status-inferred').trim() || '#f59e0b',
    conflict: style.getPropertyValue('--color-topology-status-conflict').trim() || '#ef4444',
  };
}

// 从 CSS 变量读取角色颜色
function getRoleColors(): Record<string, string> {
  if (typeof window === 'undefined') {
    return {
      core: '#3b82f6',
      aggregation: '#8b5cf6',
      access: '#06b6d4',
      firewall: '#ef4444',
      server: '#6b7280',
    };
  }
  
  const style = getComputedStyle(document.documentElement);
  return {
    core: style.getPropertyValue('--color-topology-role-core').trim() || '#3b82f6',
    aggregation: style.getPropertyValue('--color-topology-role-aggregation').trim() || '#8b5cf6',
    access: style.getPropertyValue('--color-topology-role-access').trim() || '#06b6d4',
    firewall: style.getPropertyValue('--color-topology-role-firewall').trim() || '#ef4444',
    server: style.getPropertyValue('--color-topology-role-server').trim() || '#6b7280',
  };
}

// 生成 SVG 节点图标
function getSvgIcon(role: string, color: string) {
  let svg = '';
  // 规范化 role，处理大小写和空值
  const r = (role || '').toLowerCase().trim();
  
  // 使用确切的像素尺寸（26x26）而非百分比，确保在任何复杂动画布局（如 cose 物理引力）下都不会因为画布尺寸推断错误导致 SVG 被挤压
  const svgHeader = `<svg xmlns="http://www.w3.org/2000/svg" width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">`;
  
  switch(r) {
    case 'core': 
    case 'router': // 核心/路由器 - Server Rack / Core
       svg = `${svgHeader}<rect x="2" y="2" width="20" height="20" rx="2" ry="2"></rect><line x1="2" y1="8" x2="22" y2="8"></line><line x1="2" y1="16" x2="22" y2="16"></line><path d="M6 5h.01"/><path d="M6 12h.01"/><path d="M6 19h.01"/></svg>`;
       break;
    case 'aggregation': // 汇聚 - Switch 
       svg = `${svgHeader}<rect x="2" y="6" width="20" height="12" rx="2" ry="2"></rect><path d="M6 10h.01"/><path d="M10 10h.01"/><path d="M14 10h.01"/><path d="M18 10h.01"/><path d="M6 14h.01"/><path d="M10 14h.01"/><path d="M14 14h.01"/><path d="M18 14h.01"/></svg>`;
       break;
    case 'access': // 接入 - Simple Switch
       svg = `${svgHeader}<rect x="2" y="8" width="20" height="8" rx="2" ry="2"></rect><path d="M6 12h.01"/><path d="M10 12h.01"/><path d="M14 12h.01"/><path d="M18 12h.01"/></svg>`;
       break;
    case 'firewall': // 防火墙 - Shield
       svg = `${svgHeader}<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path></svg>`;
       break;
    case 'server': // 服务器
       svg = `${svgHeader}<rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>`;
       break;
    case 'wlc':
    case 'ap': // 无线设备
       svg = `${svgHeader}<path d="M4 10a11 11 0 0 1 16 0"/><path d="M7 14a7 7 0 0 1 10 0"/><path d="M10 18a3 3 0 0 1 4 0"/><circle cx="12" cy="21" r="1"/></svg>`;
       break;
    default: // 未知/其他 - 通用网络设备 (类似交换机)
       svg = `${svgHeader}<rect x="2" y="7" width="20" height="10" rx="2" ry="2"></rect><circle cx="7" cy="12" r="1"></circle><circle cx="12" cy="12" r="1"></circle><circle cx="17" cy="12" r="1"></circle></svg>`;
  }
  return 'data:image/svg+xml;utf8,' + encodeURIComponent(svg);
}

function initGraph() {
  if (!cyContainer.value) return;

  // 获取当前主题颜色
  const themeColors = getThemeColors();
  const statusColors = getStatusColors();
  const roleColors = getRoleColors();

  // 获取额外颜色变量
  const style = getComputedStyle(document.documentElement);
  const nodeSelectedBorder = style.getPropertyValue('--color-topology-node-selected-border').trim() || '#3b82f6';
  const inferredBorder = style.getPropertyValue('--color-topology-inferred-border').trim() || '#f59e0b';
  const inferredBg = style.getPropertyValue('--color-topology-inferred-bg').trim() || '#78716c';
  const edgeDefault = style.getPropertyValue('--color-topology-edge-default').trim() || '#6b7280';

  const elements: cytoscape.ElementDefinition[] = [
    ...props.nodes.map((n) => ({
      data: {
        id: n.id,
        label: n.label || n.id,
        role: n.role,
        nodeType: n.nodeType,
        macAddress: n.macAddress,
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
            roleColors[ele.data("role")] || nodeSelectedBorder,
          "background-opacity": 0.15,
          "background-image": (ele: NodeSingular) => {
            const role = ele.data("role") || 'unknown';
            const color = roleColors[role] || nodeSelectedBorder;
            return getSvgIcon(role, color);
          },
          "background-width": "26px",
          "background-height": "26px",
          "background-position-x": "50%",
          "background-position-y": "50%",
          "background-repeat": "no-repeat",
          "background-fit": "none",
          "background-clip": "node",
          "shape": "ellipse",
          label: "data(label)",
          width: 50,
          height: 50,
          "font-size": "11px",
          "text-valign": "bottom",
          "text-margin-y": 6,
          color: themeColors.nodeLabel,
          "border-width": 2,
          "border-color": (ele: NodeSingular) =>
            roleColors[ele.data("role")] || nodeSelectedBorder,
        },
      },
      {
        selector: "node:selected",
        style: {
          "border-color": nodeSelectedBorder,
          "border-width": 3,
          "background-opacity": 0.25,
        },
      },
      // 推断节点样式：使用虚线边框和不同颜色
      {
        selector: 'node[nodeType="inferred"], node[nodeType="unknown"]',
        style: {
          "border-style": "dashed",
          "border-width": 2,
          "border-color": inferredBorder,
          "background-color": inferredBg,
          "background-opacity": 0.1,
          "background-image": () => getSvgIcon('unknown', inferredBorder),
        },
      },
      {
        selector: "edge",
        style: {
          width: 2,
          "line-color": (ele: EdgeSingular) =>
            statusColors[ele.data("status")] || edgeDefault,
          "target-arrow-color": (ele: EdgeSingular) =>
            statusColors[ele.data("status")] || edgeDefault,
          "target-arrow-shape": "none",
          "curve-style": "straight",
          label: "",
          "source-label": "data(sourceIf)",
          "target-label": "data(targetIf)",
          "font-size": "9px",
          color: themeColors.edgeLabel,
          "text-opacity": 0.95,
          "text-wrap": "wrap",
          "text-max-width": "80px",
          "text-background-color": themeColors.edgeLabelBg,
          "text-background-opacity": 0.88,
          "text-background-padding": "2px",
          "text-background-shape": "roundrectangle",
          "text-rotation": "autorotate",
          "source-text-offset": 28,
          "target-text-offset": 28,
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
          "line-color": nodeSelectedBorder,
          "target-arrow-color": nodeSelectedBorder,
        },
      },
    ],
    layout: getLayoutOpts(true) as cytoscape.LayoutOptions,
    minZoom: 0.3,
    maxZoom: 1.5,
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

  // 节点右键点击事件
  cy.on("cxttap", "node", (evt) => {
    const node = evt.target;
    const originalEvent = evt.originalEvent as MouseEvent;
    contextMenu.value = {
      visible: true,
      x: originalEvent.clientX,
      y: originalEvent.clientY,
      nodeId: node.id(),
    };
  });

  // 交互时隐藏菜单
  cy.on("tap dragstart zoom pan", () => {
    hideContextMenu();
  });

  // 监听图表缩放事件，同步更新输入框
  cy.on("zoom", () => {
    if (!isZoomingFromInput && cy) {
      zoomPercentInput.value = Math.round(cy.zoom() * 100);
    }
  });

  // 确保初始加载完成时也能自适应
  cy.ready(() => {
    safeFit();
  });
  cy.on("layoutstop", safeFit);
}

function safeFit() {
  if (!cy || !cyContainer.value) return;
  if (cyContainer.value.clientHeight > 50) {
    cy.fit(undefined, 50);
    // cy.fit 会忽略 min/maxZoom 限制，因此需要手动钳制
    const zoom = cy.zoom();
    if (zoom > 1.5) {
      cy.zoom(1.5);
      cy.center();
    } else if (zoom < 0.5) {
      cy.zoom(0.5);
      cy.center();
    }
  } else {
    setTimeout(safeFit, 100);
  }
}

function fitToScreen() {
  safeFit();
}

function zoomIn() {
  if (!cy) return;
  const currentZoom = cy.zoom();
  const nextZoom = Math.min(1.5, currentZoom + 0.2);
  cy.zoom({ level: nextZoom, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
}

function zoomOut() {
  if (!cy) return;
  const currentZoom = cy.zoom();
  const nextZoom = Math.max(0.3, currentZoom - 0.2);
  cy.zoom({ level: nextZoom, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
}

function onZoomInputChange() {
  if (!cy) return;
  isZoomingFromInput = true;
  let val = zoomPercentInput.value;
  if (val < 30) val = 30;
  if (val > 150) val = 150;
  zoomPercentInput.value = val;
  cy.zoom({ level: val / 100, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  isZoomingFromInput = false;
}

function onLayoutChange() {
  rootNodeId.value = null; // 切换布局时清除手动指定的根节点
  applyLayout();
}

function getLayoutOpts(isInitial = false) {
  const animate = !isInitial;

  if (rootNodeId.value) {
    return {
      name: "breadthfirst",
      directed: false,
      roots: `[id = "${rootNodeId.value}"]`,
      spacingFactor: 1.5,
      avoidOverlap: true,
      nodeDimensionsIncludeLabels: true,
      animate: animate,
      animationDuration: 300,
      fit: false,
    };
  }

  if (currentLayout.value === "dagre") {
    return {
      name: "dagre",
      rankDir: "TB",
      nodeSep: 100,
      edgeSep: 40,
      rankSep: 120,
      nodeDimensionsIncludeLabels: true,
      animate: animate,
      animationDuration: 300,
      fit: false,
    };
  }
  
  if (currentLayout.value === "cose") {
    return {
      name: "cose",
      animate: animate,
      animationDuration: 300,
      nodeRepulsion: 300000,
      idealEdgeLength: 150,
      edgeElasticity: 100,
      gravity: 150,
      numIter: 1000,
      initialTemp: 200,
      coolingFactor: 0.95,
      minTemp: 1.0,
      nodeDimensionsIncludeLabels: true,
      fit: false,
    };
  }

  if (currentLayout.value === "concentric") {
    return {
      name: "concentric",
      animate: animate,
      animationDuration: 300,
      avoidOverlap: true,
      nodeDimensionsIncludeLabels: true,
      minNodeSpacing: 80,
      spacingFactor: 1.5,
      fit: false,
    };
  }

  return {
    name: currentLayout.value,
    animate: animate,
    animationDuration: 300,
    avoidOverlap: true,
    nodeDimensionsIncludeLabels: true,
    spacingFactor: 1.5,
    fit: false,
  };
}

function applyLayout() {
  if (!cy) return;
  const layout = cy.layout(getLayoutOpts(false) as cytoscape.LayoutOptions);
  layout.on('layoutstop', safeFit);
  layout.run();
}

function setAsRoot() {
  if (contextMenu.value.nodeId) {
    rootNodeId.value = contextMenu.value.nodeId;
    applyLayout();
  }
  hideContextMenu();
}

function clearRoot() {
  rootNodeId.value = null;
  applyLayout();
}

function resetLayout() {
  rootNodeId.value = null;
  applyLayout();
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

// 监听主题变化：通过 MutationObserver 监听 data-theme 属性变化
let themeObserver: MutationObserver | null = null;

onMounted(() => {
  // 初始化 MutationObserver 监听主题变化
  themeObserver = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type === 'attributes' && mutation.attributeName === 'data-theme') {
        cy?.destroy();
        initGraph();
        break;
      }
    }
  });
  
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme', 'class'],
  });
});

onUnmounted(() => {
  themeObserver?.disconnect();
});

defineExpose({
  fitToScreen,
  resetLayout,
});
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

.topology-graph-container {
  @apply relative w-full h-full min-h-[500px] bg-bg-panel rounded-lg;
}

.cy-container {
  @apply w-full h-full min-h-[500px];
}
</style>
