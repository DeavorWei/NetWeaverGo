import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { Events } from "@wailsio/runtime";
import { EngineAPI } from "../services/api";
import { ExecutionSnapshot } from "../bindings/github.com/NetWeaverGo/core/internal/report/models";

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
  error: string;
  command: string;
}

const ACTIVE_ENGINE_STATES = new Set<EngineState>([
  "Starting",
  "Running",
  "Paused",
  "Closing",
]);

export const useEngineStore = defineStore("engine", () => {
  const executionSnapshot = ref<ExecutionSnapshot | null>(null);
  const suspendModal = ref<{
    show: boolean;
    sessionId: string;
    ip: string;
    content: string;
  }>({ show: false, sessionId: "", ip: "", content: "" });

  const isRunning = computed(() => Boolean(executionSnapshot.value?.isRunning));
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
        progress: 100,
      })
    );
  }

  function initListeners() {
    cleanupListeners();

    const unlistenSnapshot = Events.On("execution:snapshot", (ev: any) => {
      const data = ev.data?.[0];
      if (data) {
        applySnapshot(ExecutionSnapshot.createFrom(data));
      }
    });
    if (typeof unlistenSnapshot === "function") {
      cleanupFns.push(unlistenSnapshot);
    }

    const unlistenFinished = Events.On("engine:finished", () => {
      markExecutionFinished();
    });
    if (typeof unlistenFinished === "function") {
      cleanupFns.push(unlistenFinished);
    }

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
    if (typeof unlistenSuspend === "function") {
      cleanupFns.push(unlistenSuspend);
    }

    const unlistenTimeout = Events.On("engine:suspend_timeout", (ev: any) => {
      const data = ev.data?.[0] as { ip: string; sessionId: string };
      if (data && suspendModal.value.ip === data.ip) {
        suspendModal.value.show = false;
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
        applySnapshot(
          new ExecutionSnapshot({
            taskName: "任务执行",
            totalDevices: 0,
            finishedCount: 0,
            progress: state === "Closing" ? 100 : 0,
            isRunning: state !== "Closing",
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

  function resolveSuspend(action: "C" | "S" | "A") {
    const identifier = suspendModal.value.sessionId || suspendModal.value.ip;
    void EngineAPI.resolveSuspend(identifier, action);
    suspendModal.value.show = false;
  }

  function reset() {
    applySnapshot(null);
    suspendModal.value.show = false;
    suspendModal.value.sessionId = "";
    suspendModal.value.ip = "";
    suspendModal.value.content = "";
  }

  return {
    executionSnapshot,
    suspendModal,
    isRunning,
    initListeners,
    cleanupListeners,
    syncExecutionState,
    stopEngine,
    resolveSuspend,
    reset,
  };
});
