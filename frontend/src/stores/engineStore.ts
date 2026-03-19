import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { Events } from "@wailsio/runtime";
import { EngineAPI, DeviceAPI } from "../services/api";
import { ExecutionSnapshot } from "../bindings/github.com/NetWeaverGo/core/internal/report/models.js";
import type { DeviceAsset } from "../services/api";

type EngineState =
  | "Idle"
  | "Starting"
  | "Running"
  | "Paused"
  | "Closing"
  | "Closed";

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

const ACTIVE_ENGINE_STATES = new Set<EngineState>([
  "Starting",
  "Running",
  "Paused",
  "Closing",
]);

export const useEngineStore = defineStore("engine", () => {
  const executionSnapshot = ref<ExecutionSnapshot | null>(null);
  const suspendSessions = ref<Record<string, SuspendSessionState>>({});

  const isRunning = computed(() => Boolean(executionSnapshot.value?.isRunning));

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
    executionSnapshot.value = snapshot
      ? ExecutionSnapshot.createFrom(snapshot)
      : null;
  }

  function markExecutionFinished() {
    if (!executionSnapshot.value) {
      return;
    }

    applySnapshot(
      ExecutionSnapshot.createFrom({
        ...executionSnapshot.value,
        isRunning: false,
      })
    );
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

    const unlistenSnapshot = Events.On("execution:snapshot", (ev: any) => {
      const data = unwrapEventData<ExecutionSnapshot>(ev);
      if (data) {
        applySnapshot(ExecutionSnapshot.createFrom(data));
      }
    });
    if (typeof unlistenSnapshot === "function") {
      cleanupFns.push(unlistenSnapshot);
    }

    const unlistenFinished = Events.On("engine:finished", () => {
      markExecutionFinished();
      // 同时重置备份状态
      backupState.value.isRunning = false;
    });
    if (typeof unlistenFinished === "function") {
      cleanupFns.push(unlistenFinished);
    }

    const unlistenSuspend = Events.On("engine:suspend_required", (ev: any) => {
      const data = unwrapEventData<SuspendRequiredEvent>(ev);
      if (data) {
        suspendSessions.value[data.ip] = {
          sessionId: data.sessionId || "",
          ip: data.ip,
          command: data.command,
          error: data.error,
          content: `设备: ${data.ip}\n命令: ${data.command}\n\n错误详情:\n${data.error}`,
        };
      }
    });
    if (typeof unlistenSuspend === "function") {
      cleanupFns.push(unlistenSuspend);
    }

    const unlistenTimeout = Events.On("engine:suspend_timeout", (ev: any) => {
      const data = unwrapEventData<{ ip: string; sessionId: string }>(ev);
      if (data) {
        delete suspendSessions.value[data.ip];
      }
    });
    if (typeof unlistenTimeout === "function") {
      cleanupFns.push(unlistenTimeout);
    }
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

  async function syncExecutionState() {
    try {
      const [snapshot, status] = await Promise.all([
        EngineAPI.getExecutionSnapshot().catch(() => null),
        EngineAPI.getEngineState().catch(() => null),
      ]);

      if (snapshot) {
        applySnapshot(snapshot);
        return snapshot.isRunning;
      }

      const state = (status?.state as EngineState | undefined) ?? "Idle";
      if (ACTIVE_ENGINE_STATES.has(state)) {
        const running = state === "Starting" || state === "Running" || state === "Paused";
        applySnapshot(
          new ExecutionSnapshot({
            taskName: "任务执行",
            totalDevices: 0,
            finishedCount: 0,
            progress: 0,
            isRunning: running,
            startTime: "",
            devices: [],
          })
        );
        return true;
      }

      applySnapshot(null);
      return false;
    } catch (err) {
      console.error("Failed to sync execution state:", err);
      return false;
    }
  }

  async function stopEngine() {
    try {
      await EngineAPI.stopEngine();
    } catch (err) {
      console.error("停止引擎失败:", err);
      throw err;
    }
  }

  function resolveSuspend(ip: string, action: "C" | "A") {
    const session = suspendSessions.value[ip];
    if (!session) {
      return;
    }
    const identifier = session.sessionId || session.ip;
    void EngineAPI.resolveSuspend(identifier, action);
    delete suspendSessions.value[ip];
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
      // 调用后端 StartBackup API
      await EngineAPI.startBackup();
    } catch (err) {
      console.error("启动备份失败:", err);
      backupState.value.isRunning = false;
      throw err;
    }
  }

  function updateBackupSnapshot(snapshot: ExecutionSnapshot) {
    backupState.value.progress = snapshot.progress;

    // 检查是否完成
    if (!snapshot.isRunning) {
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
