import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { Events } from "@wailsio/runtime";
import { DeviceAPI } from "../services/api";
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
 * Engine Store - 执行引擎状态管理
 * 
 * @deprecated 此 store 基于旧执行引擎，现已迁移到 taskexecStore。
 * 保留此文件仅为兼容过渡期，请使用 useTaskexecStore() 替代。
 */
export const useEngineStore = defineStore("engine", () => {
  // ================== 执行状态（已废弃，使用 taskexecStore）==================
  const executionSnapshot = ref<ExecutionSnapshot | null>(null);
  const suspendSessions = ref<Record<string, SuspendSessionState>>({});

  // 使用 status 字段判断运行状态
  const isRunning = computed(() => executionSnapshot.value?.status === 'running');

  // ================== 备份相关状态 ==================
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
  let cleanupFns: (() => void)[] = [];

  function applySnapshot(snapshot: ExecutionSnapshot | null) {
    // 简单赋值，不再使用 ExecutionSnapshot.createFrom
    executionSnapshot.value = snapshot;
  }

  function markExecutionFinished() {
    if (!executionSnapshot.value) {
      return;
    }
    // 直接修改状态
    executionSnapshot.value = {
      ...executionSnapshot.value,
      status: 'completed'
    };
  }

  function unwrapEventData<T = any>(ev: any): T | null {
    if (!ev) {
      return null;
    }
    if (Array.isArray(ev.data)) {
      return (ev.data[0] ?? null) as T | null;
    }
    return (ev.data ?? null) as T | null;
  }

  function initListeners() {
    cleanupListeners();

    // 监听统一运行时事件（新事件格式）
    const unlistenSnapshot = Events.On("task:snapshot", (ev: any) => {
      const data = unwrapEventData<ExecutionSnapshot>(ev);
      if (data) {
        applySnapshot(data);
      }
    });
    if (typeof unlistenSnapshot === "function") {
      cleanupFns.push(unlistenSnapshot);
    }

    const unlistenFinished = Events.On("task:finished", () => {
      markExecutionFinished();
      backupState.value.isRunning = false;
    });
    if (typeof unlistenFinished === "function") {
      cleanupFns.push(unlistenFinished);
    }

    // 旧事件监听（兼容期）
    const unlistenOldFinished = Events.On("engine:finished", () => {
      markExecutionFinished();
      backupState.value.isRunning = false;
    });
    if (typeof unlistenOldFinished === "function") {
      cleanupFns.push(unlistenOldFinished);
    }

    // 暂停功能已随旧引擎删除
  }

  function cleanupListeners() {
    cleanupFns.forEach((fn) => {
      try {
        fn();
      } catch (e) {
        console.warn("清理事件监听器时发生警告:", e);
      }
    });
    cleanupFns = [];
  }

  async function syncExecutionState(): Promise<boolean> {
    // 统一运行时通过事件驱动，不再主动轮询
    console.warn('syncExecutionState 已废弃，统一运行时通过事件驱动状态更新');
    return isRunning.value;
  }

  async function stopEngine() {
    console.warn('stopEngine 已废弃，请使用 taskexecStore.cancelTask(runId)');
    // 不再调用旧 API
  }

  function resolveSuspend(_ip: string, _action: "C" | "A") {
    console.warn('暂停功能已随旧执行引擎删除');
    // 暂停功能已删除
  }

  function reset() {
    applySnapshot(null);
    suspendSessions.value = {};
  }

  // ================== 备份相关方法 ==================
  async function loadBackupDevices() {
    try {
      const devices = await DeviceAPI.listDevices();
      backupState.value.devices = devices || [];
      return devices || [];
    } catch (err) {
      console.error("加载备份设备列表失败:", err);
      backupState.value.devices = [];
      return [];
    }
  }

  async function startBackup(selectedDevices: DeviceAsset[]) {
    if (backupState.value.isRunning) {
      return;
    }

    backupState.value = {
      isRunning: true,
      progress: 0,
      devices: selectedDevices,
      startTime: new Date().toISOString(),
    };

    try {
      // 调用后端 StartBackup API（统一运行时暂不支持，保留接口）
      console.warn('备份功能在统一运行时中暂不支持');
      throw new Error('备份功能暂不可用');
    } catch (err) {
      console.error("启动备份失败:", err);
      backupState.value.isRunning = false;
      throw err;
    }
  }

  function updateBackupSnapshot(snapshot: ExecutionSnapshot) {
    backupState.value.progress = snapshot.progress;

    // 检查是否完成（统一运行时使用 status 字段）
    if (snapshot.status !== 'running') {
      backupState.value.isRunning = false;
    }
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
    executionSnapshot,
    suspendSessions,
    isRunning,
    initListeners,
    cleanupListeners,
    syncExecutionState,
    stopEngine,
    resolveSuspend,
    reset,
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
