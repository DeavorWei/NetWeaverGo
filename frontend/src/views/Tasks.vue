<template>
  <div class="animate-slide-in gap-5 h-full flex flex-col p-1">
    <!-- 标题栏 -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 rounded-xl border border-border bg-bg-panel/40 backdrop-blur-md shadow-sm flex-shrink-0">
      <div>
        <h1 class="text-lg font-semibold text-text-primary">新建任务</h1>
        <p class="text-xs text-text-muted mt-1">配置执行目标设备与操作指令，发布并执行采集/备份/交互命令任务</p>
      </div>
      <div class="flex items-center gap-3">
        <el-button :icon="Position" @click="goToTaskExecution">
          前往任务执行
        </el-button>
        <el-button
          type="primary"
          :icon="Plus"
          @click="openCreateModal"
          :disabled="!canCreate"
          class="shadow-md shadow-primary/10"
        >
          创建任务
        </el-button>
      </div>
    </div>

    <!-- 主体区域：左右双栏布局 -->
    <div class="flex-1 grid grid-cols-1 lg:grid-cols-12 gap-5 min-h-0 overflow-hidden">
      <!-- 左侧面板: 基础配置与设备选择 -->
      <div class="lg:col-span-4 flex flex-col gap-4 min-h-0">
        
        <!-- 任务类型 -->
        <el-card shadow="never" class="border-border rounded-xl flex-shrink-0" :body-style="{ padding: '16px' }">
          <template #header>
            <div class="flex items-center gap-2">
              <el-icon class="text-primary"><Tickets /></el-icon>
              <span class="text-sm font-semibold">1. 选择任务类型</span>
            </div>
          </template>
          <el-radio-group v-model="selectedTaskType" class="w-full flex">
            <el-radio-button label="normal" value="normal" class="flex-1 text-center">普通任务</el-radio-button>
            <el-radio-button label="topology" value="topology" class="flex-1 text-center">拓扑采集</el-radio-button>
            <el-radio-button label="backup" value="backup" class="flex-1 text-center">配置备份</el-radio-button>
          </el-radio-group>
        </el-card>

        <!-- 目标设备 -->
        <el-card
          shadow="never"
          class="border-border rounded-xl flex-1 flex flex-col min-h-0"
          :body-style="{ padding: '16px', display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }"
        >
          <template #header>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <el-icon class="text-primary"><Cpu /></el-icon>
                <span class="text-sm font-semibold">2. 目标设备</span>
                <el-tag v-if="selectedDevices.length > 0" size="small" type="primary" effect="light" class="font-mono">
                  已选 {{ selectedDevices.length }}
                </el-tag>
              </div>
              <el-button link type="primary" size="small" @click="goToDevices">
                管理资产
              </el-button>
            </div>
          </template>
          
          <div class="flex flex-col h-full flex-1 min-h-0">
            <el-button class="w-full mb-3 shadow-sm" type="primary" plain :icon="Plus" @click="showDeviceSelector = true">
              选择目标设备
            </el-button>
            
            <!-- 已选设备列表 -->
            <div v-if="selectedDevices.length > 0" class="flex-1 overflow-y-auto scrollbar-custom border border-border rounded-lg bg-bg-panel/20 p-2 min-h-0">
              <div class="space-y-1.5">
                <div
                  v-for="(dev, idx) in selectedDevices"
                  :key="dev.ip"
                  class="group flex items-center justify-between p-2 rounded bg-bg-card hover:bg-bg-panel border border-border/40 transition-colors text-xs"
                >
                  <div class="flex items-center gap-2 min-w-0">
                    <span class="font-mono font-medium truncate text-text-primary">{{ dev.ip }}</span>
                    <el-tag size="small" type="info" class="flex-shrink-0">{{ dev.protocol }}</el-tag>
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="text-[10px] text-text-muted truncate max-w-[80px]" :title="dev.group">{{ dev.group || '默认分组' }}</span>
                    <el-button
                      link
                      type="danger"
                      size="small"
                      :icon="Delete"
                      class="hover:text-red-500 opacity-60 group-hover:opacity-100 transition-opacity"
                      @click="selectedDevices.splice(idx, 1)"
                    />
                  </div>
                </div>
              </div>
            </div>
            <el-empty
              v-else
              description="暂未选择任何执行目标设备"
              :image-size="60"
              class="flex-1 flex flex-col justify-center items-center py-6"
            />
          </div>
        </el-card>

      </div>

      <!-- 右侧面板: 核心业务参数配置 -->
      <div class="lg:col-span-8 flex flex-col min-h-0">
        
        <!-- 步骤2: 普通任务命令组 -->
        <el-card
          v-if="selectedTaskType === 'normal'"
          shadow="never"
          class="border-border rounded-xl h-full flex flex-col"
          :body-style="{ padding: '16px', display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }"
        >
          <template #header>
            <div class="flex items-center gap-2">
              <el-icon class="text-primary"><Tickets /></el-icon>
              <span class="text-sm font-semibold">3. 选择命令组</span>
            </div>
          </template>
          <div class="flex-1 min-h-0">
            <CommandGroupSelector
              v-model="selectedCommandGroupId"
              @selectionChange="onCommandGroupChange"
            />
          </div>
        </el-card>

        <!-- 步骤2: 备份采集参数 -->
        <el-card
          v-else-if="selectedTaskType === 'backup'"
          shadow="never"
          class="border-border rounded-xl h-full flex flex-col"
          :body-style="{ padding: '16px', display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }"
        >
          <template #header>
            <div class="flex items-center gap-2">
              <el-icon class="text-primary"><Folder /></el-icon>
              <span class="text-sm font-semibold">3. 配置备份参数</span>
            </div>
          </template>
          <div class="flex-1 space-y-4 min-h-0 overflow-y-auto scrollbar-custom pr-1">
            <el-alert
              type="info"
              :closable="false"
              class="border border-info/20"
            >
              <template #title>
                <span class="font-medium text-xs">配置备份说明</span>
              </template>
              <span class="text-xs">
                此任务类型将自动连接目标设备下载启动配置（通常为 startup-config 或 saved-configuration）。默认参数将在创建任务时自动生成，您也可以在任务列表中点击编辑进行详情配置。
              </span>
            </el-alert>
            
            <div class="rounded-xl border border-border bg-bg-panel/40 p-4 flex items-center justify-between">
              <div>
                <div class="text-sm font-semibold text-text-primary">SFTP 下载超时 (秒)</div>
                <div class="text-xs text-text-muted mt-1">SFTP 下载大配置文件时的独立超时时间。设置为 0 时，自动使用普通命令超时的 2 倍。</div>
              </div>
              <el-input-number v-model="backupSftpTimeoutSec" :min="0" controls-position="right" class="w-32" />
            </div>
          </div>
        </el-card>

        <!-- 步骤2: 拓扑参数 -->
        <el-card
          v-else
          shadow="never"
          class="border-border rounded-xl h-full flex flex-col"
          :body-style="{ padding: '16px', display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }"
        >
          <template #header>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <el-icon class="text-primary"><Connection /></el-icon>
                <span class="text-sm font-semibold">3. 拓扑采集参数</span>
              </div>
              <div class="flex items-center gap-2">
                <el-button :loading="topologyPreviewLoading" :icon="Refresh" @click="loadTopologyPreview" size="small" type="primary" plain>
                  刷新预览
                </el-button>
                <el-button @click="goToTopologyCommandConfig" size="small" :icon="Setting" plain>
                  配置中心
                </el-button>
              </div>
            </div>
          </template>
          
          <div class="flex-1 flex flex-col gap-4 min-h-0">
            <el-form label-position="top" class="grid grid-cols-1 md:grid-cols-2 gap-4 flex-shrink-0">
              <el-form-item label="目标厂商" class="mb-0">
                <el-select v-model="topologyVendor" class="w-full">
                  <el-option label="自动识别" value="" />
                  <el-option v-for="vendor in supportedVendors" :key="vendor" :label="vendor" :value="vendor" />
                </el-select>
              </el-form-item>
              
              <el-form-item label="自动构建拓扑" class="mb-0">
                <div class="w-full flex items-center justify-between h-[32px] px-3 rounded-lg border border-border bg-bg-panel/20">
                  <span class="text-xs text-text-muted">采集完成后自动触发拓扑构建</span>
                  <el-switch v-model="autoBuildTopology" size="small" />
                </div>
              </el-form-item>
            </el-form>
            
            <div class="flex-1 flex flex-col min-h-0 rounded-xl border border-border bg-bg-panel/20 p-4 gap-3">
              <div class="flex items-center justify-between flex-shrink-0">
                <h4 class="text-xs font-semibold text-text-primary">字段级命令覆盖</h4>
                <div class="text-[10px] text-text-muted">
                  在任务维度覆盖默认命令，执行前将按覆盖结果重新生成采集计划
                </div>
              </div>

              <el-alert v-if="topologyPreviewDirty" type="warning" :closable="false" show-icon class="py-1 flex-shrink-0">
                检测到未刷新的拓扑命令变更，请先刷新预览后再创建任务。
              </el-alert>

              <el-alert v-if="topologyPreviewError" type="error" :closable="false" show-icon class="py-1 flex-shrink-0">
                {{ topologyPreviewError }}
              </el-alert>

              <!-- 预览加载/空状态 -->
              <div v-if="topologyPreviewLoading" class="flex-1 flex flex-col justify-center items-center text-xs text-text-muted py-8">
                <el-icon class="animate-spin text-lg mb-2"><Refresh /></el-icon>
                正在加载拓扑命令预览...
              </div>

              <el-empty
                v-else-if="topologyPreviewCommands.length === 0"
                description="暂无预览命令。请先选择设备并刷新预览。"
                :image-size="60"
                class="flex-1 flex flex-col justify-center items-center py-6 bg-bg-card/40 rounded-lg border border-dashed border-border"
              />

              <!-- 命令列表 -->
              <div v-else class="flex-1 overflow-y-auto scrollbar-custom pr-1 space-y-3 min-h-0">
                <div
                  v-for="cmd in topologyPreviewCommands"
                  :key="cmd.fieldKey"
                  class="rounded-xl border border-border bg-bg-card p-3 space-y-3 shadow-sm hover:shadow-md transition-shadow"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <div class="flex items-center gap-1.5 flex-wrap mb-1">
                        <span class="text-xs font-semibold text-text-primary">{{ cmd.displayName }}</span>
                        <el-tag size="small" type="info" effect="plain" class="font-mono text-[10px]">{{ cmd.fieldKey }}</el-tag>
                        <el-tag size="small" :type="cmd.required ? 'warning' : 'info'" effect="light" class="text-[10px]">
                          {{ cmd.required ? "关键" : "可选" }}
                        </el-tag>
                        <el-tag size="small" type="success" effect="plain" class="text-[10px]">{{ cmd.commandSource || "unknown" }}</el-tag>
                      </div>
                      <div class="text-[11px] text-text-muted leading-relaxed">
                        {{ cmd.description || "无描述" }}
                      </div>
                    </div>
                    <el-checkbox
                      :model-value="topologyEnabledValue(cmd.fieldKey, cmd.enabled)"
                      @change="onTopologyEnabledChange(cmd.fieldKey, $event as boolean)"
                      size="small"
                    >
                      启用
                    </el-checkbox>
                  </div>

                  <div class="flex gap-2 items-start">
                    <el-input
                      type="textarea"
                      :rows="1"
                      autosize
                      placeholder="请输入命令内容"
                      :model-value="topologyCommandValue(cmd.fieldKey, cmd.command)"
                      @input="onTopologyCommandInput(cmd.fieldKey, $event)"
                      class="flex-1 font-mono text-xs"
                    />
                    <div class="flex flex-col gap-1 items-end">
                      <el-input-number
                        :model-value="topologyTimeoutValue(cmd.fieldKey, cmd.timeoutSec)"
                        @change="onTopologyTimeoutInput(cmd.fieldKey, $event)"
                        :min="0"
                        controls-position="right"
                        class="w-24 text-xs"
                        size="small"
                      />
                      <span class="text-[9px] text-text-muted">超时(秒)</span>
                    </div>
                    <el-button @click="resetTopologyOverride(cmd.fieldKey)" size="small" type="warning" plain>
                      重置
                    </el-button>
                  </div>
                </div>
              </div>

              <div class="flex items-center justify-between text-xs pt-2 border-t border-border flex-shrink-0">
                <span class="text-text-muted">已覆盖 {{ topologyOverrides.length }} 个字段</span>
                <span v-if="topologyInvalidCount > 0" class="text-error font-medium">
                  存在 {{ topologyInvalidCount }} 项已启用但命令为空的覆盖
                </span>
                <span v-else class="text-success font-medium flex items-center gap-1">
                  <el-icon><Check /></el-icon> 校验通过
                </span>
              </div>
            </div>
          </div>
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
      class="rounded-xl"
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
import { ref, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { Plus, Position, Right, Cpu, Connection, Folder, Tickets, Refresh, Setting, Delete, Check } from "@element-plus/icons-vue";
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
