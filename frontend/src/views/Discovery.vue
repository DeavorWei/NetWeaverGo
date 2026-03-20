<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <p class="text-sm text-text-muted">
        选择设备启动网络拓扑发现任务，采集设备信息和LLDP邻居数据
      </p>
      <div class="flex gap-3">
        <button
          v-if="isRunning"
          @click="cancelDiscovery"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 shadow-card bg-error/10 border border-error/30 text-error hover:bg-error/20"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
          </svg>
          取消任务
        </button>
        <button
          v-else
          @click="openStartModal"
          :disabled="!canStart"
          class="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold transition-all duration-200 shadow-card"
          :class="
            !canStart
              ? 'bg-bg-card border border-border text-text-muted cursor-not-allowed'
              : 'bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow'
          "
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          启动发现
        </button>
      </div>
    </div>

    <!-- 主内容区域 -->
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
      <div class="flex-1 overflow-y-auto scrollbar-custom pr-1">
        <!-- 步骤1: 选择目标设备 -->
        <div
          class="bg-bg-card border border-border rounded-xl overflow-hidden"
          :style="{ height: devicePanelHeight + 'px', minHeight: '200px' }"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <div
              class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent"
            >
              1
            </div>
            <span class="text-sm font-medium text-text-primary"
              >选择目标设备</span
            >
            <span
              v-if="selectedDevices.length > 0"
              class="ml-2 text-xs text-accent font-mono"
              >已选 {{ selectedDevices.length }} 台</span
            >
            <button
              @click="goToDevices"
              class="ml-auto text-xs text-accent hover:text-accent-glow transition-colors"
            >
              管理设备资产
            </button>
          </div>
          <div class="p-3 h-[calc(100%-45px)] overflow-y-auto scrollbar-custom">
            <DeviceSelector
              :devices="deviceList"
              @selectionChange="onDeviceSelectionChange"
            />
          </div>
        </div>

        <!-- 可拖拽分隔条 -->
        <div
          class="h-2 flex items-center justify-center cursor-row-resize group py-1"
          @mousedown="startResize"
        >
          <div
            class="w-16 h-1.5 rounded-full bg-border group-hover:bg-accent/50 transition-colors"
          ></div>
        </div>

        <!-- 步骤2: 配置发现参数 -->
        <div
          class="bg-bg-card border border-border rounded-xl overflow-hidden mb-4"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <div
              class="w-6 h-6 rounded-full bg-accent/15 flex items-center justify-center text-xs font-semibold text-accent"
            >
              2
            </div>
            <span class="text-sm font-medium text-text-primary"
              >发现参数配置</span
            >
          </div>
          <div class="p-4 space-y-4">
            <!-- 厂商选择 -->
            <div>
              <label
                class="block text-xs font-medium text-text-secondary mb-1.5"
                >目标厂商</label
              >
              <select
                v-model="selectedVendor"
                class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
              >
                <option value="">自动识别</option>
                <option
                  v-for="vendor in supportedVendors"
                  :key="vendor"
                  :value="vendor"
                >
                  {{ getVendorName(vendor) }}
                </option>
              </select>
              <p class="mt-1 text-xs text-text-muted">
                选择设备厂商以使用对应的命令集，留空则自动识别
              </p>
            </div>

            <!-- 并发和超时配置 -->
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label
                  class="block text-xs font-medium text-text-secondary mb-1.5"
                  >并发数</label
                >
                <input
                  v-model.number="maxWorkers"
                  type="number"
                  min="1"
                  max="64"
                  class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
                />
              </div>
              <div>
                <label
                  class="block text-xs font-medium text-text-secondary mb-1.5"
                  >超时时间(秒)</label
                >
                <input
                  v-model.number="timeoutSec"
                  type="number"
                  min="10"
                  max="300"
                  class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
                />
              </div>
            </div>

            <!-- 厂商命令预览 -->
            <div
              v-if="selectedVendor && vendorProfile"
              class="bg-bg-panel border border-border rounded-lg p-3"
            >
              <h4 class="text-xs font-medium text-text-secondary mb-2">
                将执行的命令
              </h4>
              <div class="space-y-1">
                <div
                  v-for="cmd in vendorProfile.commands"
                  :key="cmd.commandKey"
                  class="flex items-center gap-2 text-xs"
                >
                  <span
                    class="px-1.5 py-0.5 rounded bg-accent/10 text-accent font-mono"
                    >{{ cmd.commandKey }}</span
                  >
                  <span class="text-text-muted font-mono">{{
                    cmd.command
                  }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 历史任务列表 -->
        <div
          v-if="taskList.length > 0"
          class="bg-bg-card border border-border rounded-xl overflow-hidden"
        >
          <div
            class="flex items-center gap-3 px-4 py-2.5 border-b border-border bg-bg-panel"
          >
            <span class="text-sm font-medium text-text-primary"
              >历史发现任务</span
            >
            <span class="text-xs text-text-muted"
              >最近 {{ taskList.length }} 条</span
            >
          </div>
          <div class="divide-y divide-border">
            <div
              v-for="task in taskList"
              :key="task.id"
              class="px-4 py-3 hover:bg-bg-hover cursor-pointer transition-colors"
              @click="viewTaskDetail(task)"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <span
                    class="px-2 py-0.5 rounded text-xs font-medium"
                    :class="getStatusClass(task.status)"
                  >
                    {{ getStatusText(task.status) }}
                  </span>
                  <span class="text-sm font-medium text-text-primary">{{
                    task.name || task.id
                  }}</span>
                  <span class="text-xs text-text-muted">{{
                    task.vendor || "自动识别"
                  }}</span>
                </div>
                <div class="text-xs text-text-muted">
                  {{ formatTime(task.createdAt) }}
                </div>
              </div>
              <div class="mt-1 flex items-center gap-4 text-xs text-text-muted">
                <span>设备: {{ task.totalCount }}</span>
                <span class="text-success">成功: {{ task.successCount }}</span>
                <span class="text-error">失败: {{ task.failedCount }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 启动确认弹窗 -->
    <Transition name="modal">
      <div
        v-if="startModal.show"
        class="fixed inset-0 z-50 flex items-center justify-center"
      >
        <div
          class="absolute inset-0 bg-black/60 backdrop-blur-sm"
          @click="startModal.show = false"
        ></div>
        <div
          class="relative bg-bg-card border border-border rounded-xl shadow-2xl max-w-lg w-full mx-4 overflow-hidden animate-slide-in"
        >
          <!-- 弹窗头部 -->
          <div
            class="flex items-center justify-between px-5 py-4 border-b border-border bg-bg-panel"
          >
            <h3 class="text-sm font-semibold text-text-primary">
              确认启动发现任务
            </h3>
            <button
              @click="startModal.show = false"
              class="text-text-muted hover:text-text-primary transition-colors"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>

          <!-- 弹窗内容 -->
          <div class="px-5 py-4 space-y-4">
            <div
              class="bg-bg-panel border border-border rounded-lg p-3 space-y-2"
            >
              <div class="flex items-center justify-between text-xs">
                <span class="text-text-secondary">目标设备</span>
                <span class="text-accent font-mono"
                  >{{ selectedDevices.length }} 台</span
                >
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="text-text-secondary">目标厂商</span>
                <span class="text-text-primary">{{
                  getVendorName(selectedVendor) || "自动识别"
                }}</span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="text-text-secondary">并发数</span>
                <span class="text-text-primary font-mono">{{
                  maxWorkers
                }}</span>
              </div>
              <div class="flex items-center justify-between text-xs">
                <span class="text-text-secondary">超时时间</span>
                <span class="text-text-primary font-mono"
                  >{{ timeoutSec }}s</span
                >
              </div>
            </div>

            <div class="flex flex-wrap gap-1">
              <span
                v-for="dev in selectedDevices.slice(0, 10)"
                :key="dev.ip"
                class="text-xs font-mono px-1.5 py-0.5 rounded bg-bg-panel border border-border text-text-muted"
                >{{ dev.ip }}</span
              >
              <span
                v-if="selectedDevices.length > 10"
                class="text-xs text-text-muted"
                >+{{ selectedDevices.length - 10 }} 台</span
              >
            </div>
          </div>

          <!-- 弹窗底部 -->
          <div class="flex justify-end gap-3 px-5 py-4 border-t border-border">
            <button
              @click="startModal.show = false"
              class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-panel border border-border text-text-secondary hover:text-text-primary transition-all"
            >
              取消
            </button>
            <button
              @click="confirmStart"
              class="px-5 py-2 rounded-lg text-sm font-semibold bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow transition-all duration-200"
            >
              确认启动
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 任务进度弹窗 -->
    <Transition name="modal">
      <div
        v-if="progressModal.show"
        class="fixed inset-0 z-50 flex items-center justify-center"
      >
        <div
          class="absolute inset-0 bg-black/60 backdrop-blur-sm"
          @click="progressModal.show = false"
        ></div>
        <div
          class="relative bg-bg-card border border-border rounded-xl shadow-2xl max-w-2xl w-full mx-4 overflow-hidden animate-slide-in"
        >
          <!-- 弹窗头部 -->
          <div
            class="flex items-center justify-between px-5 py-4 border-b border-border bg-bg-panel"
          >
            <h3 class="text-sm font-semibold text-text-primary">
              发现任务进行中
            </h3>
            <button
              @click="progressModal.show = false"
              class="text-text-muted hover:text-text-primary transition-colors"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>

          <!-- 进度内容 -->
          <div
            class="px-5 py-4 space-y-4 max-h-[60vh] overflow-auto scrollbar-custom"
          >
            <!-- 总体进度 -->
            <div class="bg-bg-panel border border-border rounded-lg p-4">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium text-text-primary"
                  >总体进度</span
                >
                <span class="text-xs text-text-muted"
                  >{{ progressModal.finished }} /
                  {{ progressModal.total }}</span
                >
              </div>
              <div class="w-full h-2 bg-bg-card rounded-full overflow-hidden">
                <div
                  class="h-full bg-accent transition-all duration-300"
                  :style="{ width: `${progressPercent}%` }"
                ></div>
              </div>
              <div
                class="mt-2 flex items-center justify-between text-xs text-text-muted"
              >
                <span>成功: {{ progressModal.success }}</span>
                <span class="text-warning"
                  >部分成功: {{ progressModal.partial }}</span
                >
                <span>失败: {{ progressModal.failed }}</span>
              </div>
            </div>

            <!-- 设备列表 -->
            <div class="space-y-2">
              <h4 class="text-xs font-medium text-text-secondary">设备状态</h4>
              <div
                v-for="device in progressModal.devices"
                :key="device.deviceIp"
                class="flex items-center justify-between px-3 py-2 bg-bg-panel border border-border rounded-lg"
              >
                <div class="flex items-center gap-2">
                  <span
                    class="w-2 h-2 rounded-full"
                    :class="{
                      'bg-warning animate-pulse': device.status === 'running',
                      'bg-success': device.status === 'success',
                      'bg-warning': device.status === 'partial',
                      'bg-error': device.status === 'failed',
                      'bg-text-muted': device.status === 'pending',
                    }"
                  ></span>
                  <span class="text-sm font-mono text-text-primary">{{
                    device.deviceIp
                  }}</span>
                  <span v-if="device.vendor" class="text-xs text-text-muted">{{
                    device.vendor
                  }}</span>
                </div>
                <span
                  class="text-xs"
                  :class="{
                    'text-warning':
                      device.status === 'running' ||
                      device.status === 'partial',
                    'text-success': device.status === 'success',
                    'text-error': device.status === 'failed',
                    'text-text-muted': device.status === 'pending',
                  }"
                >
                  {{ getDeviceStatusText(device.status) }}
                </span>
              </div>
            </div>
          </div>

          <!-- 弹窗底部 -->
          <div class="flex justify-end gap-3 px-5 py-4 border-t border-border">
            <button
              v-if="
                (progressModal.status === 'completed' ||
                  progressModal.status === 'partial' ||
                  progressModal.status === 'failed') &&
                progressModal.failed > 0 &&
                !isRunning
              "
              @click="retryFailedDevices"
              class="px-4 py-2 rounded-lg text-sm font-medium bg-warning/10 border border-warning/30 text-warning hover:bg-warning/20 transition-all"
            >
              重试失败设备
            </button>
            <button
              @click="cancelDiscovery"
              :disabled="!isRunning"
              class="px-4 py-2 rounded-lg text-sm font-medium bg-error/10 border border-error/30 text-error hover:bg-error/20 transition-all"
            >
              取消任务
            </button>
            <button
              v-if="
                progressModal.status === 'completed' ||
                progressModal.status === 'partial' ||
                progressModal.status === 'failed'
              "
              @click="progressModal.show = false"
              class="px-5 py-2 rounded-lg text-sm font-semibold bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow transition-all duration-200"
            >
              关闭
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Toast 通知 -->
    <Transition name="toast">
      <div
        v-if="showToast"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50"
      >
        <div
          class="flex items-center gap-2 px-5 py-3 rounded-xl shadow-2xl border"
          :class="
            toastType === 'success'
              ? 'bg-success/10 border-success/30 text-success'
              : 'bg-error/10 border-error/30 text-error'
          "
        >
          <svg
            v-if="toastType === 'success'"
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10" />
            <line x1="15" y1="9" x2="9" y2="15" />
            <line x1="9" y1="9" x2="15" y2="15" />
          </svg>
          <span class="text-sm font-medium">{{ toastMessage }}</span>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import {
  DeviceAPI,
  DiscoveryAPI,
  SettingsAPI,
  type DeviceAsset,
  type DiscoveryTaskView,
  type DiscoveryDeviceView,
  type VendorCommandProfile,
} from "../services/api";
import DeviceSelector from "../components/task/DeviceSelector.vue";

const router = useRouter();

// 设备列表和选择状态
const deviceList = ref<DeviceAsset[]>([]);
const selectedDevices = ref<DeviceAsset[]>([]);

// 发现参数
const selectedVendor = ref("");
const maxWorkers = ref(10);
const timeoutSec = ref(60);
const supportedVendors = ref<string[]>([]);
const vendorProfiles = ref<VendorCommandProfile[]>([]);

// 运行状态
const isRunning = ref(false);
const currentTaskId = ref("");

// 任务列表
const taskList = ref<DiscoveryTaskView[]>([]);

// 面板高度控制
const devicePanelHeight = ref(280);
const minHeight = 150;

// 拖拽调整高度相关
let isResizing = false;
let startY = 0;
let startHeight = 0;

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

// 是否可以启动
const canStart = computed(() => {
  return selectedDevices.value.length > 0 && !isRunning.value;
});

// 当前选中的厂商配置
const vendorProfile = computed(() => {
  if (!selectedVendor.value) return null;
  return (
    vendorProfiles.value.find((p) => p.vendor === selectedVendor.value) || null
  );
});

// 进度百分比
const progressPercent = computed(() => {
  if (progressModal.value.total === 0) return 0;
  return Math.round(
    (progressModal.value.finished / progressModal.value.total) * 100,
  );
});

// Toast 通知
const showToast = ref(false);
const toastMessage = ref("");
const toastType = ref<"success" | "error">("success");
let toastTimer: ReturnType<typeof setTimeout> | null = null;

function triggerToast(msg: string, type: "success" | "error" = "success") {
  toastMessage.value = msg;
  toastType.value = type;
  showToast.value = true;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    showToast.value = false;
  }, 3000);
}

// 启动弹窗
const startModal = ref({
  show: false,
});

// 进度弹窗
const progressModal = ref({
  show: false,
  taskId: "",
  status: "",
  phase: "",
  phaseProgress: 0,
  total: 0,
  finished: 0,
  success: 0,
  partial: 0,
  failed: 0,
  devices: [] as DiscoveryDeviceView[],
});

function openStartModal() {
  if (!canStart.value) return;
  startModal.value.show = true;
}

async function confirmStart() {
  startModal.value.show = false;

  try {
    const req = {
      deviceIds: selectedDevices.value.map((d) => String(d.id)),
      groupNames: [],
      vendor: selectedVendor.value,
      maxWorkers: maxWorkers.value,
      timeoutSec: timeoutSec.value,
    };

    const result = await DiscoveryAPI.startDiscovery(req);
    currentTaskId.value = result.taskId;
    isRunning.value = true;

    // 打开进度弹窗
    progressModal.value = {
      show: true,
      taskId: result.taskId,
      status: "running",
      phase: "collecting",
      phaseProgress: 0,
      total: selectedDevices.value.length,
      finished: 0,
      success: 0,
      partial: 0,
      failed: 0,
      devices: [],
    };

    triggerToast("发现任务已启动", "success");

    // 开始轮询状态
    pollTaskStatus();
  } catch (err: any) {
    console.error("启动发现任务失败:", err);
    triggerToast(`启动失败: ${err?.message || err}`, "error");
  }
}

async function cancelDiscovery() {
  if (!currentTaskId.value) return;

  try {
    await DiscoveryAPI.cancelDiscovery(currentTaskId.value);
    isRunning.value = false;
    triggerToast("任务已取消", "success");
  } catch (err: any) {
    console.error("取消任务失败:", err);
    triggerToast(`取消失败: ${err?.message || err}`, "error");
  }
}

async function retryFailedDevices() {
  if (!progressModal.value.taskId || isRunning.value) return;
  try {
    await DiscoveryAPI.retryFailedDevices(progressModal.value.taskId);
    currentTaskId.value = progressModal.value.taskId;
    isRunning.value = true;
    progressModal.value.status = "running";
    triggerToast("已启动失败设备重试", "success");
    pollTaskStatus();
  } catch (err: any) {
    console.error("重试失败设备失败:", err);
    triggerToast(`重试失败: ${err?.message || err}`, "error");
  }
}

let pollTimer: ReturnType<typeof setInterval> | null = null;

async function pollTaskStatus() {
  if (pollTimer) clearInterval(pollTimer);

  pollTimer = setInterval(async () => {
    if (!currentTaskId.value) {
      if (pollTimer) clearInterval(pollTimer);
      return;
    }

    try {
      const status = await DiscoveryAPI.getTaskStatus(currentTaskId.value);
      if (status) {
        progressModal.value.status = status.status;
        progressModal.value.total = status.totalCount;
        progressModal.value.finished = status.successCount + status.failedCount;
        progressModal.value.success = status.successCount;
        progressModal.value.failed = status.failedCount;

        // 获取设备列表
        const devices = await DiscoveryAPI.getTaskDevices(currentTaskId.value);
        progressModal.value.devices = devices || [];
        progressModal.value.partial = (devices || []).filter(
          (d: { status: string }) => d.status === "partial",
        ).length;

        // 检查是否完成
        if (
          status.status === "completed" ||
          status.status === "partial" ||
          status.status === "failed" ||
          status.status === "cancelled"
        ) {
          isRunning.value = false;
          if (pollTimer) clearInterval(pollTimer);
          await loadTaskList();
        }
      }
    } catch (err) {
      console.error("轮询任务状态失败:", err);
    }
  }, 2000);
}

async function viewTaskDetail(task: DiscoveryTaskView) {
  try {
    const devices = await DiscoveryAPI.getTaskDevices(task.id);
    progressModal.value = {
      show: true,
      taskId: task.id,
      status: task.status,
      phase: (task as any).phase || "completed",
      phaseProgress: (task as any).phaseProgress || 100,
      total: task.totalCount,
      finished: task.successCount + task.failedCount,
      success: task.successCount,
      partial: (devices || []).filter(
        (d: { status: string }) => d.status === "partial",
      ).length,
      failed: task.failedCount,
      devices: devices || [],
    };
  } catch (err) {
    console.error("获取任务详情失败:", err);
  }
}

// 设备选择变化
function onDeviceSelectionChange(devs: DeviceAsset[]) {
  selectedDevices.value = devs;
}

// 导航
function goToDevices() {
  router.push("/devices");
}

// 厂商名称映射
function getVendorName(vendor: string): string {
  const map: Record<string, string> = {
    huawei: "华为",
    h3c: "华三",
    cisco: "思科",
  };
  return map[vendor] || vendor;
}

// 状态样式
function getStatusClass(status: string): string {
  const map: Record<string, string> = {
    pending: "bg-text-muted/20 text-text-muted",
    running: "bg-warning/20 text-warning",
    completed: "bg-success/20 text-success",
    partial: "bg-warning/20 text-warning",
    failed: "bg-error/20 text-error",
    cancelled: "bg-text-muted/20 text-text-muted",
  };
  return map[status] || "bg-text-muted/20 text-text-muted";
}

function getStatusText(status: string): string {
  const map: Record<string, string> = {
    pending: "等待中",
    running: "运行中",
    completed: "已完成",
    partial: "部分成功",
    failed: "失败",
    cancelled: "已取消",
  };
  return map[status] || status;
}

function getDeviceStatusText(status: string): string {
  const map: Record<string, string> = {
    pending: "等待中",
    running: "采集ing...",
    success: "成功",
    partial: "部分成功",
    failed: "失败",
  };
  return map[status] || status;
}

function formatTime(time: any): string {
  if (!time) return "";
  const date = new Date(time);
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// 加载设备列表
async function loadDevices() {
  try {
    const result = await DeviceAPI.listDevices();
    deviceList.value = result || [];
  } catch (err) {
    console.error("加载设备列表失败:", err);
    deviceList.value = [];
  }
}

// 加载厂商配置
async function loadVendorProfiles() {
  try {
    const [vendors, profiles] = await Promise.all([
      DiscoveryAPI.getSupportedVendors(),
      DiscoveryAPI.getVendorProfiles(),
    ]);
    supportedVendors.value = vendors || [];
    vendorProfiles.value = (profiles || []).filter(
      (p: VendorCommandProfile | null): p is VendorCommandProfile => p !== null,
    );
  } catch (err) {
    console.error("加载厂商配置失败:", err);
  }
}

async function loadDiscoveryDefaults() {
  try {
    const runtime = await SettingsAPI.getRuntimeConfig();
    if (runtime?.discovery?.workerCount && runtime.discovery.workerCount > 0) {
      maxWorkers.value = runtime.discovery.workerCount;
    }
    if (
      runtime?.discovery?.commandTimeout &&
      runtime.discovery.commandTimeout > 0
    ) {
      timeoutSec.value = Math.max(
        1,
        Math.round(runtime.discovery.commandTimeout / 1000),
      );
    }
  } catch (err) {
    console.error("加载发现默认配置失败:", err);
  }
}

// 加载任务列表
async function loadTaskList() {
  try {
    const result = await DiscoveryAPI.listDiscoveryTasks(10);
    taskList.value = result || [];
  } catch (err) {
    console.error("加载任务列表失败:", err);
    taskList.value = [];
  }
}

// 检查运行状态
async function checkRunningStatus() {
  try {
    const running = await DiscoveryAPI.isDiscoveryRunning();
    if (running) {
      const taskId = await DiscoveryAPI.getCurrentDiscoveryTask();
      if (taskId) {
        currentTaskId.value = taskId;
        isRunning.value = true;
        // 恢复进度显示
        const status = await DiscoveryAPI.getTaskStatus(taskId);
        if (status) {
          progressModal.value = {
            show: true,
            taskId: taskId,
            status: status.status,
            phase: (status as any).phase || "collecting",
            phaseProgress: (status as any).phaseProgress || 0,
            total: status.totalCount,
            finished: status.successCount + status.failedCount,
            success: status.successCount,
            partial: 0,
            failed: status.failedCount,
            devices: [],
          };
          pollTaskStatus();
        }
      }
    }
  } catch (err) {
    console.error("检查运行状态失败:", err);
  }
}

onMounted(() => {
  loadDiscoveryDefaults();
  loadDevices();
  loadVendorProfiles();
  loadTaskList();
  checkRunningStatus();
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
  if (toastTimer) clearTimeout(toastTimer);
});
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.toast-enter-active {
  transition: all 0.3s ease-out;
}
.toast-leave-active {
  transition: all 0.2s ease-in;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(-50%) translateY(20px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(10px);
}
</style>
