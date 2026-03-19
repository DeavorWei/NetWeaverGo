<template>
  <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center">
    <!-- 背景遮罩 -->
    <div
      class="absolute inset-0 bg-black/50 backdrop-blur-sm"
      @click="$emit('close')"
    ></div>

    <!-- 弹窗内容 -->
    <div
      class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-sm mx-4 animate-slide-in"
    >
      <div class="p-6">
        <!-- 图标和标题 -->
        <div class="flex items-center gap-3 mb-4">
          <div class="p-2 bg-error-bg rounded-lg">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-6 h-6 text-error"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
          </div>
          <div>
            <h3 class="text-lg font-semibold text-text-primary">
              {{ isBatch ? "批量删除确认" : "确认删除" }}
            </h3>
            <p class="text-sm text-text-muted">此操作不可撤销</p>
          </div>
        </div>

        <!-- 提示信息 -->
        <p v-if="isBatch" class="text-sm text-text-secondary mb-6">
          确定要删除选中的
          <span class="font-mono text-accent font-bold">{{
            selectedCount
          }}</span>
          台设备吗？
        </p>
        <p v-else class="text-sm text-text-secondary mb-6">
          确定要删除设备
          <span class="font-mono text-accent">{{ device?.ip }}</span>
          吗？
        </p>

        <!-- 操作按钮 -->
        <div class="flex items-center justify-end gap-3">
          <button
            @click="$emit('close')"
            class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
          >
            取消
          </button>
          <button
            @click="handleConfirm"
            :disabled="isDeleting"
            class="px-4 py-2 text-sm font-medium text-white bg-error hover:bg-error/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ isDeleting ? "删除中..." : isBatch ? "确认删除" : "删除" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import type { DeviceAsset } from "@/services/api";

// Props
interface Props {
  show: boolean;
  isBatch: boolean;
  device?: DeviceAsset | null;
  selectedCount?: number;
}

const props = withDefaults(defineProps<Props>(), {
  device: null,
  selectedCount: 0,
});

// Emits
interface Emits {
  (e: "close"): void;
  (e: "confirm"): void;
}

const emit = defineEmits<Emits>();

// 状态
const isDeleting = ref(false);

// 监听 show 变化，重置状态
watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      isDeleting.value = false;
    }
  },
);

// 确认删除
function handleConfirm() {
  emit("confirm");
}

// 暴露方法供父组件使用
defineExpose({
  setDeleting: (value: boolean) => {
    isDeleting.value = value;
  },
});
</script>
