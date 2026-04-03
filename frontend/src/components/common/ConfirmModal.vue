<template>
  <div
    v-if="show"
    class="fixed inset-0 z-[1100] flex items-center justify-center"
  >
    <!-- 背景遮罩 -->
    <div
      class="absolute inset-0 bg-black/50 backdrop-blur-sm"
      @click="handleCancel"
    ></div>

    <!-- 弹窗内容 -->
    <div
      class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-sm mx-4 animate-slide-in"
    >
      <div class="p-6">
        <!-- 图标和标题 -->
        <div class="flex items-center gap-3 mb-4">
          <div :class="iconWrapperClass">
            <component :is="iconComponent" class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-lg font-semibold text-text-primary">{{ title }}</h3>
            <p v-if="subtitle" class="text-sm text-text-muted">
              {{ subtitle }}
            </p>
          </div>
        </div>

        <!-- 提示信息 -->
        <p class="text-sm text-text-secondary mb-6">
          <slot>{{ message }}</slot>
        </p>

        <!-- 操作按钮 -->
        <div class="flex items-center justify-end gap-3">
          <button
            @click="handleCancel"
            class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
          >
            {{ cancelText }}
          </button>
          <button
            @click="handleConfirm"
            :disabled="loading"
            :class="confirmButtonClass"
          >
            <svg
              v-if="loading"
              class="animate-spin -ml-1 mr-2 h-4 w-4 inline"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ loading ? loadingText : confirmText }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch, ref } from "vue";

// Props
interface Props {
  show: boolean;
  type?: "danger" | "warning" | "info";
  title: string;
  subtitle?: string;
  message?: string;
  confirmText?: string;
  cancelText?: string;
  loadingText?: string;
  loading?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  type: "warning",
  subtitle: "",
  message: "",
  confirmText: "确认",
  cancelText: "取消",
  loadingText: "处理中...",
  loading: false,
});

// Emits
interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "confirm"): void;
  (e: "cancel"): void;
}

const emit = defineEmits<Emits>();

// 内部 loading 状态（支持外部控制或内部自动管理）
const internalLoading = ref(false);
const isLoading = computed(() => props.loading || internalLoading.value);

// 图标组件
const iconComponent = computed(() => {
  const icons = {
    danger: DangerIcon,
    warning: WarningIcon,
    info: InfoIcon,
  };
  return icons[props.type];
});

// 图标包装器样式
const iconWrapperClass = computed(() => {
  const classes = {
    danger: "p-2 bg-error/10 rounded-lg",
    warning: "p-2 bg-warning/10 rounded-lg",
    info: "p-2 bg-accent/10 rounded-lg",
  };
  return classes[props.type];
});

// 确认按钮样式
const confirmButtonClass = computed(() => {
  const baseClass =
    "px-4 py-2 text-sm font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed";
  const typeClasses = {
    danger: "text-white bg-error hover:bg-error/90",
    warning: "text-white bg-warning hover:bg-warning/90",
    info: "text-white bg-accent hover:bg-accent/90",
  };
  return `${baseClass} ${typeClasses[props.type]}`;
});

// 处理取消
const handleCancel = () => {
  if (isLoading.value) return;
  emit("update:show", false);
  emit("cancel");
};

// 处理确认
const handleConfirm = () => {
  if (isLoading.value) return;
  emit("confirm");
};

// 监听 show 变化，重置状态
watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      internalLoading.value = false;
    }
  },
);

// 暴露方法供父组件使用
defineExpose({
  setLoading: (value: boolean) => {
    internalLoading.value = value;
  },
});

// 图标组件定义
const DangerIcon = {
  template: `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  `,
};

const WarningIcon = {
  template: `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  `,
};

const InfoIcon = {
  template: `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="16" x2="12" y2="12" />
      <line x1="12" y1="8" x2="12.01" y2="8" />
    </svg>
  `,
};
</script>
