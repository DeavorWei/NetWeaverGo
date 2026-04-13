<template>
  <div class="raw-file-preview">
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
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
        <span class="text-sm font-medium text-text-primary">Raw 文件预览</span>
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

    <!-- 设备信息 -->
    <div v-if="deviceIp" class="mb-3 text-xs text-text-muted">
      设备: <span class="text-text-primary font-mono">{{ deviceIp }}</span>
    </div>

    <!-- 命令选择 -->
    <div class="mb-3">
      <label class="block text-xs text-text-muted mb-1">选择命令</label>
      <select
        v-model="selectedCommand"
        class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
        :disabled="loading || commands.length === 0"
      >
        <option value="">请选择命令</option>
        <option
          v-for="cmd in commands"
          :key="cmd.commandKey"
          :value="cmd.commandKey"
        >
          {{ cmd.commandKey }} ({{ formatSize(cmd.fileSize) }})
        </option>
      </select>
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

    <!-- 无文件提示 -->
    <div
      v-else-if="commands.length === 0"
      class="p-4 bg-bg-panel rounded-lg text-sm text-text-muted text-center"
    >
      暂无 Raw 文件
    </div>

    <!-- 文件内容 -->
    <div v-else-if="preview && preview.exists" class="space-y-2">
      <!-- 文件信息 -->
      <div class="flex items-center gap-4 text-xs text-text-muted">
        <span>大小: {{ formatSize(preview.size) }}</span>
        <span
          >路径: <span class="font-mono">{{ preview.filePath }}</span></span
        >
      </div>

      <!-- 内容区域 -->
      <div class="bg-bg-panel border border-border rounded-lg overflow-hidden">
        <pre
          class="p-3 text-xs font-mono text-text-primary overflow-auto max-h-[400px] whitespace-pre-wrap break-all"
          >{{ preview.content }}</pre
        >
      </div>
    </div>

    <!-- 未选择命令 -->
    <div
      v-else-if="!selectedCommand"
      class="p-4 bg-bg-panel rounded-lg text-sm text-text-muted text-center"
    >
      请选择要预览的命令
    </div>

    <!-- 文件不存在 -->
    <div
      v-else
      class="p-4 bg-warning/10 border border-warning/30 rounded-lg text-sm text-warning"
    >
      文件不存在
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from "vue";
import { TaskExecutionAPI } from "@/services/api";
import type {
  RawFileInfo,
  RawFilePreview,
} from "@/bindings/github.com/NetWeaverGo/core/internal/taskexec/models";

interface Props {
  runId: string;
  deviceIp: string;
  closable?: boolean;
}

const props = defineProps<Props>();
defineEmits(["close"]);

const loading = ref(false);
const error = ref<string | null>(null);
const commands = ref<RawFileInfo[]>([]);
const selectedCommand = ref("");
const preview = ref<RawFilePreview | null>(null);

// 格式化文件大小
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// 加载命令列表
async function loadCommands() {
  if (!props.runId || !props.deviceIp) return;

  loading.value = true;
  error.value = null;

  try {
    const result = await TaskExecutionAPI.listRawFiles(
      props.runId,
      props.deviceIp,
    );
    commands.value = result || [];

    // 如果只有一个命令，自动选择
    if (commands.value.length === 1 && commands.value[0]) {
      selectedCommand.value = commands.value[0].commandKey;
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : "加载命令列表失败";
    commands.value = [];
  } finally {
    loading.value = false;
  }
}

// 加载文件预览
async function loadPreview() {
  if (!props.runId || !props.deviceIp || !selectedCommand.value) {
    preview.value = null;
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    const result = await TaskExecutionAPI.getRawFilePreview(
      props.runId,
      props.deviceIp,
      selectedCommand.value,
    );
    preview.value = result;
  } catch (err) {
    error.value = err instanceof Error ? err.message : "加载文件内容失败";
    preview.value = null;
  } finally {
    loading.value = false;
  }
}

// 监听命令选择变化
watch(selectedCommand, () => {
  loadPreview();
});

// 监听属性变化
watch(
  () => [props.runId, props.deviceIp],
  () => {
    selectedCommand.value = "";
    preview.value = null;
    loadCommands();
  },
  { immediate: true },
);

onMounted(() => {
  loadCommands();
});
</script>
