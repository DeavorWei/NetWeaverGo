import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { useTaskexecStore } from "./taskexecStore";
import type { DeviceAsset, ExecutionSnapshot } from "../services/api";

export interface SuspendRequiredEvent {
  sessionId?: string;
  ip: string;
  command: string;
  error: string;
}

export interface SuspendSessionState {
  sessionId: string;
  ip: string;
  command: string;
  error: string;
  content: string;
}

/**
 * Engine Store - 执行引擎状态管理（兼容层）
 * 
 * @deprecated 此 store 现已作为 taskexecStore 的代理层保留。
 * 所有功能已迁移到 taskexecStore，请直接使用 useTaskexecStore()。
 * 保留此文件仅为兼容 TaskExecution.vue 中的旧引用。
 */
export const useEngineStore = defineStore("engine", () => {
  // 委托给 taskexecStore
  const taskexecStore = useTaskexecStore();

  // ================== 执行状态代理 ==================
  const executionSnapshot = computed(() => taskexecStore.currentSnapshot);
  const suspendSessions = ref<Record<string, SuspendSessionState>>({});

  // 使用 status 字段判断运行状态
  const isRunning = computed(() => taskexecStore.isRunning);

  // ================== 备份相关状态（本地保留）==================
  const backupState = ref<{
    isRunning: boolean;
    progress: number;
    devices: DeviceAsset[];
    startTime: string | null;
  }>({
    isRunning: false,
    progress: 0,
    devices: [],
    startTime: null,
  });

  const isBackupRunning = computed(() => backupState.value.isRunning);
  const backupProgress = computed(() => backupState.value.progress);
  const backupDevices = computed(() => backupState.value.devices);

  function applySnapshot(snapshot: ExecutionSnapshot | null) {
    // 通过 taskexecStore 更新
    if (snapshot && taskexecStore.currentRunId) {
      taskexecStore.updateSnapshot(taskexecStore.currentRunId, snapshot);
    }
  }

  function markExecutionFinished() {
    // 由 taskexecStore 事件处理
  }

  function initListeners() {
    // 事件监听由 taskexecStore 统一管理
    // 此函数保留仅为兼容旧代码调用
  }

  function cleanupListeners() {
    // 事件清理由 taskexecStore 统一管理
  }

  async function syncExecutionState(): Promise<boolean> {
    // 刷新当前快照
    const runId = taskexecStore.currentRunId;
    if (runId) {
      await taskexecStore.refreshSnapshot(runId);
    }
    return isRunning.value;
  }

  async function stopEngine() {
    const runId = taskexecStore.currentRunId;
    if (runId) {
      await taskexecStore.cancelTask(runId);
    }
  }

  function resolveSuspend(_ip: string, _action: "C" | "A") {
    // 暂停功能已随旧执行引擎删除
    console.warn("暂停功能已删除");
  }

  function reset() {
    taskexecStore.setCurrentRunId("");
  }

  // ================== 备份相关方法（本地保留）==================
  async function loadBackupDevices(): Promise<DeviceAsset[]> {
    // 备份功能暂不支持，返回空数组
    return [];
  }

  async function startBackup(_selectedDevices: DeviceAsset[]) {
    // 备份功能暂不支持
    console.warn("备份功能暂不可用");
    throw new Error("备份功能暂不可用");
  }

  function updateBackupSnapshot(_snapshot: ExecutionSnapshot) {
    // 备份功能暂不支持
  }

  function resetBackup() {
    backupState.value = {
      isRunning: false,
      progress: 0,
      devices: [],
      startTime: null,
    };
  }

  return {
    // 执行状态
    executionSnapshot,
    suspendSessions,
    isRunning,
    
    // 方法
    initListeners,
    cleanupListeners,
    syncExecutionState,
    stopEngine,
    resolveSuspend,
    reset,
    applySnapshot,
    markExecutionFinished,
    
    // 备份相关
    backupState,
    isBackupRunning,
    backupProgress,
    backupDevices,
    loadBackupDevices,
    startBackup,
    updateBackupSnapshot,
    resetBackup,
  };
});
