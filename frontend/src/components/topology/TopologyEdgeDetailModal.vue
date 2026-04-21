<template>
  <Transition name="modal">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="absolute inset-0 bg-black/60 backdrop-blur-sm"
        @click="closeModal"
      ></div>
      <div
        class="relative bg-bg-card border border-border rounded-xl shadow-2xl max-w-lg w-full mx-4 max-h-[80vh] overflow-hidden animate-slide-in flex flex-col"
      >
        <!-- 头部 -->
        <div
          class="px-4 py-3 border-b border-border bg-bg-panel flex items-center justify-between"
        >
          <div>
            <h3 class="text-sm font-semibold text-text-primary">链路详情</h3>
            <p class="text-xs text-text-muted mt-0.5">链路证据与连接信息</p>
          </div>
          <button
            @click="closeModal"
            class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor">
              <path d="M6 18L18 6M6 6l12 12" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
        </div>

        <!-- 内容 -->
        <div class="p-4 flex-1 overflow-auto scrollbar-custom">
          <!-- 无数据状态 -->
          <div v-if="!edgeDetail" class="text-center py-8 text-sm text-text-muted">
            无链路信息
          </div>

          <!-- 链路信息 -->
          <div v-else class="space-y-4">
            <!-- 链路连接信息 -->
            <div class="bg-bg-panel border border-border rounded-lg p-4">
              <div class="flex items-center justify-between gap-2">
                <!-- A端设备 -->
                <div class="flex-1 text-center">
                  <button
                    @click="handleDeviceClick(edgeDetail.aDevice.id)"
                    class="text-sm font-semibold text-accent hover:text-accent-glow transition-colors"
                  >
                    {{ edgeDetail.aDevice.label || edgeDetail.aDevice.id }}
                  </button>
                  <div class="text-xs text-text-muted font-mono mt-1">
                    {{ edgeDetail.aIf || '-' }}
                  </div>
                </div>

                <!-- 连接图标 -->
                <div class="flex items-center gap-1 text-text-muted">
                  <div class="w-8 h-0.5 bg-border"></div>
                  <svg class="w-4 h-4" :class="statusColorClass" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                    <path d="M8 7h8M8 12h8M8 17h8" stroke-width="2" stroke-linecap="round"/>
                  </svg>
                  <div class="w-8 h-0.5 bg-border"></div>
                </div>

                <!-- B端设备 -->
                <div class="flex-1 text-center">
                  <button
                    @click="handleDeviceClick(edgeDetail.bDevice.id)"
                    class="text-sm font-semibold text-accent hover:text-accent-glow transition-colors"
                  >
                    {{ edgeDetail.bDevice.label || edgeDetail.bDevice.id }}
                  </button>
                  <div class="text-xs text-text-muted font-mono mt-1">
                    {{ edgeDetail.bIf || '-' }}
                  </div>
                </div>
              </div>

              <!-- 链路属性 -->
              <div class="flex items-center justify-center gap-3 mt-4 pt-3 border-t border-border">
                <div class="flex items-center gap-1">
                  <span class="text-xs text-text-muted">类型:</span>
                  <span class="text-xs px-2 py-0.5 rounded bg-bg-hover text-text-primary">
                    {{ edgeDetail.edgeType }}
                  </span>
                </div>
                <div class="flex items-center gap-1">
                  <span class="text-xs text-text-muted">状态:</span>
                  <span class="text-xs px-2 py-0.5 rounded" :class="statusBgClass">
                    {{ edgeDetail.status }}
                  </span>
                </div>
                <div class="flex items-center gap-1">
                  <span class="text-xs text-text-muted">置信度:</span>
                  <span class="text-xs text-text-primary font-mono">
                    {{ edgeDetail.confidence.toFixed(2) }}
                  </span>
                </div>
              </div>
            </div>

            <!-- 证据列表 -->
            <div>
              <div class="text-xs font-medium text-text-secondary uppercase tracking-wide mb-2">
                证据列表 ({{ edgeDetail.evidence?.length || 0 }})
              </div>
              <div class="space-y-2 max-h-[300px] overflow-auto scrollbar-custom">
                <div
                  v-for="(ev, idx) in edgeDetail.evidence"
                  :key="idx"
                  class="bg-bg-panel border border-border rounded-lg p-3"
                >
                  <!-- 证据类型和摘要 -->
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span
                        class="px-2 py-0.5 rounded text-xs font-medium"
                        :class="getEvidenceTypeClass(ev.type)"
                      >
                        {{ ev.type }}
                      </span>
                      <span class="text-sm text-text-primary">
                        {{ ev.summary || '-' }}
                      </span>
                    </div>
                  </div>

                  <!-- 证据来源 -->
                  <div class="mt-2 text-xs text-text-muted font-mono space-y-0.5">
                    <div>device: {{ ev.deviceId }}</div>
                    <div>command: {{ ev.command }}</div>
                    <div v-if="ev.rawRefId">raw: {{ ev.rawRefId }}</div>
                  </div>

                  <!-- 远端设备信息 -->
                  <div
                    v-if="ev.remoteName || ev.remoteIf || ev.remoteMac || ev.remoteIp"
                    class="mt-2 pt-2 border-t border-border text-xs text-text-muted font-mono"
                  >
                    <div class="flex flex-wrap gap-x-3 gap-y-1">
                      <span v-if="ev.remoteName">远端设备: {{ ev.remoteName }}</span>
                      <span v-if="ev.remoteIf">接口: {{ ev.remoteIf }}</span>
                      <span v-if="ev.remoteMac">MAC: {{ ev.remoteMac }}</span>
                      <span v-if="ev.remoteIp">IP: {{ ev.remoteIp }}</span>
                    </div>
                  </div>
                </div>

                <!-- 无证据提示 -->
                <div
                  v-if="!edgeDetail.evidence || edgeDetail.evidence.length === 0"
                  class="text-center py-4 text-xs text-text-muted"
                >
                  暂无证据数据
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 底部 -->
        <div class="px-4 py-3 border-t border-border bg-bg-panel flex justify-end">
          <button
            @click="closeModal"
            class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-secondary border border-border text-text-secondary hover:text-text-primary hover:border-accent/50 transition-all"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { TopologyEdgeDetailView } from "../../services/api";

const props = defineProps<{
  show: boolean;
  edgeDetail: TopologyEdgeDetailView | null;
}>();

const emit = defineEmits<{
  "update:show": [value: boolean];
  "device-click": [deviceId: string];
}>();

// 状态颜色类
const statusColorClass = computed(() => {
  if (!props.edgeDetail) return "text-text-muted";
  const status = props.edgeDetail.status;
  const map: Record<string, string> = {
    confirmed: "text-success",
    semi_confirmed: "text-warning",
    inferred: "text-warning",
    conflict: "text-error",
  };
  return map[status] || "text-text-muted";
});

const statusBgClass = computed(() => {
  if (!props.edgeDetail) return "bg-bg-hover text-text-muted";
  const status = props.edgeDetail.status;
  const map: Record<string, string> = {
    confirmed: "bg-success/20 text-success",
    semi_confirmed: "bg-warning/20 text-warning",
    inferred: "bg-warning/20 text-warning",
    conflict: "bg-error/20 text-error",
  };
  return map[status] || "bg-bg-hover text-text-muted";
});

// 证据类型样式
function getEvidenceTypeClass(type: string): string {
  const map: Record<string, string> = {
    LLDP: "bg-success/20 text-success",
    FDB: "bg-accent/20 text-accent",
    ARP: "bg-warning/20 text-warning",
    CDP: "bg-success/20 text-success",
    CONFIG: "bg-bg-hover text-text-primary",
  };
  return map[type.toUpperCase()] || "bg-bg-hover text-text-muted";
}

function closeModal() {
  emit("update:show", false);
}

function handleDeviceClick(deviceId: string) {
  emit("device-click", deviceId);
}
</script>

<style scoped>
/* modal 过渡动画已移至全局 _animations.css */
/* 滚动条样式已移至全局 _scrollbar.css，请使用 scrollbar-custom 类 */
</style>