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
      <!-- 标题栏 -->
      <div
        class="flex items-center justify-between px-6 py-4 border-b border-border"
      >
        <h3 class="text-lg font-semibold text-text-primary">
          批量修改{{ batchFieldLabel }}
          <span
            v-if="selectedCount > 0"
            class="ml-2 text-sm font-normal text-accent"
          >
            ({{ selectedCount }} 台设备)
          </span>
        </h3>
        <button
          @click="$emit('close')"
          class="p-1 text-text-muted hover:text-text-primary transition-colors"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-5 h-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <!-- 表单 -->
      <form @submit.prevent="handleSubmit" class="p-6 space-y-4">
        <p class="text-sm text-text-secondary">
          将选中的设备的{{ batchFieldLabel }}修改为：
        </p>

        <!-- 协议选择 -->
        <div v-if="field === 'protocol'">
          <select
            v-model="localValue"
            class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary focus:border-accent focus:outline-none transition-colors"
          >
            <option v-for="p in validProtocols" :key="p" :value="p">
              {{ p }}
            </option>
          </select>
        </div>

        <!-- 端口输入 -->
        <div v-else-if="field === 'port'">
          <input
            v-model.number="localValue"
            type="number"
            placeholder="端口号"
            class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            min="1"
            max="65535"
          />
        </div>

        <!-- 用户名输入 -->
        <div v-else-if="field === 'username'">
          <input
            v-model="localValue"
            type="text"
            :placeholder="'请输入' + batchFieldLabel"
            class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
          />
        </div>

        <!-- 密码输入 -->
        <div v-else-if="field === 'password'">
          <input
            v-model="localValue"
            type="password"
            :placeholder="'请输入' + batchFieldLabel"
            class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
          />
        </div>

        <!-- 分组输入 -->
        <div v-else-if="field === 'group'">
          <input
            v-model="localValue"
            type="text"
            :placeholder="'请输入' + batchFieldLabel"
            class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
          />
        </div>

        <!-- 标签输入 -->
        <div v-else-if="field === 'tag'">
          <div class="space-y-2">
            <input
              v-model="localValue"
              type="text"
              :placeholder="'请输入' + batchFieldLabel + ' (多个用逗号分隔)'"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
            <p class="text-[10px] text-text-muted">
              提示：输入多个标签时请使用英文或中文逗号分隔。
            </p>
          </div>
        </div>

        <!-- 错误消息 -->
        <div
          v-if="errorMessage"
          class="px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg"
        >
          {{ errorMessage }}
        </div>

        <!-- 操作按钮 -->
        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            @click="$emit('close')"
            class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="isSaving"
            class="px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ isSaving ? "保存中..." : "确定" }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";

// 批量编辑字段类型
export type BatchField =
  | "group"
  | "protocol"
  | "port"
  | "username"
  | "password"
  | "tag";

// Props
interface Props {
  show: boolean;
  field: BatchField | null;
  selectedCount: number;
  validProtocols?: string[];
}

const props = withDefaults(defineProps<Props>(), {
  validProtocols: () => ["SSH", "SNMP", "TELNET"],
});

// Emits
interface Emits {
  (e: "close"): void;
  (e: "save", field: BatchField, value: string | number): void;
}

const emit = defineEmits<Emits>();

// 本地状态
const localValue = ref<string | number>("");
const errorMessage = ref("");
const isSaving = ref(false);

// 字段标签映射
const fieldLabels: Record<BatchField, string> = {
  protocol: "协议",
  port: "端口",
  username: "用户名",
  password: "密码",
  group: "分组",
  tag: "标签",
};

// 计算属性
const batchFieldLabel = computed(() => {
  return props.field ? fieldLabels[props.field] : "";
});

// 监听 show 变化，重置状态
watch(
  () => props.show,
  (newShow) => {
    if (newShow) {
      localValue.value = "";
      errorMessage.value = "";
      isSaving.value = false;
    }
  },
);

// 提交表单
function handleSubmit() {
  if (!props.field) return;

  // 验证
  if (props.field === "port") {
    const port = Number(localValue.value);
    if (isNaN(port) || port < 1 || port > 65535) {
      errorMessage.value = "端口号必须在 1-65535 之间";
      return;
    }
  }

  if (props.field === "tag" && !localValue.value) {
    errorMessage.value = "请输入标签";
    return;
  }

  emit("save", props.field, localValue.value);
}

// 暴露方法供父组件使用
defineExpose({
  setSaving: (value: boolean) => {
    isSaving.value = value;
  },
  setError: (value: string) => {
    errorMessage.value = value;
  },
});
</script>
