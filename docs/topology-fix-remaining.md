# 拓扑还原功能剩余问题修复方案

> 基于 `docs/fix-plan-topology.md` 实施情况的分析，本文档针对尚未完成的问题提供详细修复方案。

---

## 1. 问题清单与优先级

| 优先级 | 问题                 | 影响范围                          | 修复目标日期 |
| ------ | -------------------- | --------------------------------- | ------------ |
| P2     | 设备凭据明文存储     | `internal/config/config.go`       | T+7          |
| P3     | 拓扑页图形化展示缺失 | `frontend/src/views/Topology.vue` | T+14         |
| P3     | 任务分阶段状态机缺失 | `internal/models/discovery.go`    | T+14         |
| P3     | 前端解析错误展示不足 | `frontend/src/views/Topology.vue` | T+5          |

---

#### 2.2.2 修改设备资产模型

**文件**：`internal/models/models.go`

```go
// DeviceAsset 设备资产
type DeviceAsset struct {
	ID          uint     `json:"id" gorm:"primaryKey;autoIncrement"`
	IP          string   `json:"ip" gorm:"uniqueIndex;not null"`
	Port        int      `json:"port"`
	Protocol    string   `json:"protocol"`
	Username    string   `json:"username"`
	Password    string   `json:"-" gorm:"column:password"` // JSON 不输出，数据库存储加密值
	Group       string   `json:"group" gorm:"column:group_name"`
	Tags        string   `json:"tags"` // JSON 序列化存储
	Vendor      string   `json:"vendor"`
	Role        string   `json:"role"`
	Site        string   `json:"site"`
	DisplayName string   `json:"displayName" gorm:"column:display_name"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// BeforeSave GORM 钩子：保存前加密密码
func (d *DeviceAsset) BeforeSave(tx *gorm.DB) error {
	if d.Password != "" && !config.IsEncrypted(d.Password) {
		cipher := config.GetCredentialCipher()
		encrypted, err := cipher.Encrypt(d.Password)
		if err != nil {
			return fmt.Errorf("加密密码失败: %v", err)
		}
		d.Password = encrypted
	}
	return nil
}

// AfterFind GORM 钩子：查询后解密密码
func (d *DeviceAsset) AfterFind(tx *gorm.DB) error {
	if d.Password != "" {
		cipher := config.GetCredentialCipher()
		decrypted, err := cipher.Decrypt(d.Password)
		if err != nil {
			// 解密失败时记录日志，但不阻断查询
			logger.Warn("Config", "-", "解密设备 %s 密码失败: %v", d.IP, err)
			return nil
		}
		d.Password = decrypted
	}
	return nil
}
```

#### 2.2.3 迁移脚本

**文件**：`scripts/migrate_credentials.go`

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
)

func main() {
	// 初始化配置
	config.InitConfig()

	cipher := config.GetCredentialCipher()

	var devices []models.DeviceAsset
	if err := config.DB.Find(&devices).Error; err != nil {
		log.Fatalf("查询设备失败: %v", err)
	}

	migrated := 0
	for _, d := range devices {
		if d.Password == "" {
			continue
		}

		// 检查是否已是密文
		if strings.HasPrefix(d.Password, "enc:") {
			continue
		}

		// 加密密码
		encrypted, err := cipher.Encrypt(d.Password)
		if err != nil {
			log.Printf("加密设备 %s 失败: %v", d.IP, err)
			continue
		}

		// 直接更新数据库（跳过 BeforeSave 钩子）
		if err := config.DB.Model(&models.DeviceAsset{}).
			Where("id = ?", d.ID).
			Update("password", encrypted).Error; err != nil {
			log.Printf("更新设备 %s 失败: %v", d.IP, err)
			continue
		}

		migrated++
		log.Printf("设备 %s 密码已加密", d.IP)
	}

	log.Printf("迁移完成，共处理 %d 台设备", migrated)
}
```

### 2.3 验收标准

- [ ] 数据库中 `device_assets.password` 字段存储格式为 `enc:xxxxx`
- [ ] 前端设备列表接口不返回密码字段
- [ ] 设备连接时能正确解密密码并登录
- [ ] 迁移脚本成功执行，无明文密码残留

---

## 3. P3 体验优化：拓扑页图形化展示

### 3.1 问题分析

- **现状**：[`Topology.vue`](frontend/src/views/Topology.vue) 仅为表格视图，缺少图形化拓扑展示
- **影响**：用户难以直观理解网络拓扑结构
- **证据**：`frontend/src/views/Topology.vue:107` 使用 `<table>` 展示链路

### 3.2 修复方案

#### 3.2.1 安装依赖

```bash
cd frontend
npm install cytoscape cytoscape-dagre @types/cytoscape
```

#### 3.2.2 创建图形组件

**文件**：`frontend/src/components/topology/TopologyGraph.vue`

```vue
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
import { ref, onMounted, onUnmounted, watch, computed } from "vue";
import cytoscape, { Core, NodeSingular, EdgeSingular } from "cytoscape";
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
    },
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
  }).run();
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
```

#### 3.2.3 修改 Topology.vue 集成图形组件

**文件**：`frontend/src/views/Topology.vue`

在 `<template>` 中添加视图切换和图形组件：

```vue
<template>
  <div class="space-y-5 animate-slide-in">
    <!-- ... 现有头部和过滤器 ... -->

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

    <!-- 表格视图（现有代码） -->
    <div v-else class="grid grid-cols-3 gap-4 items-start">
      <!-- ... 现有表格代码 ... -->
    </div>
  </div>
</template>

<script setup lang="ts">
import TopologyGraph from "../components/topology/TopologyGraph.vue";

const viewType = ref<"graph" | "table">("graph");

// 转换数据格式供图形组件使用
const graphNodes = computed(() =>
  (graph.value.nodes || []).map((n) => ({
    id: n.id,
    label: n.label || n.id,
    role: n.role,
    site: n.site,
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
</script>
```

### 3.3 验收标准

- [ ] 拓扑页面默认显示图形视图
- [ ] 支持图形/表格视图切换
- [ ] 节点按角色着色，边按状态着色
- [ ] 支持缩放、拖拽、点击查看详情
- [ ] inferred 状态边显示为虚线

---

## 4. P3 体验优化：任务分阶段状态机

### 4.1 问题分析

- **现状**：[`DiscoveryTask`](internal/models/discovery.go:11) 仅有 `status` 字段，无阶段信息
- **影响**：前端无法精确展示当前执行阶段
- **证据**：`internal/models/discovery.go:14` status 注释为 `pending / running / partial / completed / failed / cancelled`

### 4.2 修复方案

#### 4.2.1 扩展数据模型

**文件**：`internal/models/discovery.go`

```go
// DiscoveryTaskPhase 发现任务阶段
type DiscoveryTaskPhase string

const (
	PhasePending    DiscoveryTaskPhase = "pending"     // 等待启动
	PhaseCollecting DiscoveryTaskPhase = "collecting"  // SSH 采集中
	PhaseParsing    DiscoveryTaskPhase = "parsing"     // 结构化解析中
	PhaseBuilding   DiscoveryTaskPhase = "building"    // 拓扑构建中
	PhaseCompleted  DiscoveryTaskPhase = "completed"   // 完成
	PhaseFailed     DiscoveryTaskPhase = "failed"      // 失败
	PhaseCancelled  DiscoveryTaskPhase = "cancelled"   // 已取消
)

// DiscoveryTask 发现任务主表
type DiscoveryTask struct {
	ID             string             `json:"id" gorm:"primaryKey"`
	Name           string             `json:"name"`
	Status         string             `json:"status"`         // 终态：completed / failed / cancelled
	Phase          DiscoveryTaskPhase `json:"phase"`          // 当前阶段
	PhaseStartedAt *time.Time         `json:"phaseStartedAt"` // 当前阶段开始时间
	PhaseProgress  int                `json:"phaseProgress"`  // 当前阶段进度 (0-100)
	TotalCount     int                `json:"totalCount"`
	SuccessCount   int                `json:"successCount"`
	FailedCount    int                `json:"failedCount"`
	StartedAt      *time.Time         `json:"startedAt"`
	FinishedAt     *time.Time         `json:"finishedAt"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	MaxWorkers     int                `json:"maxWorkers"`
	TimeoutSec     int                `json:"timeoutSec"`
	Vendor         string             `json:"vendor"`
	ParseErrors    string             `json:"parseErrors" gorm:"type:text"` // JSON 序列化的解析错误
}
```

#### 4.2.2 修改 Runner 更新阶段

**文件**：`internal/discovery/runner.go`

```go
// setPhase 更新任务阶段
func (r *Runner) setPhase(taskID string, phase models.DiscoveryTaskPhase) {
	now := time.Now()
	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"phase":           phase,
		"phase_started_at": now,
		"phase_progress":  0,
	})
}

// setPhaseProgress 更新阶段进度
func (r *Runner) setPhaseProgress(taskID string, progress int) {
	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Update("phase_progress", progress)
}

// Start 启动发现任务（修改部分）
func (r *Runner) Start(ctx context.Context, req models.StartDiscoveryRequest) (string, error) {
	// ... 创建任务 ...

	// 设置初始阶段
	r.setPhase(taskID, models.PhaseCollecting)

	// ... 启动采集 ...
}

// collectDevice 设备采集完成后更新进度
func (r *Runner) onDeviceCollected(taskID string, completed int, total int) {
	progress := int(float64(completed) / float64(total) * 100)
	r.setPhaseProgress(taskID, progress)
}

// onCollectionComplete 采集完成后切换到解析阶段
func (r *Runner) onCollectionComplete(taskID string) {
	r.setPhase(taskID, models.PhaseParsing)
}

// onParseComplete 解析完成后切换到构建阶段
func (r *Runner) onParseComplete(taskID string) {
	r.setPhase(taskID, models.PhaseBuilding)
}

// onBuildComplete 构建完成
func (r *Runner) onBuildComplete(taskID string) {
	r.setPhase(taskID, models.PhaseCompleted)
	r.db.Model(&models.DiscoveryTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":      "completed",
		"finished_at": time.Now(),
	})
}
```

#### 4.2.3 前端阶段展示

**文件**：`frontend/src/views/Discovery.vue`

```vue
<template>
  <!-- 阶段进度条 -->
  <div class="bg-bg-card border border-border rounded-lg p-4">
    <div class="flex items-center justify-between">
      <div
        v-for="(phase, idx) in phases"
        :key="phase.key"
        class="flex items-center"
      >
        <div class="flex flex-col items-center">
          <div
            class="w-8 h-8 rounded-full flex items-center justify-center text-xs font-medium"
            :class="{
              'bg-accent text-white': currentPhase === phase.key,
              'bg-success text-white': isPhaseCompleted(phase.key),
              'bg-bg-panel text-text-muted': isPhasePending(phase.key),
            }"
          >
            <svg
              v-if="isPhaseCompleted(phase.key)"
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
            >
              <path
                d="M20 6L9 17l-5-5"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
            <span v-else>{{ idx + 1 }}</span>
          </div>
          <span
            class="text-xs mt-1"
            :class="{
              'text-accent': currentPhase === phase.key,
              'text-success': isPhaseCompleted(phase.key),
              'text-text-muted': isPhasePending(phase.key),
            }"
            >{{ phase.label }}</span
          >
        </div>
        <div
          v-if="idx < phases.length - 1"
          class="w-12 h-0.5 mx-2"
          :class="{
            'bg-success': isPhaseCompleted(phases[idx + 1].key),
            'bg-border': !isPhaseCompleted(phases[idx + 1].key),
          }"
        ></div>
      </div>
    </div>

    <!-- 当前阶段进度 -->
    <div v-if="currentTask?.phaseProgress" class="mt-3">
      <div class="flex justify-between text-xs text-text-muted mb-1">
        <span>{{ currentPhaseLabel }}</span>
        <span>{{ currentTask.phaseProgress }}%</span>
      </div>
      <div class="h-1.5 bg-bg-panel rounded-full overflow-hidden">
        <div
          class="h-full bg-accent transition-all duration-300"
          :style="{ width: `${currentTask.phaseProgress}%` }"
        ></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const phases = [
  { key: "collecting", label: "采集" },
  { key: "parsing", label: "解析" },
  { key: "building", label: "构建" },
  { key: "completed", label: "完成" },
];

const phaseOrder = [
  "pending",
  "collecting",
  "parsing",
  "building",
  "completed",
];

const currentPhase = computed(() => currentTask.value?.phase || "pending");

const currentPhaseLabel = computed(() => {
  const phase = phases.find((p) => p.key === currentPhase.value);
  return phase?.label || "";
});

function isPhaseCompleted(phaseKey: string): boolean {
  const currentIndex = phaseOrder.indexOf(currentPhase.value);
  const targetIndex = phaseOrder.indexOf(phaseKey);
  return targetIndex < currentIndex;
}

function isPhasePending(phaseKey: string): boolean {
  const currentIndex = phaseOrder.indexOf(currentPhase.value);
  const targetIndex = phaseOrder.indexOf(phaseKey);
  return targetIndex > currentIndex;
}
</script>
```

### 4.3 验收标准

- [ ] 任务详情显示当前阶段和进度
- [ ] 阶段切换时前端实时更新
- [ ] 完成阶段显示绿色勾选
- [ ] 当前阶段高亮显示

---

## 5. P3 体验优化：前端解析错误展示

### 5.1 问题分析

- **现状**：[`Topology.vue`](frontend/src/views/Topology.vue) 已有 `summary.errors` 但未在 UI 中展示
- **影响**：用户无法感知解析失败导致的拓扑数据不完整

### 5.2 修复方案

**文件**：`frontend/src/views/Topology.vue`

在拓扑构建按钮下方添加错误提示：

```vue
<template>
  <!-- ... 现有代码 ... -->

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

  <!-- ... 后续代码 ... -->
</template>
```

### 5.3 验收标准

- [ ] 存在解析错误时显示警告框
- [ ] 错误列表可滚动查看
- [ ] 错误信息包含设备 IP 和命令类型

---

## 6. 实施计划

### 6.1 第一阶段：安全加固（T+1 ~ T+7）

| 日期    | 任务                      | 产出     |
| ------- | ------------------------- | -------- |
| T+1     | 实现 crypto.go 加密模块   | PR #1    |
| T+2     | 修改 DeviceAsset 模型钩子 | PR #1    |
| T+3     | 编写迁移脚本              | PR #1    |
| T+4     | 测试加密/解密流程         | 测试报告 |
| T+5     | 前端错误展示优化          | PR #2    |
| T+6~T+7 | 集成测试与验收            | 验收报告 |

### 6.2 第二阶段：体验优化（T+8 ~ T+14）

| 日期      | 任务                      | 产出     |
| --------- | ------------------------- | -------- |
| T+8~T+10  | 拓扑图形化组件            | PR #3    |
| T+11      | Topology.vue 集成图形视图 | PR #3    |
| T+12~T+13 | 任务分阶段状态机          | PR #4    |
| T+14      | 整体验收                  | 发布说明 |

---

## 7. 风险评估

| 风险                     | 影响 | 缓解措施                                 |
| ------------------------ | ---- | ---------------------------------------- |
| 加密密钥丢失             | 高   | 密钥文件备份到安全位置，提供恢复流程文档 |
| Cytoscape 大规模节点性能 | 中   | 节点超过 100 时提示切换表格视图          |
| 阶段状态迁移兼容性       | 低   | 旧任务默认 phase 为 pending 或 completed |

---

## 8. 验收检查清单

### P2 安全加固

- [ ] 数据库中无明文密码
- [ ] 迁移脚本成功执行
- [ ] 设备连接功能正常

### P3 体验优化

- [ ] 拓扑图形化展示正常
- [ ] 阶段状态可见
- [ ] 解析错误明确展示

---

**文档版本**: v1.0  
**创建日期**: 2026-03-19  
**作者**: NetWeaverGo Team
