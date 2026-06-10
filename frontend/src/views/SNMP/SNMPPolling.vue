<script setup lang="ts">
/**
 * SNMP 设备轮询管理页面（重构版）
 *
 * 功能：
 * - 调度器状态监控和启停控制
 * - 凭据管理（增删改查）
 * - 模板管理（增删改查，OID 配置）
 * - 目标管理（增删改查，启用/禁用，立即轮询）
 * - 轮询结果列表展示（支持分页、筛选）
 * - 轮询结果详情查看
 * - 实时轮询结果推送和高亮显示
 * - 可拖拽面板布局 + 折叠展开
 * - 自动结果更新（Wails 事件订阅）
 */
import { ref, computed, onMounted } from "vue";
import { ElMessageBox, ElMessage } from "element-plus";
import { SNMPPollingAPI } from "@/services/snmpApi";
import { useSNMPPolling } from "@/composables/useSNMPPolling";
import { usePanelResize } from "@/composables/usePanelResize";
import { getLogger } from "@/utils/logger";
import type {
  CredentialVM,
  PollingTemplateVM,
  PollingTargetVM,
  PollingResultVM,
  CreateCredentialRequest,
  UpdateCredentialRequest,
  CreatePollingTemplateRequest,
  UpdatePollingTemplateRequest,
  CreatePollingTargetRequest,
  UpdatePollingTargetRequest,
  ConcurrencyConfigVM,
} from "@/bindings/github.com/NetWeaverGo/core/internal/ui/models";
import { PollingResultFilterVM } from "@/bindings/github.com/NetWeaverGo/core/internal/ui/models";
import type { PollingResultEvent } from "@/composables/useSNMPPolling";

const logger = getLogger();

// ==================== 组合式函数 ====================

/** 自动刷新防抖定时器 */
let autoRefreshTimer: ReturnType<typeof setTimeout> | null = null;
const AUTO_REFRESH_DEBOUNCE = 500;

/**
 * 轮询结果事件回调 - 收到新结果时自动刷新结果列表（500ms 防抖）
 */
function handlePollingResultEvent(_event: PollingResultEvent) {
  if (autoRefreshTimer) clearTimeout(autoRefreshTimer);
  autoRefreshTimer = setTimeout(() => {
    loadPollingResults();
    loadTargets();  // 同时刷新目标列表状态（lastPollAt、lastPollStatus 等）
  }, AUTO_REFRESH_DEBOUNCE);
}

const {
  schedulerStatus,
  targets,
  credentials,
  templates,
  targetsLoading,
  isRunning,
  enabledTargetCount,
  targetCount,
  successTargetCount,
  loadTargets,
  loadCredentials,
  loadTemplates,
  loadAll,
  startScheduler,
  stopScheduler,
  pollNow,
  pollAllNow,
  toggleTarget,
  loadTargetStats,
  isNewResult,
  getCredentialById,
  getTemplateById,
  getLatestResult,
  getTargetStats,
} = useSNMPPolling(handlePollingResultEvent);

// ==================== 面板拖拽布局 ====================

const mainContainerRef = ref<HTMLElement | null>(null);

/** 水平面板：左侧面板 | 中间主面板 | 右侧面板 */
const horizontalResize = usePanelResize(mainContainerRef, "horizontal", [
  { initialSize: 180, minSize: 0, maxSize: 500 }, // 左侧凭据/模板面板
  { initialSize: 600, minSize: 300 }, // 中间主面板
  { initialSize: 200, minSize: 0, maxSize: 600 }, // 右侧详情面板
]);

/** 垂直面板：目标列表 | 结果列表 */
const centerContainerRef = ref<HTMLElement | null>(null);
const verticalResize = usePanelResize(centerContainerRef, "vertical", [
  { initialSize: 400, minSize: 150 }, // 目标列表
  { initialSize: 256, minSize: 120 }, // 结果列表
]);

// ==================== 状态 ====================

/** 轮询结果列表 */
const pollingResults = ref<PollingResultVM[]>([]);
const resultsLoading = ref(false);
const totalResults = ref(0);
const currentPage = ref(1);
const pageSize = ref(20);
const totalPages = ref(0);

/** 选中的目标 */
const selectedTarget = ref<PollingTargetVM | null>(null);
const selectedResult = ref<PollingResultVM | null>(null);

/** 结果过滤条件 */
const resultFilter = ref<PollingResultFilterVM>(new PollingResultFilterVM());

/** 目标过滤条件 */
const targetFilterEnabled = ref<boolean | undefined>(undefined);

/** 模态框状态 */
const showCredentialModal = ref(false);
const showTemplateModal = ref(false);
const showTargetModal = ref(false);

/** 编辑状态 */
const editingCredential = ref<CredentialVM | null>(null);
const editingTemplate = ref<PollingTemplateVM | null>(null);
const editingTarget = ref<PollingTargetVM | null>(null);

/** 操作加载状态 */
const schedulerOperating = ref(false);
const pollingAll = ref(false);
const pollingTargetIds = ref<Set<number>>(new Set());

/** 凭据表单 */
const credentialForm = ref<CreateCredentialRequest & { id?: number }>({
  name: "",
  version: "v2c",
  community: "public",
  securityLevel: "noAuthNoPriv",
  username: "",
  authProtocol: "MD5",
  authPassword: "",
  privProtocol: "AES",
  privPassword: "",
  contextName: "",
  contextEngineId: "",
});

/** 模板表单 */
const templateForm = ref<CreatePollingTemplateRequest & { id?: number }>({
  name: "",
  description: "",
  category: "",
  oidItems: [],
});

/** 目标表单 */
const targetForm = ref<CreatePollingTargetRequest & { id?: number }>({
  targetIP: "",
  targetPort: 161,
  displayName: "",
  credentialId: null,
  templateId: null,
  pollInterval: 60,
  enabled: true,
});

/** Community 显示/隐藏映射 */
const showCommunityMap = ref<Map<number, boolean>>(new Map());

/** 左侧面板激活的标签页 */
const leftActiveTab = ref<"credentials" | "templates">("credentials");

// ==================== 并发控制配置 ====================

/** 并发配置表单 */
const concurrencyConfig = ref<ConcurrencyConfigVM>({
  maxDevices: 10,
  maxOpsPerDevice: 1,
  skipIfBusy: true,
  queueTimeoutSecs: 30,
});

/** 并发配置加载/保存状态 */
const concurrencyLoading = ref(false);
const concurrencySaving = ref(false);

/** 并发控制设置弹窗显示状态 */
const showConcurrencySettingsModal = ref(false);

// ==================== 计算属性 ====================

/** 目标过滤选项 */
const targetEnabledOptions = [
  { value: undefined, label: "全部" },
  { value: true, label: "已启用" },
  { value: false, label: "已禁用" },
];

/** 过滤后的目标列表 */
const filteredTargets = computed(() => {
  if (targetFilterEnabled.value === undefined) return targets.value;
  return targets.value.filter((t) => t.enabled === targetFilterEnabled.value);
});

/** 选中目标的统计信息 */
const selectedTargetStats = computed(() => {
  if (!selectedTarget.value) return null;
  return getTargetStats(selectedTarget.value.id) ?? null;
});

// ==================== 方法 ====================

/**
 * 加载轮询结果列表
 */
async function loadPollingResults() {
  resultsLoading.value = true;
  try {
    const result = await SNMPPollingAPI.getPollingResults(
      resultFilter.value,
      currentPage.value,
      pageSize.value,
    );
    if (result) {
      pollingResults.value = result.data;
      totalResults.value = result.total;
      totalPages.value = result.totalPages;
    }
  } catch (error) {
    logger.error("SNMP-Polling", "加载轮询结果失败", error);
  } finally {
    resultsLoading.value = false;
  }
}

/**
 * 选择目标
 */
function selectTarget(target: PollingTargetVM) {
  selectedTarget.value = target;
  // 加载该目标的统计信息
  loadTargetStats(target.id);
  // 设置结果过滤
  resultFilter.value = new PollingResultFilterVM({ targetId: target.id });
  currentPage.value = 1;
  loadPollingResults();
}

/**
 * 选择轮询结果
 */
function selectResult(result: PollingResultVM) {
  selectedResult.value = result;
}

/**
 * 启动调度器
 */
async function handleStartScheduler() {
  schedulerOperating.value = true;
  try {
    await startScheduler();
  } catch {
    // 错误已在 composable 中处理
  } finally {
    schedulerOperating.value = false;
  }
}

/**
 * 停止调度器
 */
async function handleStopScheduler() {
  schedulerOperating.value = true;
  try {
    await stopScheduler();
  } catch {
    // 错误已在 composable 中处理
  } finally {
    schedulerOperating.value = false;
  }
}

/**
 * 立即轮询单个目标
 */
async function handlePollNow(targetId: number) {
  pollingTargetIds.value.add(targetId);
  try {
    await pollNow(targetId);
  } catch {
    // 错误已在 composable 中处理
  } finally {
    pollingTargetIds.value.delete(targetId);
  }
}

/**
 * 立即轮询所有目标
 */
async function handlePollAllNow() {
  pollingAll.value = true;
  try {
    await pollAllNow();
  } catch {
    // 错误已在 composable 中处理
  } finally {
    pollingAll.value = false;
  }
}

/**
 * 切换目标启用状态
 */
async function handleToggleTarget(target: PollingTargetVM) {
  try {
    await toggleTarget(target.id, !target.enabled);
  } catch {
    // 错误已在 composable 中处理
  }
}

/**
 * 删除目标
 */
async function handleDeleteTarget(target: PollingTargetVM) {
  try {
    await ElMessageBox.confirm(
      `确定要删除目标「${target.displayName}」吗？`,
      "删除确认",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
  } catch {
    return; // 用户取消
  }
  try {
    await SNMPPollingAPI.deletePollingTarget(target.id);
    if (selectedTarget.value?.id === target.id) {
      selectedTarget.value = null;
    }
    await loadTargets();
  } catch (error) {
    logger.error("SNMP-Polling", "删除目标失败", error);
  }
}

// ==================== 凭据管理 ====================

/**
 * 打开凭据编辑对话框
 */
function openCredentialModal(credential: CredentialVM | null = null) {
  if (credential) {
    editingCredential.value = credential;
    credentialForm.value = {
      id: credential.id,
      name: credential.name,
      version: credential.version,
      community: credential.community,
      securityLevel: credential.securityLevel || "noAuthNoPriv",
      username: credential.username || "",
      authProtocol: credential.authProtocol || "MD5",
      authPassword: "", // 编辑时不回显密码
      privProtocol: credential.privProtocol || "AES",
      privPassword: "", // 编辑时不回显密码
      contextName: credential.contextName || "",
      contextEngineId: credential.contextEngineId || "",
    };
  } else {
    editingCredential.value = null;
    credentialForm.value = {
      name: "",
      version: "v2c",
      community: "public",
      securityLevel: "noAuthNoPriv",
      username: "",
      authProtocol: "MD5",
      authPassword: "",
      privProtocol: "AES",
      privPassword: "",
      contextName: "",
      contextEngineId: "",
    };
  }
  showCredentialModal.value = true;
}

/**
 * 保存凭据
 */
async function saveCredential() {
  try {
    if (editingCredential.value) {
      const req: UpdateCredentialRequest = {
        name: credentialForm.value.name,
        version: credentialForm.value.version,
        community: credentialForm.value.community,
        securityLevel: credentialForm.value.securityLevel,
        username: credentialForm.value.username,
        authProtocol: credentialForm.value.authProtocol,
        authPassword: credentialForm.value.authPassword,
        privProtocol: credentialForm.value.privProtocol,
        privPassword: credentialForm.value.privPassword,
        contextName: credentialForm.value.contextName,
        contextEngineId: credentialForm.value.contextEngineId,
      };
      await SNMPPollingAPI.updateCredential(editingCredential.value.id, req);
    } else {
      const req: CreateCredentialRequest = {
        name: credentialForm.value.name,
        version: credentialForm.value.version,
        community: credentialForm.value.community,
        securityLevel: credentialForm.value.securityLevel,
        username: credentialForm.value.username,
        authProtocol: credentialForm.value.authProtocol,
        authPassword: credentialForm.value.authPassword,
        privProtocol: credentialForm.value.privProtocol,
        privPassword: credentialForm.value.privPassword,
        contextName: credentialForm.value.contextName,
        contextEngineId: credentialForm.value.contextEngineId,
      };
      await SNMPPollingAPI.createCredential(req);
    }
    showCredentialModal.value = false;
    await loadCredentials();
  } catch (error) {
    logger.error("SNMP-Polling", "保存凭据失败", error);
  }
}

/**
 * 删除凭据
 */
async function deleteCredential(credential: CredentialVM) {
  try {
    await ElMessageBox.confirm(
      `确定要删除凭据「${credential.name}」吗？`,
      "删除确认",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
  } catch {
    return; // 用户取消
  }
  try {
    await SNMPPollingAPI.deleteCredential(Number(credential.id));
    await loadCredentials();
  } catch (error) {
    logger.error("SNMP-Polling", "删除凭据失败", error);
  }
}

/**
 * 切换 Community 显示/隐藏
 */
function toggleCommunityVisibility(id: number) {
  showCommunityMap.value.set(id, !(showCommunityMap.value.get(id) ?? false));
}

/**
 * 获取 Community 显示文本
 */
function getCommunityDisplay(credential: CredentialVM): string {
  const visible = showCommunityMap.value.get(credential.id) ?? false;
  return visible ? credential.community : "••••••••";
}

// ==================== 模板管理 ====================

/**
 * 打开模板编辑对话框
 */
function openTemplateModal(template: PollingTemplateVM | null = null) {
  if (template) {
    editingTemplate.value = template;
    templateForm.value = {
      id: template.id,
      name: template.name,
      description: template.description,
      category: template.category,
      oidItems: [...template.oidItems],
    };
  } else {
    editingTemplate.value = null;
    templateForm.value = {
      name: "",
      description: "",
      category: "",
      oidItems: [],
    };
  }
  showTemplateModal.value = true;
}

/**
 * 添加 OID 到模板
 */
function addOIDToTemplate() {
  templateForm.value.oidItems.push({
    oid: "",
    name: "",
    type: "",
    operation: "get",
    description: "",
  });
}

/**
 * 从模板中移除 OID
 */
function removeOIDFromTemplate(index: number) {
  templateForm.value.oidItems.splice(index, 1);
}

/**
 * 保存模板
 */
async function saveTemplate() {
  try {
    if (editingTemplate.value) {
      const req: UpdatePollingTemplateRequest = {
        name: templateForm.value.name,
        description: templateForm.value.description,
        category: templateForm.value.category,
        oidItems: templateForm.value.oidItems,
      };
      await SNMPPollingAPI.updatePollingTemplate(editingTemplate.value.id, req);
    } else {
      const req: CreatePollingTemplateRequest = {
        name: templateForm.value.name,
        description: templateForm.value.description,
        category: templateForm.value.category,
        oidItems: templateForm.value.oidItems,
      };
      await SNMPPollingAPI.createPollingTemplate(req);
    }
    showTemplateModal.value = false;
    await loadTemplates();
  } catch (error) {
    logger.error("SNMP-Polling", "保存模板失败", error);
  }
}

/**
 * 删除模板
 */
async function deleteTemplate(template: PollingTemplateVM) {
  try {
    await ElMessageBox.confirm(
      `确定要删除模板「${template.name}」吗？`,
      "删除确认",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
  } catch {
    return; // 用户取消
  }
  try {
    await SNMPPollingAPI.deletePollingTemplate(template.id);
    await loadTemplates();
  } catch (error) {
    logger.error("SNMP-Polling", "删除模板失败", error);
  }
}

// ==================== 目标管理 ====================

/**
 * 打开目标编辑对话框
 */
function openTargetModal(target: PollingTargetVM | null = null) {
  if (target) {
    editingTarget.value = target;
    targetForm.value = {
      id: target.id,
      targetIP: target.targetIP,
      targetPort: target.targetPort,
      displayName: target.displayName,
      credentialId: target.credentialId,
      templateId: target.templateId,
      pollInterval: target.pollInterval,
      enabled: target.enabled,
    };
  } else {
    editingTarget.value = null;
    targetForm.value = {
      targetIP: "",
      targetPort: 161,
      displayName: "",
      credentialId: credentials.value[0]?.id ?? null,
      templateId: templates.value[0]?.id ?? null,
      pollInterval: 60,
      enabled: true,
    };
  }
  showTargetModal.value = true;
}

/**
 * 保存目标
 */
async function saveTarget() {
  try {
    if (editingTarget.value) {
      const req: UpdatePollingTargetRequest = {
        targetIP: targetForm.value.targetIP,
        targetPort: targetForm.value.targetPort,
        displayName: targetForm.value.displayName,
        credentialId: targetForm.value.credentialId,
        templateId: targetForm.value.templateId,
        pollInterval: targetForm.value.pollInterval,
        enabled: targetForm.value.enabled,
      };
      await SNMPPollingAPI.updatePollingTarget(editingTarget.value.id, req);
    } else {
      const req: CreatePollingTargetRequest = {
        targetIP: targetForm.value.targetIP,
        targetPort: targetForm.value.targetPort,
        displayName: targetForm.value.displayName,
        credentialId: targetForm.value.credentialId,
        templateId: targetForm.value.templateId,
        pollInterval: targetForm.value.pollInterval,
        enabled: targetForm.value.enabled,
      };
      await SNMPPollingAPI.createPollingTarget(req);
    }
    showTargetModal.value = false;
    await loadTargets();
  } catch (error) {
    logger.error("SNMP-Polling", "保存目标失败", error);
  }
}

/**
 * 切换页面
 */
function changePage(page: number) {
  currentPage.value = page;
  loadPollingResults();
}

/**
 * 格式化时间
 */
function formatTime(timeStr?: string): string {
  if (!timeStr) return "-";
  try {
    return new Date(timeStr).toLocaleString("zh-CN");
  } catch {
    return timeStr;
  }
}

/**
 * 格式化延迟
 */
function formatLatency(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

/**
 * 清理过期结果（使用 ElMessageBox.prompt 替代原生 prompt）
 */
async function clearOldResults() {
  try {
    const { value: days } = await ElMessageBox.prompt(
      "请输入要保留的天数（之前的记录将被删除）：",
      "清理过期结果",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        inputPattern: /^\d+$/,
        inputErrorMessage: "请输入有效的数字",
        inputPlaceholder: "30",
        inputValue: "30",
      },
    );

    const daysNum = parseInt(days);
    if (isNaN(daysNum)) return;

    const before = new Date();
    before.setDate(before.getDate() - daysNum);
    const beforeStr = before.toISOString().split("T")[0] || "";

    const count = await SNMPPollingAPI.clearPollingResults(beforeStr);
    await loadPollingResults();
    ElMessage.success(`已清理 ${count} 条结果`);
    logger.info(`SNMP-Polling: 已清理 ${count} 条结果`);
  } catch {
    // 用户取消或输入无效
  }
}

/**
 * 重置结果过滤
 */
function resetResultFilter() {
  resultFilter.value = new PollingResultFilterVM();
  currentPage.value = 1;
  loadPollingResults();
}

// ==================== 并发控制管理 ====================

/** 并发配置默认值 */
const DEFAULT_CONCURRENCY: ConcurrencyConfigVM = {
  maxDevices: 10,
  maxOpsPerDevice: 1,
  skipIfBusy: true,
  queueTimeoutSecs: 30,
};

/**
 * 加载并发控制配置
 */
async function loadConcurrencyConfig() {
  concurrencyLoading.value = true;
  try {
    const config = await SNMPPollingAPI.getConcurrencyConfig();
    if (config) {
      concurrencyConfig.value = { ...config };
    }
  } catch (error) {
    logger.error("SNMP-Polling", "加载并发配置失败", error);
  } finally {
    concurrencyLoading.value = false;
  }
}

/**
 * 保存并发控制配置
 */
async function saveConcurrencyConfig() {
  concurrencySaving.value = true;
  try {
    await SNMPPollingAPI.updateConcurrencyConfig(
      concurrencyConfig.value.maxDevices,
      concurrencyConfig.value.maxOpsPerDevice,
    );
    ElMessage.success("并发配置已保存");
    await loadConcurrencyConfig();
    logger.info("SNMP-Polling: 并发配置已保存");
  } catch (error) {
    logger.error("SNMP-Polling", "保存并发配置失败", error);
    ElMessage.error("保存并发配置失败");
  } finally {
    concurrencySaving.value = false;
  }
}

/**
 * 恢复默认并发配置
 */
async function resetConcurrencyConfig() {
  concurrencyConfig.value = { ...DEFAULT_CONCURRENCY };
  await saveConcurrencyConfig();
}

// ==================== 生命周期 ====================

onMounted(async () => {
  await loadAll();
  await loadPollingResults();
  await loadConcurrencyConfig();
});
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- 顶部工具栏 -->
    <header
      class="flex items-center justify-between px-4 py-3 bg-bg-secondary border-b border-border/50 flex-shrink-0"
    >
      <!-- 左侧：调度器状态 -->
      <div class="flex items-center gap-4">
        <!-- 状态指示器 -->
        <div class="flex items-center gap-2">
          <span
            :class="[
              'inline-block w-2.5 h-2.5 rounded-full',
              isRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-500',
            ]"
          ></span>
          <span
            class="text-sm font-medium"
            :class="isRunning ? 'text-green-400' : 'text-text-muted'"
          >
            {{ isRunning ? "运行中" : "已停止" }}
          </span>
        </div>

        <!-- 统计信息 -->
        <div
          class="flex items-center gap-3 px-3 py-1.5 bg-bg-tertiary rounded-md text-xs"
        >
          <span class="text-text-muted"
            >目标:
            <span class="text-text-primary font-medium">{{
              targetCount
            }}</span></span
          >
          <span class="text-text-muted"
            >启用:
            <span class="text-green-400 font-medium">{{
              enabledTargetCount
            }}</span></span
          >
          <span class="text-text-muted"
            >成功:
            <span class="text-green-400 font-medium">{{
              successTargetCount
            }}</span></span
          >
          <span class="text-text-muted"
            >总轮询:
            <span class="text-text-primary font-medium">{{
              schedulerStatus?.totalPolls ?? 0
            }}</span></span
          >
          <template v-if="schedulerStatus?.dispatcherStatus">
            <span class="text-text-muted"
              >并发:
              <span class="text-blue-400 font-medium"
                >{{ schedulerStatus.dispatcherStatus.activeDevices }}/{{
                  schedulerStatus.dispatcherStatus.maxDevices
                }}</span
              ></span
            >
            <span
              v-if="schedulerStatus.dispatcherStatus.waitingTasks > 0"
              class="text-text-muted"
              >排队:
              <span class="text-yellow-400 font-medium">{{
                schedulerStatus.dispatcherStatus.waitingTasks
              }}</span></span
            >
          </template>
        </div>

        <!-- 启停按钮 -->
        <button
          v-if="!isRunning"
          @click="handleStartScheduler"
          :disabled="schedulerOperating"
          class="px-3 py-1.5 text-sm bg-green-600 hover:bg-green-700 text-white rounded-md transition-colors disabled:opacity-50"
        >
          启动调度
        </button>
        <button
          v-else
          @click="handleStopScheduler"
          :disabled="schedulerOperating"
          class="px-3 py-1.5 text-sm bg-red-600 hover:bg-red-700 text-white rounded-md transition-colors disabled:opacity-50"
        >
          停止调度
        </button>
      </div>

      <!-- 右侧：操作按钮 -->
      <div class="flex items-center gap-2">
        <button
          @click="handlePollAllNow"
          :disabled="pollingAll || targetCount === 0"
          class="px-3 py-1.5 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors disabled:opacity-50"
        >
          {{ pollingAll ? "轮询中..." : "立即轮询全部" }}
        </button>

        <button
          @click="
            loadAll();
            loadPollingResults();
          "
          class="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-bg-tertiary hover:bg-bg-hover border border-border/50 text-text-secondary hover:text-text-primary transition-colors"
          title="手动刷新"
        >
          <span>刷新</span>
        </button>

        <button
          @click="showConcurrencySettingsModal = true"
          class="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-bg-tertiary hover:bg-bg-hover border border-border/50 text-text-secondary hover:text-text-primary transition-colors"
          title="并发控制设置"
        >
          <svg
            class="w-4 h-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="3" />
            <path
              d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-2 2 2 2 0 01-2-2v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 01-2-2 2 2 0 012-2h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 012-2 2 2 0 012 2v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 012 2 2 2 0 01-2 2h-.09a1.65 1.65 0 00-1.51 1z"
            />
          </svg>
          <span>设置</span>
        </button>
      </div>
    </header>

    <!-- 主内容区域：可拖拽三栏布局 -->
    <div
      ref="mainContainerRef"
      class="flex flex-1 min-h-0 overflow-hidden relative"
    >
      <!-- 左侧面板：凭据和模板 -->
      <aside
        :style="[horizontalResize.getPanelStyle(0), { overflow: 'visible' }]"
        :class="[
          'relative flex flex-col bg-bg-secondary border-r border-border/50',
          horizontalResize.isResizing.value
            ? 'select-none'
            : 'transition-[width] duration-200 ease-in-out',
        ]"
      >
        <!-- 折叠/展开悬浮按钮 -->
        <button
          @click="horizontalResize.toggleCollapse(0)"
          :class="[
            'absolute top-1/2 -translate-y-1/2 z-20 w-7 h-7 rounded-full glass-strong border border-border/50 flex items-center justify-center text-text-muted hover:text-accent hover:border-accent transition-all shadow-md opacity-70 hover:opacity-100',
            horizontalResize.isCollapsed(0) ? '-right-7' : 'right-1',
          ]"
          :title="horizontalResize.isCollapsed(0) ? '展开面板' : '折叠面板'"
        >
          <svg
            class="w-3.5 h-3.5 transition-transform duration-200"
            :class="horizontalResize.isCollapsed(0) ? 'rotate-180' : ''"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
          </svg>
        </button>

        <template v-if="!horizontalResize.isCollapsed(0)">
          <!-- 标签页切换 -->
          <div class="flex border-b border-border/30">
            <button
              @click="leftActiveTab = 'credentials'"
              :class="[
                'flex-1 px-3 py-2 text-sm font-medium transition-colors border-b-2',
                leftActiveTab === 'credentials'
                  ? 'text-accent border-accent'
                  : 'text-text-muted hover:text-text-primary border-transparent',
              ]"
            >
              凭据
            </button>
            <button
              @click="leftActiveTab = 'templates'"
              :class="[
                'flex-1 px-3 py-2 text-sm font-medium transition-colors border-b-2',
                leftActiveTab === 'templates'
                  ? 'text-accent border-accent'
                  : 'text-text-muted hover:text-text-primary border-transparent',
              ]"
            >
              模板
            </button>
          </div>

          <!-- 凭据列表 -->
          <div
            v-if="leftActiveTab === 'credentials'"
            class="flex-1 overflow-y-auto p-3"
          >
            <div class="flex items-center justify-between mb-3">
              <span class="text-xs text-text-muted"
                >{{ credentials.length }} 个凭据</span
              >
              <button
                @click="openCredentialModal(null)"
                class="p-1 text-text-muted hover:text-accent transition-colors"
                title="添加凭据"
              >
                <svg
                  class="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
              </button>
            </div>

            <div class="space-y-2">
              <div
                v-for="cred in credentials"
                :key="cred.id"
                class="p-2.5 bg-bg-tertiary rounded-md border border-border/30 hover:border-border/60 transition-colors"
              >
                <div class="flex items-center justify-between">
                  <span
                    class="text-sm text-text-primary font-medium truncate"
                    >{{ cred.name }}</span
                  >
                  <div class="flex items-center gap-1">
                    <button
                      @click="openCredentialModal(cred)"
                      class="p-1 text-text-muted hover:text-accent transition-colors"
                      title="编辑"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                        />
                      </svg>
                    </button>
                    <button
                      @click="deleteCredential(cred)"
                      class="p-1 text-text-muted hover:text-red-400 transition-colors"
                      title="删除"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
                <div class="flex items-center gap-2 mt-1.5">
                  <span
                    class="text-xs px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-400"
                    >{{ cred.version }}</span
                  >
                  <span class="text-xs text-text-muted font-mono">
                    {{ getCommunityDisplay(cred) }}
                  </span>
                  <button
                    @click="toggleCommunityVisibility(cred.id)"
                    class="p-0.5 text-text-muted hover:text-text-primary transition-colors"
                  >
                    <svg
                      v-if="showCommunityMap.get(cred.id)"
                      class="w-3 h-3"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                      />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                      />
                    </svg>
                    <svg
                      v-else
                      class="w-3 h-3"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                      />
                    </svg>
                  </button>
                </div>
                <div
                  v-if="cred.hasAuthKey || cred.hasPrivKey"
                  class="text-xs text-text-muted mt-1 truncate"
                >
                  {{ cred.hasAuthKey ? "Auth: ✓" : "" }}
                  {{ cred.hasPrivKey ? "Priv: ✓" : "" }}
                </div>
              </div>

              <div
                v-if="credentials.length === 0"
                class="text-center text-text-muted text-sm py-4"
              >
                暂无凭据
              </div>
            </div>
          </div>

          <!-- 模板列表 -->
          <div
            v-if="leftActiveTab === 'templates'"
            class="flex-1 overflow-y-auto p-3"
          >
            <div class="flex items-center justify-between mb-3">
              <span class="text-xs text-text-muted"
                >{{ templates.length }} 个模板</span
              >
              <button
                @click="openTemplateModal(null)"
                class="p-1 text-text-muted hover:text-accent transition-colors"
                title="添加模板"
              >
                <svg
                  class="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
              </button>
            </div>

            <div class="space-y-2">
              <div
                v-for="tmpl in templates"
                :key="tmpl.id"
                class="p-2.5 bg-bg-tertiary rounded-md border border-border/30 hover:border-border/60 transition-colors"
              >
                <div class="flex items-center justify-between">
                  <span
                    class="text-sm text-text-primary font-medium truncate"
                    >{{ tmpl.name }}</span
                  >
                  <div class="flex items-center gap-1">
                    <button
                      @click="openTemplateModal(tmpl)"
                      class="p-1 text-text-muted hover:text-accent transition-colors"
                      title="编辑"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                        />
                      </svg>
                    </button>
                    <button
                      @click="deleteTemplate(tmpl)"
                      class="p-1 text-text-muted hover:text-red-400 transition-colors"
                      title="删除"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
                <div class="flex items-center gap-2 mt-1.5">
                  <span
                    class="text-xs px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400"
                    >{{ tmpl.oidItems.length }} OID</span
                  >
                </div>
                <div
                  v-if="tmpl.description"
                  class="text-xs text-text-muted mt-1 truncate"
                >
                  {{ tmpl.description }}
                </div>
              </div>

              <div
                v-if="templates.length === 0"
                class="text-center text-text-muted text-sm py-4"
              >
                暂无模板
              </div>
            </div>
          </div>
        </template>
      </aside>

      <!-- 左侧拖拽手柄 -->
      <div
        v-if="!horizontalResize.isCollapsed(0)"
        class="w-1 flex-shrink-0 cursor-col-resize group relative z-10"
        @mousedown="(e) => horizontalResize.startResize(0, e)"
      >
        <div
          class="absolute inset-y-0 left-0 w-px bg-border/30 group-hover:bg-accent transition-colors"
        ></div>
        <div class="absolute inset-y-0 -left-1 w-3"></div>
      </div>

      <!-- 中间面板：目标列表 + 轮询结果 -->
      <main
        ref="centerContainerRef"
        :class="[
          'flex-1 flex flex-col min-w-0 overflow-hidden',
          horizontalResize.isResizing.value ? 'select-none' : '',
        ]"
      >
        <!-- 目标列表区域 -->
        <div
          :style="{
            height: `${verticalResize.sizes.value[0]}px`,
            flexShrink: 0,
          }"
          class="flex flex-col min-h-0 overflow-hidden"
        >
          <!-- 目标工具栏 -->
          <div
            class="flex items-center justify-between px-4 py-2 bg-bg-secondary/50 border-b border-border/30 flex-shrink-0"
          >
            <div class="flex items-center gap-3">
              <h3 class="text-sm font-medium text-text-primary">轮询目标</h3>
              <select
                v-model="targetFilterEnabled"
                class="px-2 py-1 text-xs bg-bg-tertiary border border-border/50 rounded text-text-primary focus:outline-none focus:border-accent"
              >
                <option
                  v-for="opt in targetEnabledOptions"
                  :key="String(opt.value)"
                  :value="opt.value"
                >
                  {{ opt.label }}
                </option>
              </select>
            </div>
            <button
              @click="openTargetModal(null)"
              class="px-3 py-1 text-xs bg-accent hover:bg-accent-dark text-white rounded-md transition-colors"
            >
              添加目标
            </button>
          </div>

          <!-- 目标表格 -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-sm">
              <thead
                class="sticky top-0 bg-bg-secondary border-b border-border/50"
              >
                <tr>
                  <th
                    class="px-3 py-2 text-left text-text-muted font-medium w-8"
                  ></th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    名称
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    IP 地址
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    状态
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    最近轮询
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    延迟
                  </th>
                  <th class="px-3 py-2 text-center text-text-muted font-medium">
                    操作
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="target in filteredTargets"
                  :key="target.id"
                  @click="selectTarget(target)"
                  :class="[
                    'cursor-pointer border-b border-border/20 transition-colors',
                    selectedTarget?.id === target.id
                      ? 'bg-accent-bg/30'
                      : 'hover:bg-bg-hover',
                    isNewResult(target.id)
                      ? 'animate-pulse bg-green-500/5'
                      : '',
                  ]"
                >
                  <!-- 状态图标 -->
                  <td class="px-3 py-2">
                    <span
                      :class="[
                        'inline-block w-2 h-2 rounded-full',
                        !target.enabled
                          ? 'bg-gray-500'
                          : getLatestResult(target.id)?.status === 'success'
                            ? 'bg-green-500'
                            : getLatestResult(target.id)?.status === 'failure'
                              ? 'bg-red-500'
                              : 'bg-yellow-500',
                      ]"
                    ></span>
                  </td>
                  <!-- 名称 -->
                  <td class="px-3 py-2">
                    <span class="text-text-primary">{{
                      target.displayName
                    }}</span>
                  </td>
                  <!-- IP 地址 -->
                  <td class="px-3 py-2">
                    <span class="text-text-secondary font-mono text-xs"
                      >{{ target.targetIP }}:{{ target.targetPort }}</span
                    >
                  </td>
                  <!-- 状态 -->
                  <td class="px-3 py-2">
                    <span
                      v-if="!target.enabled"
                      class="text-xs px-1.5 py-0.5 rounded bg-gray-500/20 text-gray-400"
                      >已禁用</span
                    >
                    <span
                      v-else-if="
                        getLatestResult(target.id)?.status === 'success'
                      "
                      class="text-xs px-1.5 py-0.5 rounded bg-green-500/20 text-green-400"
                      >正常</span
                    >
                    <span
                      v-else-if="
                        getLatestResult(target.id)?.status === 'failure'
                      "
                      class="text-xs px-1.5 py-0.5 rounded bg-red-500/20 text-red-400"
                      >异常</span
                    >
                    <span
                      v-else
                      class="text-xs px-1.5 py-0.5 rounded bg-yellow-500/20 text-yellow-400"
                      >待轮询</span
                    >
                  </td>
                  <!-- 最近轮询时间 -->
                  <td class="px-3 py-2">
                    <span class="text-text-muted text-xs">{{
                      formatTime(target.lastPollAt)
                    }}</span>
                  </td>
                  <!-- 延迟 -->
                  <td class="px-3 py-2">
                    <span
                      v-if="getLatestResult(target.id)"
                      class="text-text-secondary text-xs font-mono"
                    >
                      {{ target.lastPollStatus || "-" }}
                    </span>
                    <span v-else class="text-text-muted text-xs">-</span>
                  </td>
                  <!-- 操作 -->
                  <td class="px-3 py-2">
                    <div class="flex items-center justify-center gap-1">
                      <button
                        @click.stop="handlePollNow(target.id)"
                        :disabled="pollingTargetIds.has(target.id)"
                        class="p-1 text-text-muted hover:text-accent transition-colors disabled:opacity-50"
                        title="立即轮询"
                      >
                        <svg
                          class="w-3.5 h-3.5"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
                          />
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                          />
                        </svg>
                      </button>
                      <button
                        @click.stop="handleToggleTarget(target)"
                        :class="[
                          'w-8 h-4 rounded-full transition-colors relative flex-shrink-0',
                          target.enabled ? 'bg-green-500' : 'bg-gray-600',
                        ]"
                        title="启用/禁用"
                      >
                        <span
                          :class="[
                            'absolute top-0.5 w-3 h-3 rounded-full bg-white transition-transform',
                            target.enabled ? 'left-4' : 'left-0.5',
                          ]"
                        ></span>
                      </button>
                      <button
                        @click.stop="openTargetModal(target)"
                        class="p-1 text-text-muted hover:text-accent transition-colors"
                        title="编辑"
                      >
                        <svg
                          class="w-3.5 h-3.5"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                          />
                        </svg>
                      </button>
                      <button
                        @click.stop="handleDeleteTarget(target)"
                        class="p-1 text-text-muted hover:text-red-400 transition-colors"
                        title="删除"
                      >
                        <svg
                          class="w-3.5 h-3.5"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                          />
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
                <tr v-if="filteredTargets.length === 0">
                  <td
                    colspan="7"
                    class="px-3 py-8 text-center text-text-muted text-sm"
                  >
                    {{ targetsLoading ? "加载中..." : "暂无轮询目标" }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- 垂直拖拽手柄（目标/结果间） -->
        <div
          class="h-1 flex-shrink-0 cursor-row-resize group relative z-10"
          @mousedown="(e) => verticalResize.startResize(0, e)"
        >
          <div
            class="absolute inset-x-0 top-0 h-px bg-border/30 group-hover:bg-accent transition-colors"
          ></div>
          <div class="absolute inset-x-0 -top-1 h-3"></div>
        </div>

        <!-- 轮询结果区域 -->
        <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
          <!-- 结果工具栏 -->
          <div
            class="flex items-center justify-between px-4 py-2 bg-bg-secondary/50 border-b border-border/30 flex-shrink-0"
          >
            <div class="flex items-center gap-3">
              <h3 class="text-sm font-medium text-text-primary">轮询结果</h3>
              <span
                v-if="resultFilter.targetId"
                class="text-xs text-text-muted"
              >
                目标 #{{ resultFilter.targetId }}
                <button
                  @click="resetResultFilter"
                  class="ml-1 text-accent hover:underline"
                >
                  清除
                </button>
              </span>
            </div>
            <button
              @click="clearOldResults"
              class="px-2 py-1 text-xs bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded transition-colors"
            >
              清理结果
            </button>
          </div>

          <!-- 结果表格 -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-sm">
              <thead
                class="sticky top-0 bg-bg-secondary border-b border-border/50"
              >
                <tr>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    时间
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    目标 IP
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    OID
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    值
                  </th>
                  <th class="px-3 py-2 text-left text-text-muted font-medium">
                    类型
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="result in pollingResults"
                  :key="result.id"
                  @click="selectResult(result)"
                  :class="[
                    'cursor-pointer border-b border-border/20 transition-colors',
                    selectedResult?.id === result.id
                      ? 'bg-accent-bg/30'
                      : 'hover:bg-bg-hover',
                  ]"
                >
                  <td class="px-3 py-2">
                    <span class="text-text-secondary text-xs">{{
                      formatTime(result.pollTime)
                    }}</span>
                  </td>
                  <td class="px-3 py-2">
                    <span class="text-text-primary text-xs font-mono">{{
                      result.targetIP
                    }}</span>
                  </td>
                  <td class="px-3 py-2">
                    <span class="text-text-primary text-xs font-mono">{{
                      result.oid
                    }}</span>
                    <span
                      v-if="result.oidName"
                      class="text-text-muted text-xs ml-1"
                      >({{ result.oidName }})</span
                    >
                  </td>
                  <td class="px-3 py-2">
                    <span
                      class="text-text-primary text-xs font-mono truncate max-w-[200px] block"
                      >{{ result.value || "-" }}</span
                    >
                  </td>
                  <td class="px-3 py-2">
                    <span class="text-text-muted text-xs">{{
                      result.valueType || "-"
                    }}</span>
                  </td>
                </tr>
                <tr v-if="pollingResults.length === 0">
                  <td
                    colspan="5"
                    class="px-3 py-6 text-center text-text-muted text-sm"
                  >
                    {{ resultsLoading ? "加载中..." : "暂无轮询结果" }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 分页 -->
          <div
            class="flex items-center justify-between px-4 py-2 bg-bg-secondary/50 border-t border-border/30 flex-shrink-0"
          >
            <span class="text-xs text-text-muted">共 {{ totalResults }} 条</span>
            <div class="flex items-center gap-1">
              <span v-if="totalPages > 1" class="text-xs text-text-muted mr-1">
                第 {{ currentPage }} / {{ totalPages }} 页
              </span>
              <button
                @click="changePage(currentPage - 1)"
                :disabled="currentPage <= 1"
                class="px-2 py-1 text-xs bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded disabled:opacity-50 transition-colors"
              >
                上一页
              </button>
              <button
                @click="changePage(currentPage + 1)"
                :disabled="currentPage >= totalPages"
                class="px-2 py-1 text-xs bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded disabled:opacity-50 transition-colors"
              >
                下一页
              </button>
            </div>
          </div>
        </div>
      </main>

      <!-- 右侧拖拽手柄 -->
      <div
        v-if="!horizontalResize.isCollapsed(2)"
        class="w-1 flex-shrink-0 cursor-col-resize group relative z-10"
        @mousedown="(e) => horizontalResize.startResize(1, e)"
      >
        <div
          class="absolute inset-y-0 left-0 w-px bg-border/30 group-hover:bg-accent transition-colors"
        ></div>
        <div class="absolute inset-y-0 -left-1 w-3"></div>
      </div>

      <!-- 右侧面板：详情 -->
      <aside
        :style="horizontalResize.getPanelStyle(2)"
        :class="[
          'flex flex-col bg-bg-secondary border-l border-border/50 overflow-y-auto',
          horizontalResize.isResizing.value
            ? 'select-none'
            : 'transition-[width] duration-200 ease-in-out',
        ]"
      >
        <template v-if="!horizontalResize.isCollapsed(2)">
          <!-- 当前分发器状态 -->
          <div
            v-if="schedulerStatus?.dispatcherStatus"
            class="p-4 border-b border-border/30"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-medium text-text-primary">当前状态</h3>
            </div>
            <div
              class="p-2.5 bg-bg-tertiary rounded-md border border-border/20"
            >
              <div class="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                <span class="text-text-muted">活跃设备</span>
                <span class="text-right text-text-primary font-medium">
                  {{ schedulerStatus.dispatcherStatus.activeDevices }}/{{
                    schedulerStatus.dispatcherStatus.maxDevices
                  }}
                </span>
                <span class="text-text-muted">排队任务</span>
                <span class="text-right text-text-primary font-medium">
                  {{ schedulerStatus.dispatcherStatus.waitingTasks }}
                </span>
                <span class="text-text-muted col-span-2">累计跳过</span>
                <span
                  class="text-right text-text-primary font-medium col-span-2 -mt-5"
                >
                  {{ schedulerStatus.dispatcherStatus.skippedTasks }}
                </span>
              </div>
            </div>
          </div>

          <!-- 目标详情 -->
          <div v-if="selectedTarget" class="p-4 border-b border-border/30">
            <h3 class="text-sm font-medium text-text-primary mb-3">目标详情</h3>

            <div class="space-y-2 text-sm">
              <div class="flex justify-between">
                <span class="text-text-muted">名称</span>
                <span class="text-text-primary">{{
                  selectedTarget.displayName
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">地址</span>
                <span class="text-text-primary font-mono text-xs"
                  >{{ selectedTarget.targetIP }}:{{
                    selectedTarget.targetPort
                  }}</span
                >
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">凭据</span>
                <span class="text-text-primary text-xs">{{
                  selectedTarget.credentialId
                    ? (getCredentialById(selectedTarget.credentialId)?.name ??
                      "-")
                    : "-"
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">模板</span>
                <span class="text-text-primary text-xs">{{
                  selectedTarget.templateId
                    ? (getTemplateById(selectedTarget.templateId)?.name ?? "-")
                    : "-"
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">间隔</span>
                <span class="text-text-primary"
                  >{{ selectedTarget.pollInterval }}s</span
                >
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">状态</span>
                <span
                  :class="
                    selectedTarget.enabled ? 'text-green-400' : 'text-gray-400'
                  "
                >
                  {{ selectedTarget.enabled ? "已启用" : "已禁用" }}
                </span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">最后轮询</span>
                <span class="text-text-primary text-xs">{{
                  formatTime(selectedTarget.lastPollAt)
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">轮询状态</span>
                <span class="text-text-primary text-xs">{{
                  selectedTarget.lastPollStatus || "未知"
                }}</span>
              </div>
            </div>
          </div>

          <!-- 统计信息 -->
          <div v-if="selectedTargetStats" class="p-4 border-b border-border/30">
            <h3 class="text-sm font-medium text-text-primary mb-3">统计信息</h3>

            <div class="space-y-2 text-sm">
              <div class="flex justify-between">
                <span class="text-text-muted">总轮询</span>
                <span class="text-text-primary">{{
                  selectedTargetStats.totalPolls
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">成功</span>
                <span class="text-green-400">{{
                  selectedTargetStats.successCount
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">失败</span>
                <span class="text-red-400">{{
                  selectedTargetStats.failCount
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">成功率</span>
                <span class="text-text-primary">
                  {{
                    selectedTargetStats.totalPolls > 0
                      ? (
                          (selectedTargetStats.successCount /
                            selectedTargetStats.totalPolls) *
                          100
                        ).toFixed(1)
                      : 0
                  }}%
                </span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">平均延迟</span>
                <span class="text-text-primary">{{
                  formatLatency(selectedTargetStats.avgLatencyMs)
                }}</span>
              </div>
            </div>

            <!-- 成功率进度条 -->
            <div class="mt-3">
              <div
                class="w-full h-2 bg-bg-tertiary rounded-full overflow-hidden"
              >
                <div
                  class="h-full rounded-full transition-all duration-500"
                  :class="
                    selectedTargetStats.totalPolls > 0 &&
                    selectedTargetStats.successCount /
                      selectedTargetStats.totalPolls >=
                      0.8
                      ? 'bg-green-500'
                      : 'bg-yellow-500'
                  "
                  :style="{
                    width: `${selectedTargetStats.totalPolls > 0 ? (selectedTargetStats.successCount / selectedTargetStats.totalPolls) * 100 : 0}%`,
                  }"
                ></div>
              </div>
            </div>
          </div>

          <!-- 最新结果详情 -->
          <div v-if="selectedResult" class="p-4">
            <h3 class="text-sm font-medium text-text-primary mb-3">结果详情</h3>

            <div class="space-y-2 text-sm mb-3">
              <div class="flex justify-between">
                <span class="text-text-muted">时间</span>
                <span class="text-text-primary text-xs">{{
                  formatTime(selectedResult.pollTime)
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">目标 IP</span>
                <span class="text-text-primary text-xs font-mono">{{
                  selectedResult.targetIP
                }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">批次 ID</span>
                <span
                  class="text-text-secondary text-xs font-mono truncate max-w-[150px]"
                  >{{ selectedResult.batchId }}</span
                >
              </div>
            </div>

            <!-- OID 详情 -->
            <div class="p-2.5 bg-bg-tertiary rounded border border-border/20">
              <div class="flex items-center justify-between mb-1">
                <span class="text-xs text-accent font-mono truncate">{{
                  selectedResult.oid
                }}</span>
                <span
                  class="text-xs px-1 py-0.5 rounded bg-blue-500/20 text-blue-400 ml-2 flex-shrink-0"
                  >{{ selectedResult.valueType || "未知" }}</span
                >
              </div>
              <div
                v-if="selectedResult.oidName"
                class="text-xs text-text-muted mb-1 truncate"
              >
                {{ selectedResult.oidName }}
              </div>
              <div
                class="text-xs text-text-primary break-all bg-bg-primary p-1.5 rounded mt-1"
              >
                {{ selectedResult.value || "(无值)" }}
              </div>
            </div>
          </div>

          <!-- 无选中状态 -->
          <div
            v-if="!selectedTarget && !selectedResult"
            class="flex-1 flex items-center justify-center"
          >
            <div class="text-center text-text-muted">
              <svg
                class="w-12 h-12 mx-auto mb-3 opacity-30"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                />
              </svg>
              <p class="text-sm">选择目标查看详情</p>
            </div>
          </div>
        </template>

        <!-- 折叠时的展开按钮 -->
        <div v-else class="flex-1 flex items-center justify-center">
          <button
            @click="horizontalResize.expandPanel(2)"
            class="w-8 h-8 rounded-full glass-strong border border-border/50 flex items-center justify-center text-text-muted hover:text-accent hover:border-accent transition-all shadow-md"
            title="展开详情面板"
          >
            <svg
              class="w-4 h-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15 19l-7-7 7-7"
              />
            </svg>
          </button>
        </div>
      </aside>
    </div>

    <!-- ==================== 凭据编辑模态框 ==================== -->
    <Teleport to="body">
      <div
        v-if="showCredentialModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
        @click.self="showCredentialModal = false"
      >
        <div
          class="bg-bg-secondary rounded-lg shadow-xl border border-border/50 w-[520px] max-h-[85vh] overflow-y-auto"
        >
          <div class="px-5 py-4 border-b border-border/30">
            <h3 class="text-base font-medium text-text-primary">
              {{ editingCredential ? "编辑凭据" : "添加凭据" }}
            </h3>
          </div>

          <div class="p-5 space-y-4">
            <!-- 名称 -->
            <div>
              <label class="block text-sm text-text-muted mb-1">名称 *</label>
              <input
                v-model="credentialForm.name"
                type="text"
                placeholder="输入凭据名称"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
              />
            </div>

            <!-- SNMP 版本 -->
            <div>
              <label class="block text-sm text-text-muted mb-1"
                >SNMP 版本 *</label
              >
              <select
                v-model="credentialForm.version"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
              >
                <option value="v1">v1</option>
                <option value="v2c">v2c</option>
                <option value="v3">v3</option>
              </select>
            </div>

            <!-- v1/v2c: Community String -->
            <div v-if="credentialForm.version !== 'v3'">
              <label class="block text-sm text-text-muted mb-1"
                >Community String *</label
              >
              <input
                v-model="credentialForm.community"
                type="text"
                placeholder="输入 Community String"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
              />
            </div>

            <!-- v3: 安全级别 -->
            <div v-if="credentialForm.version === 'v3'">
              <label class="block text-sm text-text-muted mb-1"
                >安全级别 *</label
              >
              <select
                v-model="credentialForm.securityLevel"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
              >
                <option value="noAuthNoPriv">
                  无认证无加密 (noAuthNoPriv)
                </option>
                <option value="authNoPriv">认证无加密 (authNoPriv)</option>
                <option value="authPriv">认证且加密 (authPriv)</option>
              </select>
            </div>

            <!-- v3: 用户名 -->
            <div v-if="credentialForm.version === 'v3'">
              <label class="block text-sm text-text-muted mb-1">用户名 *</label>
              <input
                v-model="credentialForm.username"
                type="text"
                placeholder="输入 SNMPv3 用户名"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
              />
            </div>

            <!-- v3: 认证协议和密钥 -->
            <div
              v-if="
                credentialForm.version === 'v3' &&
                credentialForm.securityLevel !== 'noAuthNoPriv'
              "
            >
              <label class="block text-sm text-text-muted mb-1"
                >认证协议 *</label
              >
              <select
                v-model="credentialForm.authProtocol"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
              >
                <option value="MD5">MD5</option>
                <option value="SHA">SHA</option>
                <option value="SHA224">SHA-224</option>
                <option value="SHA256">SHA-256</option>
                <option value="SHA384">SHA-384</option>
                <option value="SHA512">SHA-512</option>
              </select>
            </div>

            <div
              v-if="
                credentialForm.version === 'v3' &&
                credentialForm.securityLevel !== 'noAuthNoPriv'
              "
            >
              <label class="block text-sm text-text-muted mb-1"
                >认证密钥
                {{ editingCredential ? "(留空保持原值)" : "*" }}</label
              >
              <input
                v-model="credentialForm.authPassword"
                type="password"
                placeholder="输入认证密钥"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
              />
            </div>

            <!-- v3: 加密协议和密钥 (仅 authPriv) -->
            <div
              v-if="
                credentialForm.version === 'v3' &&
                credentialForm.securityLevel === 'authPriv'
              "
            >
              <label class="block text-sm text-text-muted mb-1"
                >加密协议 *</label
              >
              <select
                v-model="credentialForm.privProtocol"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
              >
                <option value="DES">DES</option>
                <option value="AES">AES</option>
                <option value="AES192">AES-192</option>
                <option value="AES256">AES-256</option>
                <option value="AES192C">AES-192C</option>
                <option value="AES256C">AES-256C</option>
              </select>
            </div>

            <div
              v-if="
                credentialForm.version === 'v3' &&
                credentialForm.securityLevel === 'authPriv'
              "
            >
              <label class="block text-sm text-text-muted mb-1"
                >加密密钥
                {{ editingCredential ? "(留空保持原值)" : "*" }}</label
              >
              <input
                v-model="credentialForm.privPassword"
                type="password"
                placeholder="输入加密密钥"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
              />
            </div>

            <!-- v3: 上下文配置 (可选) -->
            <div v-if="credentialForm.version === 'v3'">
              <label class="block text-sm text-text-muted mb-1"
                >上下文名称 (可选)</label
              >
              <input
                v-model="credentialForm.contextName"
                type="text"
                placeholder="输入 SNMPv3 上下文名称"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
              />
            </div>

            <div v-if="credentialForm.version === 'v3'">
              <label class="block text-sm text-text-muted mb-1"
                >上下文引擎 ID (可选)</label
              >
              <input
                v-model="credentialForm.contextEngineId"
                type="text"
                placeholder="输入 SNMPv3 上下文引擎 ID"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
              />
            </div>
          </div>

          <div
            class="px-5 py-3 border-t border-border/30 flex justify-end gap-2"
          >
            <button
              @click="showCredentialModal = false"
              class="px-4 py-2 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
            >
              取消
            </button>
            <button
              @click="saveCredential"
              :disabled="
                !credentialForm.name ||
                (credentialForm.version !== 'v3' &&
                  !credentialForm.community) ||
                (credentialForm.version === 'v3' && !credentialForm.username)
              "
              class="px-4 py-2 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors disabled:opacity-50"
            >
              保存
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ==================== 模板编辑模态框 ==================== -->
    <Teleport to="body">
      <div
        v-if="showTemplateModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
        @click.self="showTemplateModal = false"
      >
        <div
          class="bg-bg-secondary rounded-lg shadow-xl border border-border/50 w-[560px] max-h-[85vh] overflow-y-auto"
        >
          <div class="px-5 py-4 border-b border-border/30">
            <h3 class="text-base font-medium text-text-primary">
              {{ editingTemplate ? "编辑模板" : "添加模板" }}
            </h3>
          </div>

          <div class="p-5 space-y-4">
            <div>
              <label class="block text-sm text-text-muted mb-1">名称 *</label>
              <input
                v-model="templateForm.name"
                type="text"
                placeholder="输入模板名称"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
              />
            </div>

            <div>
              <label class="block text-sm text-text-muted mb-1">描述</label>
              <textarea
                v-model="templateForm.description"
                placeholder="输入描述（可选）"
                rows="2"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent resize-none"
              ></textarea>
            </div>

            <!-- OID 列表 -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <label class="text-sm text-text-muted">OID 列表</label>
                <button
                  @click="addOIDToTemplate"
                  class="px-2 py-1 text-xs bg-accent hover:bg-accent-dark text-white rounded transition-colors"
                >
                  添加 OID
                </button>
              </div>

              <div class="space-y-2 max-h-[240px] overflow-y-auto">
                <div
                  v-for="(oid, idx) in templateForm.oidItems"
                  :key="idx"
                  class="p-2.5 bg-bg-tertiary rounded border border-border/20"
                >
                  <div class="flex items-center gap-2">
                    <input
                      v-model="oid.oid"
                      type="text"
                      placeholder="OID（如 1.3.6.1.2.1.1.1.0）"
                      class="flex-1 px-2 py-1 text-xs bg-bg-primary border border-border/50 rounded text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
                    />
                    <select
                      v-model="oid.operation"
                      class="px-2 py-1 text-xs bg-bg-primary border border-border/50 rounded text-text-primary focus:outline-none focus:border-accent"
                    >
                      <option value="bulk">BULK</option>
                      <option value="walk">WALK</option>
                      <option value="get">GET</option>
                    </select>
                    <button
                      @click="removeOIDFromTemplate(idx)"
                      class="p-1 text-text-muted hover:text-red-400 transition-colors"
                    >
                      <svg
                        class="w-3.5 h-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M6 18L18 6M6 6l12 12"
                        />
                      </svg>
                    </button>
                  </div>
                  <div class="flex items-center gap-2 mt-1.5">
                    <input
                      v-model="oid.name"
                      type="text"
                      placeholder="名称（可选）"
                      class="flex-1 px-2 py-1 text-xs bg-bg-primary border border-border/50 rounded text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
                    />
                    <input
                      v-model="oid.description"
                      type="text"
                      placeholder="描述（可选）"
                      class="flex-1 px-2 py-1 text-xs bg-bg-primary border border-border/50 rounded text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
                    />
                  </div>
                </div>

                <div
                  v-if="templateForm.oidItems.length === 0"
                  class="text-center text-text-muted text-xs py-3"
                >
                  点击「添加 OID」按钮添加 OID 配置
                </div>
              </div>
            </div>
          </div>

          <div
            class="px-5 py-3 border-t border-border/30 flex justify-end gap-2"
          >
            <button
              @click="showTemplateModal = false"
              class="px-4 py-2 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
            >
              取消
            </button>
            <button
              @click="saveTemplate"
              :disabled="
                !templateForm.name || templateForm.oidItems.length === 0
              "
              class="px-4 py-2 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors disabled:opacity-50"
            >
              保存
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ==================== 目标编辑模态框 ==================== -->
    <Teleport to="body">
      <div
        v-if="showTargetModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
        @click.self="showTargetModal = false"
      >
        <div
          class="bg-bg-secondary rounded-lg shadow-xl border border-border/50 w-[480px] max-h-[80vh] overflow-y-auto"
        >
          <div class="px-5 py-4 border-b border-border/30">
            <h3 class="text-base font-medium text-text-primary">
              {{ editingTarget ? "编辑目标" : "添加目标" }}
            </h3>
          </div>

          <div class="p-5 space-y-4">
            <div>
              <label class="block text-sm text-text-muted mb-1">名称 *</label>
              <input
                v-model="targetForm.displayName"
                type="text"
                placeholder="输入目标名称"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
              />
            </div>

            <div class="grid grid-cols-3 gap-3">
              <div class="col-span-2">
                <label class="block text-sm text-text-muted mb-1"
                  >IP 地址 *</label
                >
                <input
                  v-model="targetForm.targetIP"
                  type="text"
                  placeholder="如 192.168.1.1"
                  class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
                />
              </div>
              <div>
                <label class="block text-sm text-text-muted mb-1">端口</label>
                <input
                  v-model.number="targetForm.targetPort"
                  type="number"
                  min="1"
                  max="65535"
                  class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
                />
              </div>
            </div>

            <div>
              <label class="block text-sm text-text-muted mb-1">凭据 *</label>
              <select
                v-model="targetForm.credentialId"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
              >
                <option
                  v-for="cred in credentials"
                  :key="cred.id"
                  :value="cred.id"
                >
                  {{ cred.name }} ({{ cred.version }})
                </option>
              </select>
              <p
                v-if="credentials.length === 0"
                class="text-xs text-yellow-400 mt-1"
              >
                请先创建凭据
              </p>
            </div>

            <div>
              <label class="block text-sm text-text-muted mb-1">模板 *</label>
              <select
                v-model="targetForm.templateId"
                class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
              >
                <option
                  v-for="tmpl in templates"
                  :key="tmpl.id"
                  :value="tmpl.id"
                >
                  {{ tmpl.name }} ({{ tmpl.oidItems.length }} OID)
                </option>
              </select>
              <p
                v-if="templates.length === 0"
                class="text-xs text-yellow-400 mt-1"
              >
                请先创建模板
              </p>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm text-text-muted mb-1"
                  >轮询间隔（秒）</label
                >
                <input
                  v-model.number="targetForm.pollInterval"
                  type="number"
                  min="5"
                  class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
                />
              </div>
              <div>
                <label class="block text-sm text-text-muted mb-1">启用</label>
                <div class="flex items-center h-[38px]">
                  <button
                    @click="targetForm.enabled = !targetForm.enabled"
                    :class="[
                      'w-10 h-5 rounded-full transition-colors relative',
                      targetForm.enabled ? 'bg-green-500' : 'bg-gray-600',
                    ]"
                  >
                    <span
                      :class="[
                        'absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform',
                        targetForm.enabled ? 'left-5' : 'left-0.5',
                      ]"
                    ></span>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div
            class="px-5 py-3 border-t border-border/30 flex justify-end gap-2"
          >
            <button
              @click="showTargetModal = false"
              class="px-4 py-2 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
            >
              取消
            </button>
            <button
              @click="saveTarget"
              :disabled="
                !targetForm.displayName ||
                !targetForm.targetIP ||
                !targetForm.credentialId ||
                !targetForm.templateId
              "
              class="px-4 py-2 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors disabled:opacity-50"
            >
              保存
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ==================== 并发控制设置弹窗 ==================== -->
    <Teleport to="body">
      <div
        v-if="showConcurrencySettingsModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
        @click.self="showConcurrencySettingsModal = false"
      >
        <div
          class="bg-bg-secondary rounded-lg shadow-xl border border-border/50 w-[480px] max-h-[85vh] overflow-y-auto"
        >
          <!-- 标题头 -->
          <div
            class="px-5 py-4 border-b border-border/30 flex items-center justify-between"
          >
            <h3 class="text-base font-medium text-text-primary">
              并发控制设置
            </h3>
            <button
              @click="showConcurrencySettingsModal = false"
              class="p-1 rounded-md hover:bg-bg-hover text-text-muted hover:text-text-primary transition-colors"
            >
              <svg
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
          <!-- 表单体 -->
          <div class="p-5 space-y-4">
            <!-- 最大并发设备数 -->
            <div>
              <label class="block text-sm text-text-muted mb-1"
                >最大并发设备数 (1-100)</label
              >
              <div class="flex items-center gap-2">
                <input
                  v-model.number="concurrencyConfig.maxDevices"
                  type="number"
                  min="1"
                  max="100"
                  :disabled="concurrencySaving"
                  class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent disabled:opacity-50"
                />
                <span class="text-sm text-text-muted whitespace-nowrap"
                  >台</span
                >
              </div>
            </div>

            <!-- 单设备并发操作数 -->
            <div>
              <label class="block text-sm text-text-muted mb-1"
                >单设备并发操作数 (1-5)</label
              >
              <div class="flex items-center gap-2">
                <input
                  v-model.number="concurrencyConfig.maxOpsPerDevice"
                  type="number"
                  min="1"
                  max="5"
                  :disabled="concurrencySaving"
                  class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent disabled:opacity-50"
                />
                <span class="text-sm text-text-muted whitespace-nowrap"
                  >个</span
                >
              </div>
            </div>

            <!-- 设备繁忙时跳过 -->
            <div class="flex items-center justify-between">
              <label class="text-sm text-text-muted">设备繁忙时跳过</label>
              <button
                @click="
                  concurrencyConfig.skipIfBusy = !concurrencyConfig.skipIfBusy
                "
                :disabled="concurrencySaving"
                :class="[
                  'w-10 h-5 rounded-full transition-colors relative',
                  concurrencyConfig.skipIfBusy ? 'bg-green-500' : 'bg-gray-600',
                ]"
              >
                <span
                  :class="[
                    'absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform',
                    concurrencyConfig.skipIfBusy ? 'left-5' : 'left-0.5',
                  ]"
                ></span>
              </button>
            </div>

            <!-- 排队超时时间 -->
            <div>
              <label class="block text-sm text-text-muted mb-1"
                >排队超时时间 (10-120)</label
              >
              <div class="flex items-center gap-2">
                <input
                  v-model.number="concurrencyConfig.queueTimeoutSecs"
                  type="number"
                  min="10"
                  max="120"
                  :disabled="concurrencySaving"
                  class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent disabled:opacity-50"
                />
                <span class="text-sm text-text-muted whitespace-nowrap"
                  >秒</span
                >
              </div>
            </div>
          </div>
          <!-- 底部按钮 -->
          <div
            class="px-5 py-3 border-t border-border/30 flex justify-end gap-2"
          >
            <button
              @click="resetConcurrencyConfig"
              :disabled="concurrencySaving"
              class="px-4 py-2 text-sm rounded-md bg-bg-tertiary hover:bg-bg-hover border border-border/50 text-text-secondary transition-colors disabled:opacity-50"
            >
              恢复默认
            </button>
            <button
              @click="saveConcurrencyConfig"
              :disabled="concurrencySaving"
              class="px-4 py-2 text-sm rounded-md bg-accent hover:bg-accent/90 text-white transition-colors disabled:opacity-50"
            >
              {{ concurrencySaving ? "保存中..." : "保存" }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
