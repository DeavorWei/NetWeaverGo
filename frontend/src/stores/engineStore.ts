import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { Events } from "@wailsio/runtime";

export type EngineState =
  | "Idle"
  | "Starting"
  | "Running"
  | "Paused"
  | "Closing"
  | "Closed";

export interface DeviceViewState {
  ip: string;
  status: string;
  logs: string[];
  logCount: number;
  truncated: boolean;
  cmdIndex: number;
  totalCmd: number;
}

export interface ExecutionSnapshot {
  taskName: string;
  totalDevices: number;
  finishedCount: number;
  progress: number;
  isRunning: boolean;
  devices: DeviceViewState[];
}

export interface SuspendRequiredEvent {
  sessionId?: string;
  ip: string;
  error: string;
  command: string;
}

export const useEngineStore = defineStore("engine", () => {
  // ========== 状态定义 ==========
  const currentState = ref<EngineState>("Idle");
  const executionSnapshot = ref<ExecutionSnapshot | null>(null);
  const isConnecting = ref(false); // 纯前端 UI 状态
  const suspendModal = ref<{
    show: boolean;
    sessionId: string;
    ip: string;
    content: string;
  }>({ show: false, sessionId: "", ip: "", content: "" });

  // 事件日志（限制长度防止内存溢出）
  const eventLogs = ref<any[]>([]);
  const maxLogs = 1000;

  // ========== 计算属性 ==========
  const isRunning = computed(() => currentState.value === "Running");
  const progressPercent = computed(
    () => executionSnapshot.value?.progress ?? 0
  );
  const execDevices = computed(() => executionSnapshot.value?.devices ?? []);

  // ========== 事件监听初始化 ==========
  let cleanupFns: (() => void)[] = [];

  function initListeners() {
    // 清理之前的监听器
    cleanupListeners();

    // 监听执行快照（200ms 定时推送）
    const unlistenSnapshot = Events.On("execution:snapshot", (ev: any) => {
      const data = ev.data?.[0] as ExecutionSnapshot;
      if (data) {
        executionSnapshot.value = data;
        currentState.value = data.isRunning ? "Running" : "Idle";
        isConnecting.value = false;
      }
    });
    if (typeof unlistenSnapshot === 'function') {
      cleanupFns.push(unlistenSnapshot);
    }

    // 监听引擎完成
    const unlistenFinished = Events.On("engine:finished", () => {
      currentState.value = "Idle";
      isConnecting.value = false;
    });
    if (typeof unlistenFinished === 'function') {
      cleanupFns.push(unlistenFinished);
    }

    // 监听挂起请求
    const unlistenSuspend = Events.On("engine:suspend_required", (ev: any) => {
      const data = ev.data?.[0] as SuspendRequiredEvent;
      if (data) {
        suspendModal.value = {
          show: true,
          sessionId: data.sessionId || "",
          ip: data.ip,
          content: `设备: ${data.ip}\n命令: ${data.command}\n\n错误详情:\n${data.error}`,
        };
      }
    });
    if (typeof unlistenSuspend === 'function') {
      cleanupFns.push(unlistenSuspend);
    }

    // 监听挂起超时
    const unlistenTimeout = Events.On("engine:suspend_timeout", (ev: any) => {
      const data = ev.data?.[0] as { ip: string; sessionId: string };
      if (data && suspendModal.value.ip === data.ip) {
        suspendModal.value.show = false;
      }
    });
    if (typeof unlistenTimeout === 'function') {
      cleanupFns.push(unlistenTimeout);
    }

    // 监听设备事件（用于日志）
    const unlistenDevice = Events.On("device:event", (ev: any) => {
      const event = ev.data?.[0];
      if (event) {
        eventLogs.value.push(event);
        if (eventLogs.value.length > maxLogs) {
          eventLogs.value.shift(); // O(1) 淘汰最旧日志
        }
      }
    });
    if (typeof unlistenDevice === 'function') {
      cleanupFns.push(unlistenDevice);
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

  // ========== 状态同步 ==========
  async function syncStateFromGo() {
    try {
      // 通过 Wails 绑定调用 Go 方法
      const status = await (window as any).go?.ui?.EngineService?.GetEngineState?.();
      if (status) {
        currentState.value = (status.state as EngineState) || "Idle";
      }
    } catch (err) {
      console.error("Failed to sync engine state:", err);
    }
  }

  // ========== 操作 ==========
  async function stopEngine() {
    try {
      await (window as any).go?.ui?.EngineService?.StopEngine?.();
    } catch (err) {
      console.error("停止引擎失败:", err);
    }
  }

  function resolveSuspend(action: "C" | "S" | "A") {
    const identifier = suspendModal.value.sessionId || suspendModal.value.ip;
    (window as any).go?.ui?.EngineService?.ResolveSuspend?.(identifier, action);
    suspendModal.value.show = false;
  }

  function closeSuspendModal() {
    suspendModal.value.show = false;
  }

  function reset() {
    currentState.value = "Idle";
    executionSnapshot.value = null;
    isConnecting.value = false;
    eventLogs.value = [];
    suspendModal.value.show = false;
  }

  return {
    // 状态
    currentState,
    executionSnapshot,
    isConnecting,
    suspendModal,
    eventLogs,

    // 计算属性
    isRunning,
    progressPercent,
    execDevices,

    // 方法
    initListeners,
    cleanupListeners,
    syncStateFromGo,
    stopEngine,
    resolveSuspend,
    closeSuspendModal,
    reset,
  };
});
