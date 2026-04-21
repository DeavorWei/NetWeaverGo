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
        class="relative bg-bg-card border border-border rounded-xl shadow-2xl max-w-md w-full mx-4 max-h-[80vh] overflow-hidden animate-slide-in flex flex-col"
      >
        <!-- 头部 -->
        <div
          class="px-4 py-3 border-b border-border bg-bg-panel flex items-center justify-between"
        >
          <div>
            <h3 class="text-sm font-semibold text-text-primary">设备详情</h3>
            <p class="text-xs text-text-muted mt-0.5">
              {{ isInferred ? '推断节点' : '采集设备' }}
            </p>
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
          <!-- 加载状态 -->
          <div v-if="loading" class="flex items-center justify-center py-8">
            <svg class="w-6 h-6 animate-spin text-accent" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-opacity="0.25"/>
              <path d="M12 2a10 10 0 0 1 10 10" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            <span class="ml-2 text-sm text-text-muted">加载中...</span>
          </div>

          <!-- 设备信息 -->
          <div v-else-if="deviceDetail || nodeInfo" class="space-y-4">
            <!-- 设备标识 -->
            <div class="flex items-center gap-3">
              <div
                class="w-12 h-12 rounded-lg flex items-center justify-center"
                :class="isInferred ? 'bg-warning/20 text-warning' : 'bg-accent/20 text-accent'"
              >
                <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                  <rect x="2" y="3" width="20" height="14" rx="2" stroke-width="2"/>
                  <path d="M8 21h8M12 17v4" stroke-width="2" stroke-linecap="round"/>
                </svg>
              </div>
              <div>
                <div class="text-base font-semibold text-text-primary">
                  {{ deviceDetail?.identity?.hostname || nodeInfo?.label || deviceId }}
                </div>
                <div class="text-xs text-text-muted">
                  IP: {{ deviceDetail?.deviceIp || deviceId }}
                </div>
              </div>
            </div>

            <!-- 推断节点提示 -->
            <div v-if="isInferred" class="bg-warning/10 border border-warning/30 rounded-lg p-3 space-y-2">
              <div class="flex items-center gap-2 text-sm text-warning font-medium">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                  <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L4.798 16c-.77 1.333.192 3 1.732 3z" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                推断节点
              </div>
              <div class="text-xs text-text-muted">
                此设备通过 FDB/ARP 数据推断，未直接采集设备信息
              </div>
              <div v-if="nodeInfo && nodeInfo.macAddress" class="text-xs text-text-muted">
                MAC: {{ nodeInfo.macAddress }}
              </div>
              <div v-if="nodeInfo && (nodeInfo.macAddresses?.length ?? 0) > 1" class="text-xs text-text-muted">
                多MAC: {{ nodeInfo.macAddresses?.join(', ') }}
              </div>
              <div v-if="nodeInfo && nodeInfo.vendor" class="text-xs text-text-muted">
                厂商推断: {{ nodeInfo.vendor }}
              </div>
            </div>

            <!-- 基本信息（非推断节点） -->
            <div v-if="!isInferred && deviceDetail?.identity" class="bg-bg-panel border border-border rounded-lg p-3 space-y-2">
              <div class="text-xs font-medium text-text-secondary uppercase tracking-wide">基本信息</div>
              <div class="grid grid-cols-2 gap-2 text-xs">
                <div class="text-text-muted">厂商:</div>
                <div class="text-text-primary">{{ deviceDetail.identity.vendor || '-' }}</div>
                <div class="text-text-muted">型号:</div>
                <div class="text-text-primary">{{ deviceDetail.identity.model || '-' }}</div>
                <div class="text-text-muted">版本:</div>
                <div class="text-text-primary font-mono">{{ deviceDetail.identity.version || '-' }}</div>
                <div class="text-text-muted">ESN:</div>
                <div class="text-text-primary font-mono">{{ deviceDetail.identity.serialNumber || '-' }}</div>
              </div>
            </div>

            <!-- 统计信息（非推断节点） -->
            <div v-if="!isInferred && deviceDetail" class="bg-bg-panel border border-border rounded-lg p-3">
              <div class="text-xs font-medium text-text-secondary uppercase tracking-wide mb-2">统计信息</div>
              <div class="flex items-center gap-4 text-xs">
                <div class="flex items-center gap-1">
                  <svg class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                    <path d="M4 6h16M4 12h16M4 18h16" stroke-width="2" stroke-linecap="round"/>
                  </svg>
                  <span class="text-text-muted">接口</span>
                  <span class="text-text-primary font-semibold">{{ deviceDetail.interfaces?.length || 0 }}</span>
                </div>
                <div class="flex items-center gap-1">
                  <svg class="w-4 h-4 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                    <circle cx="12" cy="12" r="10" stroke-width="2"/>
                    <path d="M12 6v6l4 2" stroke-width="2" stroke-linecap="round"/>
                  </svg>
                  <span class="text-text-muted">LLDP</span>
                  <span class="text-text-primary font-semibold">{{ deviceDetail.lldpNeighbors?.length || 0 }}</span>
                </div>
                <div class="flex items-center gap-1">
                  <svg class="w-4 h-4 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                    <path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z" stroke-width="2"/>
                  </svg>
                  <span class="text-text-muted">聚合</span>
                  <span class="text-text-primary font-semibold">{{ deviceDetail.aggregates?.length || 0 }}</span>
                </div>
              </div>
            </div>

            <!-- 角色和站点信息 -->
            <div v-if="nodeInfo?.role || nodeInfo?.site" class="bg-bg-panel border border-border rounded-lg p-3">
              <div class="text-xs font-medium text-text-secondary uppercase tracking-wide mb-2">拓扑属性</div>
              <div class="grid grid-cols-2 gap-2 text-xs">
                <div v-if="nodeInfo?.role">
                  <span class="text-text-muted">角色:</span>
                  <span class="ml-1 px-2 py-0.5 rounded text-text-primary bg-accent/20">{{ nodeInfo.role }}</span>
                </div>
                <div v-if="nodeInfo?.site">
                  <span class="text-text-muted">站点:</span>
                  <span class="ml-1 px-2 py-0.5 rounded text-text-primary bg-bg-hover">{{ nodeInfo.site }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 无数据状态 -->
          <div v-else class="text-center py-8 text-sm text-text-muted">
            无设备信息
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
import type { ParsedResult } from "../../services/api";
import type { GraphNode } from "./TopologyGraph.vue";

const props = defineProps<{
  show: boolean;
  loading: boolean;
  deviceDetail: ParsedResult | null;
  nodeInfo: GraphNode | undefined;
  deviceId: string;
}>();

const emit = defineEmits<{
  "update:show": [value: boolean];
}>();

// 判断是否为推断节点
const isInferred = computed(() => {
  return props.nodeInfo?.nodeType === "inferred" || props.nodeInfo?.nodeType === "unknown";
});

function closeModal() {
  emit("update:show", false);
}
</script>

<style scoped>
/* modal 过渡动画已移至全局 _animations.css */
/* 滚动条样式已移至全局 _scrollbar.css，请使用 scrollbar-custom 类 */
</style>