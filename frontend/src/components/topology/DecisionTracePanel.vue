<template>
  <div class="decision-trace-panel">
    <!-- 标题栏 -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <svg
          class="w-4 h-4 text-text-muted"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
        >
          <path
            d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
        <span class="text-sm font-medium text-text-primary">决策轨迹</span>
      </div>
      <button
        v-if="closable"
        @click="$emit('close')"
        class="text-text-muted hover:text-text-primary"
      >
        <svg
          class="w-4 h-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
        >
          <path
            d="M6 18L18 6M6 6l12 12"
            stroke-width="2"
            stroke-linecap="round"
          />
        </svg>
      </button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="flex items-center justify-center py-8">
      <svg
        class="animate-spin w-6 h-6 text-accent"
        viewBox="0 0 24 24"
        fill="none"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h0z"
        />
      </svg>
    </div>

    <!-- 错误提示 -->
    <div
      v-else-if="error"
      class="p-4 bg-error/10 border border-error/30 rounded-lg text-sm text-error"
    >
      {{ error }}
    </div>

    <!-- 无决策轨迹 -->
    <div
      v-else-if="!trace"
      class="p-4 bg-bg-panel rounded-lg text-sm text-text-muted text-center"
    >
      暂无决策轨迹
    </div>

    <!-- 决策轨迹内容 -->
    <div v-else class="space-y-4">
      <!-- 基本信息 -->
      <div class="grid grid-cols-2 gap-3 text-xs">
        <div class="bg-bg-panel rounded-lg p-3">
          <span class="text-text-muted">决策类型</span>
          <div class="mt-1 text-text-primary font-medium">
            {{ formatDecisionType(trace.decisionType) }}
          </div>
        </div>
        <div class="bg-bg-panel rounded-lg p-3">
          <span class="text-text-muted">决策结果</span>
          <div class="mt-1">
            <span
              :class="[
                'px-2 py-0.5 rounded text-xs font-medium',
                trace.decisionResult === 'retained'
                  ? 'bg-success/20 text-success'
                  : trace.decisionResult === 'rejected'
                    ? 'bg-error/20 text-error'
                    : 'bg-warning/20 text-warning',
              ]"
            >
              {{ formatDecisionResult(trace.decisionResult) }}
            </span>
          </div>
        </div>
      </div>

      <!-- 决策原因 -->
      <div class="bg-bg-panel rounded-lg p-3">
        <div class="text-xs text-text-muted mb-1">决策原因</div>
        <div class="text-sm text-text-primary">{{ trace.decisionReason }}</div>
      </div>

      <!-- 决策依据 -->
      <div v-if="trace.decisionBasis" class="bg-bg-panel rounded-lg p-3">
        <div class="text-xs text-text-muted mb-1">决策依据</div>
        <pre
          class="text-xs text-text-primary font-mono whitespace-pre-wrap overflow-auto max-h-32"
          >{{ formatDecisionBasis(trace.decisionBasis) }}</pre
        >
      </div>

      <!-- 保留的候选 -->
      <div v-if="trace.retainedCandidateIds?.length" class="space-y-2">
        <div class="text-xs text-text-muted">
          保留的候选 ({{ trace.retainedCandidateIds.length }})
        </div>
        <div class="flex flex-wrap gap-1">
          <span
            v-for="id in trace.retainedCandidateIds"
            :key="id"
            class="px-2 py-0.5 bg-success/10 text-success rounded text-xs font-mono"
          >
            {{ id.slice(0, 8) }}
          </span>
        </div>
      </div>

      <!-- 淘汰的候选 -->
      <div v-if="trace.rejectedCandidateIds?.length" class="space-y-2">
        <div class="text-xs text-text-muted">
          淘汰的候选 ({{ trace.rejectedCandidateIds.length }})
        </div>
        <div class="flex flex-wrap gap-1">
          <span
            v-for="id in trace.rejectedCandidateIds"
            :key="id"
            class="px-2 py-0.5 bg-error/10 text-error rounded text-xs font-mono"
          >
            {{ id.slice(0, 8) }}
          </span>
        </div>
      </div>

      <!-- 候选列表快照 -->
      <div v-if="trace.candidates" class="space-y-2">
        <button
          @click="showCandidatesSnapshot = !showCandidatesSnapshot"
          class="flex items-center gap-2 text-xs text-text-muted hover:text-text-primary"
        >
          <svg
            :class="[
              'w-3 h-3 transition-transform',
              showCandidatesSnapshot ? 'rotate-90' : '',
            ]"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
          >
            <path d="M9 5l7 7-7 7" stroke-width="2" stroke-linecap="round" />
          </svg>
          <span>候选列表快照</span>
        </button>
        <div v-if="showCandidatesSnapshot" class="bg-bg-panel rounded-lg p-3">
          <pre
            class="text-xs text-text-primary font-mono whitespace-pre-wrap overflow-auto max-h-48"
            >{{ formatCandidates(trace.candidates) }}</pre
          >
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from "vue";
import { TaskExecutionAPI } from "@/services/api";
import type { TopologyDecisionTraceView } from "@/bindings/github.com/NetWeaverGo/core/internal/models/models";

interface Props {
  runId: string;
  edgeId?: string;
  traceId?: string;
  closable?: boolean;
}

const props = defineProps<Props>();
defineEmits(["close"]);

const loading = ref(false);
const error = ref<string | null>(null);
const trace = ref<TopologyDecisionTraceView | null>(null);
const showCandidatesSnapshot = ref(false);

// 格式化决策类型
function formatDecisionType(type: string): string {
  const typeMap: Record<string, string> = {
    conflict_resolution: "冲突消解",
    candidate_selection: "候选选择",
    edge_merge: "边合并",
  };
  return typeMap[type] || type;
}

// 格式化决策结果
function formatDecisionResult(result: string): string {
  const resultMap: Record<string, string> = {
    retained: "保留",
    rejected: "淘汰",
    merged: "合并",
    conflict: "冲突",
  };
  return resultMap[result] || result;
}

// 格式化决策依据
function formatDecisionBasis(basis: string): string {
  try {
    const parsed = JSON.parse(basis);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return basis;
  }
}

// 格式化候选列表
function formatCandidates(candidates: string): string {
  try {
    const parsed = JSON.parse(candidates);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return candidates;
  }
}

// 加载决策轨迹
async function loadTrace() {
  if (!props.runId) return;

  loading.value = true;
  error.value = null;

  try {
    if (props.traceId) {
      // 如果有 traceId，从所有轨迹中查找
      const traces = await TaskExecutionAPI.getTopologyDecisionTracesByRun(
        props.runId,
      );
      trace.value = traces.find((t) => t.traceId === props.traceId) || null;
    } else if (props.edgeId) {
      // 如果有 edgeId，获取边解释视图
      const explain = await TaskExecutionAPI.getTopologyEdgeExplain(
        props.runId,
        props.edgeId,
      );
      trace.value = explain?.decisionTrace || null;
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : "加载决策轨迹失败";
    trace.value = null;
  } finally {
    loading.value = false;
  }
}

// 监听属性变化
watch(
  () => [props.runId, props.edgeId, props.traceId],
  () => {
    loadTrace();
  },
  { immediate: true },
);

onMounted(() => {
  loadTrace();
});
</script>
