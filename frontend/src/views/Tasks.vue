<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <p class="text-sm text-text-muted">
        选择设备和命令组，创建任务绑定到任务执行页
      </p>
      <div class="flex gap-3">
        <el-button :icon="Position" @click="goToTaskExecution">
          前往任务执行
        </el-button>
        <el-button
          type="primary"
          :icon="Plus"
          @click="openCreateModal"
          :disabled="!canCreate"
        >
          创建任务
        </el-button>
      </div>
    </div>

    <!-- 步骤选择区域 -->
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
      <div class="flex-1 overflow-y-auto scrollbar-custom pr-1">
        <!-- 任务类型 -->
        <el-card shadow="never" class="mb-3" :body-style="{ padding: '16px' }">
          <template #header>
            <div class="flex items-center">
              <span class="text-sm font-medium">任务类型</span>
            </div>
          </template>
          <el-radio-group v-model="selectedTaskType">
            <el-radio-button label="normal" value="normal">普通任务</el-radio-button>
            <el-radio-button label="topology" value="topology">拓扑采集任务</el-radio-button>
            <el-radio-button label="backup" value="backup">配置备份任务</el-radio-button>
          </el-radio-group>
        </el-card>

        <!-- 步骤1: 选择目标设备 -->
        <el-card
          shadow="never"
          class="mb-3"
          :style="{ height: devicePanelHeight + 'px', minHeight: '200px' }"
          :body-style="{ padding: '12px', height: 'calc(100% - 55px)', overflowY: 'auto' }"
        >
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent">1</div>
              <span class="text-sm font-medium">选择目标设备</span>
              <span v-if="selectedDevices.length > 0" class="ml-2 text-xs text-accent font-mono">
                已选 {{ selectedDevices.length }} 台
              </span>
              <el-button link type="primary" class="ml-auto" @click="goToDevices">
                管理设备资产
              </el-button>
            </div>
          </template>
          <div class="flex flex-col h-full">
            <el-button class="w-full mb-3" type="primary" plain @click="showDeviceSelector = true">
              点击选择设备
            </el-button>
            <div v-if="selectedDevices.length > 0" class="text-xs text-text-muted">
              已选设备预览:
              <span class="font-mono text-text-primary">
                {{ selectedDevices.map(d => d.ip).slice(0, 5).join(', ') }}{{ selectedDevices.length > 5 ? '...' : '' }}
              </span>
            </div>
          </div>
        </el-card>

        <!-- 可拖拽分隔条 -->
        <div
          class="h-2 flex items-center justify-center cursor-row-resize group py-1"
          @mousedown="startResize"
        >
          <div class="w-16 h-1.5 rounded-full bg-border group-hover:bg-accent/50 transition-colors"></div>
        </div>

        <!-- 步骤2: 普通任务命令组 -->
        <el-card
          v-if="selectedTaskType === 'normal'"
          shadow="never"
          class="mb-4"
          :style="{ minHeight: commandPanelMinHeight + 'px' }"
          :body-style="{ padding: '12px' }"
        >
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent">2</div>
              <span class="text-sm font-medium">选择命令组</span>
              <span v-if="selectedCommandGroup" class="ml-2 text-xs text-accent font-mono">
                {{ selectedCommandGroup.name }}
              </span>
            </div>
          </template>
          <CommandGroupSelector
            v-model="selectedCommandGroupId"
            @selectionChange="onCommandGroupChange"
          />
        </el-card>

        <!-- 步骤2: 备份采集参数 -->
        <el-card
          v-else-if="selectedTaskType === 'backup'"
          shadow="never"
          class="mb-4"
          :style="{ minHeight: commandPanelMinHeight + 'px' }"
          :body-style="{ padding: '16px' }"
        >
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent">2</div>
              <span class="text-sm font-medium">备份采集说明</span>
            </div>
          </template>
          <div class="space-y-4">
            <el-alert
              type="info"
              :closable="false"
              title="此任务类型将自动连接目标设备下载启动配置（通常为 startup-config 或 saved-configuration）。默认参数将在创建任务时自动生成，您也可以在任务列表中点击编辑进行详情配置。"
            />
            <!-- SFTP超时配置 -->
            <div class="rounded-lg border border-border bg-bg-panel p-3 flex items-center justify-between">
              <div>
                <div class="text-sm font-medium">SFTP下载超时(秒)</div>
                <div class="text-xs text-text-muted mt-1">SFTP下载大文件时的独立超时时间，设置为0时自动使用命令超时的2倍。</div>
              </div>
              <el-input-number v-model="backupSftpTimeoutSec" :min="0" controls-position="right" class="w-32" />
            </div>
          </div>
        </el-card>

        <!-- 步骤2: 拓扑参数 -->
        <el-card
          v-else
          shadow="never"
          class="mb-4"
          :style="{ minHeight: commandPanelMinHeight + 'px' }"
          :body-style="{ padding: '16px' }"
        >
          <template #header>
            <div class="flex items-center gap-3">
              <div class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent">2</div>
              <span class="text-sm font-medium">拓扑采集参数</span>
            </div>
          </template>
          
          <el-form label-position="top">
            <el-form-item label="目标厂商">
              <el-select v-model="topologyVendor" class="w-full">
                <el-option label="自动识别" value="" />
                <el-option v-for="vendor in supportedVendors" :key="vendor" :label="vendor" :value="vendor" />
              </el-select>
            </el-form-item>
            
            <div class="rounded-lg border border-border bg-bg-panel p-3 mb-4 flex items-center justify-between">
              <div>
                <div class="text-sm font-medium">自动构建拓扑</div>
                <div class="text-xs text-text-muted mt-1">采集完成后自动触发拓扑构建。</div>
              </div>
              <el-switch v-model="autoBuildTopology" />
            </div>

            <div class="rounded-lg border border-border bg-bg-panel p-4 space-y-4">
              <div class="flex items-center justify-between gap-2">
                <div>
                  <div class="text-sm font-medium">字段级命令覆盖</div>
                  <div class="text-xs text-text-muted mt-1">在任务维度覆盖默认命令，执行前将按覆盖结果重新生成采集计划。</div>
                </div>
                <div class="flex items-center gap-2">
                  <el-button @click="loadTopologyPreview" :loading="topologyPreviewLoading">
                    刷新预览
                  </el-button>
                  <el-button @click="goToTopologyCommandConfig" plain>
                    配置中心
                  </el-button>
                </div>
              </div>

              <el-alert v-if="topologyPreviewDirty" type="warning" :closable="false" show-icon>
                检测到未刷新的拓扑命令变更，请先刷新预览后再创建任务。
              </el-alert>

              <el-alert v-if="topologyPreviewError" type="error" :closable="false" show-icon>
                {{ topologyPreviewError }}
              </el-alert>

              <div v-if="topologyPreviewLoading" class="text-xs px-2.5 py-2 rounded-lg border border-border bg-bg-card text-text-muted">
                正在加载拓扑命令预览...
              </div>

              <el-empty v-else-if="topologyPreviewCommands.length === 0" description="暂无预览命令。请选择设备后再刷新预览。" :image-size="60" />

              <div v-else class="space-y-3 max-h-[340px] overflow-y-auto scrollbar-custom pr-1">
                <div
                  v-for="cmd in topologyPreviewCommands"
                  :key="cmd.fieldKey"
                  class="rounded-lg border border-border bg-bg-card p-3 space-y-3"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div>
                      <div class="flex items-center gap-2 flex-wrap mb-1">
                        <span class="text-sm font-medium">{{ cmd.displayName }}</span>
                        <el-tag size="small" type="info">{{ cmd.fieldKey }}</el-tag>
                        <el-tag size="small" :type="cmd.required ? 'warning' : 'info'">{{ cmd.required ? "关键字段" : "可选字段" }}</el-tag>
                        <el-tag size="small">{{ cmd.commandSource || "unknown" }}</el-tag>
                      </div>
                      <div class="text-xs text-text-muted">
                        {{ cmd.description || "无描述" }}
                      </div>
                    </div>
                    <el-checkbox
                      :model-value="topologyEnabledValue(cmd.fieldKey, cmd.enabled)"
                      @change="onTopologyEnabledChange(cmd.fieldKey, $event as boolean)"
                    >
                      启用
                    </el-checkbox>
                  </div>

                  <div class="flex gap-2 items-start">
                    <el-input
                      type="textarea"
                      :rows="2"
                      placeholder="命令内容"
                      :model-value="topologyCommandValue(cmd.fieldKey, cmd.command)"
                      @input="onTopologyCommandInput(cmd.fieldKey, $event)"
                      class="flex-1 font-mono"
                    />
                    <el-input-number
                      :model-value="topologyTimeoutValue(cmd.fieldKey, cmd.timeoutSec)"
                      @change="onTopologyTimeoutInput(cmd.fieldKey, $event)"
                      :min="0"
                      controls-position="right"
                      class="w-32"
                    />
                    <el-button @click="resetTopologyOverride(cmd.fieldKey)">
                      恢复继承
                    </el-button>
                  </div>
                </div>
              </div>

              <div class="flex items-center justify-between text-xs">
                <span class="text-text-muted">覆盖项 {{ topologyOverrides.length }} 条</span>
                <span v-if="topologyInvalidCount > 0" class="text-error">
                  存在 {{ topologyInvalidCount }} 条已启用但命令为空的覆盖项
                </span>
                <span v-else class="text-success">覆盖项校验通过</span>
              </div>
            </div>
          </el-form>
        </el-card>
      </div>
    </div>



    <!-- 创建任务弹窗 -->
    <el-dialog
      v-model="createModal.show"
      title="创建任务"
      width="500px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <el-form label-position="top">
        <el-form-item label="任务名称" required>
          <el-input v-model="createModal.name" placeholder="输入任务名称" />
        </el-form-item>
        <el-form-item label="描述（可选）">
          <el-input v-model="createModal.description" type="textarea" :rows="2" placeholder="输入任务描述" />
        </el-form-item>
        <el-form-item label="标签">
          <div class="flex flex-col gap-2 w-full">
            <div class="flex flex-wrap gap-2">
              <el-tag
                v-for="(tag, idx) in createModal.tags"
                :key="idx"
                closable
                @close="createModal.tags.splice(idx, 1)"
              >
                {{ tag }}
              </el-tag>
            </div>
            <div class="flex gap-2">
              <el-input
                v-model="createModal.newTag"
                placeholder="输入标签后点击添加"
                @keyup.enter="addTag"
                class="flex-1"
              />
              <el-button @click="addTag">添加</el-button>
            </div>
          </div>
        </el-form-item>
        <el-form-item>
          <div class="bg-bg-panel border border-border rounded-lg p-3 space-y-2 w-full">
            <h4 class="text-sm font-medium">绑定预览</h4>
            <div class="flex items-center gap-2 text-sm">
              <el-tag>
                {{
                  selectedTaskType === "normal"
                    ? selectedCommandGroup?.name || "未选择命令组"
                    : topologyVendor || "自动识别厂商"
                }}
              </el-tag>
              <el-icon class="text-text-muted"><Right /></el-icon>
              <span class="text-text-secondary">{{ selectedDevices.length }} 台设备</span>
            </div>
            <div class="flex flex-wrap gap-1 mt-1">
              <span
                v-for="dev in selectedDevices.slice(0, 10)"
                :key="dev.ip"
                class="text-xs font-mono px-1.5 py-0.5 rounded bg-bg-card border border-border text-text-muted"
              >{{ dev.ip }}</span>
              <span v-if="selectedDevices.length > 10" class="text-xs text-text-muted">
                +{{ selectedDevices.length - 10 }} 台
              </span>
            </div>
          </div>
        </el-form-item>
        <el-form-item>
          <div class="bg-bg-panel border border-border rounded-lg p-3 flex items-center justify-between w-full">
            <div>
              <div class="text-sm font-medium">原始日志</div>
              <div class="text-xs text-text-muted mt-1">默认关闭。开启后会额外保存完整 SSH 字节流。</div>
            </div>
            <el-switch v-model="createModal.enableRawLog" />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <el-button @click="createModal.show = false">取消</el-button>
          <el-button type="primary" :disabled="!createModal.name.trim()" @click="confirmCreate">
            确认创建
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 设备选择弹窗 -->
    <DeviceSelectorModal
      v-model:visible="showDeviceSelector"
      :devices="deviceList"
      :selected-i-ps="selectedDevices.map(d => d.ip)"
      title="选择目标设备"
      @confirm="onDeviceSelectionConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { Plus, Position, Right } from "@element-plus/icons-vue";
import {
  DeviceAPI,
  TaskGroupAPI,
  TopologyCommandAPI,
  TopologyCommandConfigAPI,
} from "../services/api";
import type {
  DeviceAsset,
  CommandGroup,
  TopologyCommandPreviewView,
  TopologyTaskFieldOverride,
} from "../services/api";
import DeviceSelectorModal from "../components/task/DeviceSelectorModal.vue";
import CommandGroupSelector from "../components/task/CommandGroupSelector.vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const router = useRouter();

// 设备列表和选择状态
const deviceList = ref<DeviceAsset[]>([]);
const selectedDevices = ref<DeviceAsset[]>([]);
const showDeviceSelector = ref(false);
const selectedTaskType = ref<"normal" | "topology" | "backup">("normal");
const selectedCommandGroupId = ref<number>(0);
const selectedCommandGroup = ref<CommandGroup | null>(null);
const supportedVendors = ref<string[]>([]);
const topologyVendor = ref("");
const autoBuildTopology = ref(true);
const backupSftpTimeoutSec = ref(0);

const topologyOverrides = ref<TopologyTaskFieldOverride[]>([]);
const topologyPreview = ref<TopologyCommandPreviewView | null>(null);
const topologyPreviewLoading = ref(false);
const topologyPreviewError = ref("");
const topologyPreviewDirty = ref(false);

// 面板高度控制
const devicePanelHeight = ref(280);
const commandPanelMinHeight = 300;
const minHeight = 150;

// 拖拽调整高度相关
let isResizing = false;
let startY = 0;
let startHeight = 0;

const selectedDeviceIDsSignature = computed(() =>
  [...selectedDevices.value]
    .map((item) => item.id)
    .sort((a, b) => a - b)
    .join(","),
);

const topologyPreviewCommands = computed(
  () => topologyPreview.value?.defaultResolution?.commands || [],
);

const topologyInvalidCount = computed(
  () =>
    topologyOverrides.value.filter(
      (item: TopologyTaskFieldOverride) =>
        item.enabled === true && String(item.command || "").trim() === "",
    ).length,
);

function startResize(e: MouseEvent) {
  isResizing = true;
  startY = e.clientY;
  startHeight = devicePanelHeight.value;
  document.addEventListener("mousemove", onResize);
  document.addEventListener("mouseup", stopResize);
  document.body.style.cursor = "row-resize";
  document.body.style.userSelect = "none";
}

function onResize(e: MouseEvent) {
  if (!isResizing) return;
  const deltaY = e.clientY - startY;
  const newHeight = startHeight + deltaY;

  if (newHeight >= minHeight) {
    devicePanelHeight.value = newHeight;
  }
}

function stopResize() {
  isResizing = false;
  document.removeEventListener("mousemove", onResize);
  document.removeEventListener("mouseup", stopResize);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
}

onUnmounted(() => {
  document.removeEventListener("mousemove", onResize);
  document.removeEventListener("mouseup", stopResize);
});

const canCreate = computed(() => {
  if (selectedTaskType.value === "topology") {
    return selectedDevices.value.length > 0 && topologyInvalidCount.value === 0;
  }
  if (selectedTaskType.value === "backup") {
    return selectedDevices.value.length > 0;
  }
  return selectedDevices.value.length > 0 && selectedCommandGroupId.value > 0;
});

// 创建任务弹窗
const createModal = ref({
  show: false,
  name: "",
  description: "",
  tags: [] as string[],
  newTag: "",
  enableRawLog: false,
});

function generateDefaultName() {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  const h = String(now.getHours()).padStart(2, "0");
  const mi = String(now.getMinutes()).padStart(2, "0");
  const s = String(now.getSeconds()).padStart(2, "0");
  return `Task_${y}${m}${d}_${h}${mi}${s}`;
}

function openCreateModal() {
  if (!canCreate.value) return;
  if (selectedTaskType.value === "topology" && topologyPreviewDirty.value) {
    ElMessage.error("拓扑命令存在未刷新的变更，请先刷新命令预览");
    return;
  }
  createModal.value = {
    show: true,
    name: generateDefaultName(),
    description: "",
    tags: [],
    newTag: "",
    enableRawLog: false,
  };
}

function addTag() {
  const tag = createModal.value.newTag.trim();
  if (tag && !createModal.value.tags.includes(tag)) {
    createModal.value.tags.push(tag);
  }
  createModal.value.newTag = "";
}

async function confirmCreate() {
  if (!createModal.value.name.trim() || !canCreate.value) return;
  if (selectedTaskType.value === "topology") {
    if (topologyInvalidCount.value > 0) {
      ElMessage.error("存在无效拓扑覆盖项，请修正后重试");
      return;
    }
    if (topologyPreviewDirty.value) {
      ElMessage.error("拓扑命令存在未刷新的变更，请先刷新命令预览");
      return;
    }
  }

  try {
    const taskItems =
      selectedTaskType.value === "topology" || selectedTaskType.value === "backup"
        ? [
            {
              commandGroupId: "",
              commands: [] as string[],
              deviceIDs: selectedDevices.value.map((d: DeviceAsset) => d.id),
            },
          ]
        : [
            {
              commandGroupId: String(selectedCommandGroupId.value),
              commands: [] as string[],
              deviceIDs: selectedDevices.value.map((d: DeviceAsset) => d.id),
            },
          ];

    const taskGroup: any = {
      id: 0,
      name: createModal.value.name.trim(),
      description: createModal.value.description.trim(),
      deviceGroup: "",
      commandGroup: selectedCommandGroup.value?.name || "",
      maxWorkers: 10,
      timeout: 60,
      mode: "group" as const,
      taskType: selectedTaskType.value,
      topologyVendor:
        selectedTaskType.value === "topology" ? topologyVendor.value : "",
      topologyFieldOverrides:
        selectedTaskType.value === "topology"
          ? cloneTopologyOverrides(topologyOverrides.value)
          : [],
      autoBuildTopology:
        selectedTaskType.value === "topology" ? autoBuildTopology.value : false,
      items: taskItems,
      tags: createModal.value.tags,
      enableRawLog: createModal.value.enableRawLog,
      backupSaveRootPath: "",
      backupDirNamePattern: selectedTaskType.value === 'backup' ? "%Y-%M-%D" : "",
      backupFileNamePattern: selectedTaskType.value === 'backup' ? "%H_startup_%h%m%s.cfg" : "",
      backupStartupCommand: selectedTaskType.value === 'backup' ? "display startup" : "",
      backupSftpTimeoutSec: selectedTaskType.value === 'backup' ? backupSftpTimeoutSec.value : 0,
      status: "",
      createdAt: new Date(),
      updatedAt: new Date(),
    };

    const createdTask = await TaskGroupAPI.createTaskGroup(taskGroup);
    logger.debug(`任务创建成功，id=${createdTask?.id}`, 'Tasks');
    createModal.value.show = false;
    ElMessage.success("任务创建成功，正在跳转任务执行页");
    router.push({
      path: "/task-execution",
      query: { refresh: String(Date.now()) },
    });
  } catch (err: any) {
    logger.error('创建任务失败', 'Tasks', err);
    ElMessage.error(`创建失败: ${err?.message || err}`);
  }
}

async function loadTopologyVendors() {
  try {
    supportedVendors.value =
      (await TopologyCommandAPI.getTaskTopologyVendors()) || [];
  } catch (err) {
    logger.error('加载拓扑厂商列表失败', 'Tasks', err);
    supportedVendors.value =
      (await TopologyCommandConfigAPI.getSupportedTopologyVendors()) || [];
  }
}

function cloneTopologyOverrides(
  overrides?: TopologyTaskFieldOverride[],
): TopologyTaskFieldOverride[] {
  return (overrides || []).map((item) => ({
    fieldKey: String(item.fieldKey || "").trim(),
    command: String(item.command || ""),
    timeoutSec: Number(item.timeoutSec || 0),
    enabled: typeof item.enabled === "boolean" ? item.enabled : undefined,
  }));
}

function findTopologyOverride(fieldKey: string) {
  return topologyOverrides.value.find(
    (item: TopologyTaskFieldOverride) => item.fieldKey === fieldKey,
  );
}

function findTopologyOverrideIndex(fieldKey: string) {
  return topologyOverrides.value.findIndex(
    (item: TopologyTaskFieldOverride) => item.fieldKey === fieldKey,
  );
}

function ensureTopologyOverride(fieldKey: string) {
  const normalizedFieldKey = fieldKey.trim();
  let item = findTopologyOverride(normalizedFieldKey);
  if (item) {
    return item;
  }
  item = {
    fieldKey: normalizedFieldKey,
    command: "",
    timeoutSec: 0,
  };
  topologyOverrides.value = [...topologyOverrides.value, item];
  return item;
}

function compactTopologyOverride(fieldKey: string) {
  const index = findTopologyOverrideIndex(fieldKey);
  if (index < 0) {
    return;
  }
  const current = topologyOverrides.value[index];
  if (!current) {
    return;
  }
  const hasCommand = String(current.command || "") !== "";
  const hasTimeout = Number(current.timeoutSec || 0) > 0;
  const hasEnabled = typeof current.enabled === "boolean";
  if (hasCommand || hasTimeout || hasEnabled) {
    return;
  }
  topologyOverrides.value = topologyOverrides.value.filter(
    (item: TopologyTaskFieldOverride) => item.fieldKey !== fieldKey,
  );
}

function markTopologyPreviewDirty() {
  topologyPreviewDirty.value = true;
}

function topologyCommandValue(fieldKey: string, fallback: string) {
  const override = findTopologyOverride(fieldKey);
  if (override && override.command !== "") {
    return override.command;
  }
  return fallback || "";
}

function topologyTimeoutValue(fieldKey: string, fallback: number) {
  const override = findTopologyOverride(fieldKey);
  if (override && Number(override.timeoutSec || 0) > 0) {
    return Number(override.timeoutSec);
  }
  return Number(fallback || 0);
}

function topologyEnabledValue(fieldKey: string, fallback: boolean) {
  const override = findTopologyOverride(fieldKey);
  if (override && typeof override.enabled === "boolean") {
    return override.enabled;
  }
  return Boolean(fallback);
}

function onTopologyCommandInput(fieldKey: string, value: string) {
  const override = ensureTopologyOverride(fieldKey);
  override.command = value || "";
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

function onTopologyTimeoutInput(fieldKey: string, value: number | undefined) {
  const override = ensureTopologyOverride(fieldKey);
  const v = Number(value || 0);
  override.timeoutSec = Number.isFinite(v) && v > 0 ? v : 0;
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

function onTopologyEnabledChange(fieldKey: string, value: boolean) {
  const override = ensureTopologyOverride(fieldKey);
  override.enabled = value;
  compactTopologyOverride(fieldKey);
  markTopologyPreviewDirty();
}

async function resetTopologyOverride(fieldKey: string) {
  topologyOverrides.value = topologyOverrides.value.filter(
    (item: TopologyTaskFieldOverride) => item.fieldKey !== fieldKey,
  );
  markTopologyPreviewDirty();
  await loadTopologyPreview();
}

async function loadTopologyPreview() {
  if (selectedTaskType.value !== "topology") {
    return;
  }
  topologyPreviewLoading.value = true;
  topologyPreviewError.value = "";
  try {
    const nextPreview = await TopologyCommandAPI.previewTopologyCommands(
      topologyVendor.value,
      selectedDevices.value.map((item) => item.id),
      cloneTopologyOverrides(topologyOverrides.value),
    );
    topologyPreview.value = nextPreview;
    topologyOverrides.value = cloneTopologyOverrides(
      nextPreview?.taskOverrides || [],
    );
    topologyPreviewDirty.value = false;
  } catch (err: any) {
    logger.error('加载拓扑命令预览失败', 'Tasks', err);
    topologyPreviewError.value = `命令预览加载失败: ${err?.message || err}`;
  } finally {
    topologyPreviewLoading.value = false;
  }
}

function onDeviceSelectionConfirm(devs: DeviceAsset[]) {
  selectedDevices.value = devs;
}

function onCommandGroupChange(group: CommandGroup | null) {
  selectedCommandGroup.value = group;
}

function goToDevices() {
  router.push("/devices");
}

function goToTaskExecution() {
  router.push("/task-execution");
}

function goToTopologyCommandConfig() {
  router.push("/topology-command-config");
}

async function loadDevices() {
  try {
    const result = await DeviceAPI.listDevices();
    deviceList.value = result || [];
  } catch (err) {
    logger.error('加载设备列表失败', 'Tasks', err);
    deviceList.value = [];
  }
}

watch(
  () => [
    selectedTaskType.value,
    topologyVendor.value,
    selectedDeviceIDsSignature.value,
  ],
  async ([taskType]) => {
    if (taskType !== "topology") {
      return;
    }
    if (selectedDevices.value.length === 0) {
      topologyPreview.value = null;
      topologyPreviewDirty.value = false;
      topologyPreviewError.value = "";
      return;
    }
    await loadTopologyPreview();
  },
);

watch(selectedTaskType, async (value) => {
  if (value !== "topology") {
    topologyPreview.value = null;
    topologyPreviewError.value = "";
    topologyPreviewDirty.value = false;
    topologyOverrides.value = [];
    return;
  }
  if (selectedDevices.value.length === 0) {
    topologyPreview.value = null;
    topologyPreviewError.value = "";
    topologyPreviewDirty.value = false;
    return;
  }
  await loadTopologyPreview();
});

onMounted(() => {
  loadDevices();
  loadTopologyVendors();
});
</script>

<style scoped>
</style>
