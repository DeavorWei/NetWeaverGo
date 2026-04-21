<template>
  <div class="inline-flex items-center gap-1.5">
    <template v-if="hasFile">
      <!-- 打开文件按钮 -->
      <button
        class="btn-file"
        :class="{
          'btn-file-sm': size === 'small',
          'btn-file-lg': size === 'large',
        }"
        :title="`打开${fileTypeText}`"
        @click="handleOpenFile"
        :disabled="!exists"
      >
        <svg
          viewBox="0 0 24 24"
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"
          ></path>
          <polyline points="15 3 21 3 21 9"></polyline>
          <line x1="10" y1="14" x2="21" y2="3"></line>
        </svg>
        <span v-if="showText">打开</span>
      </button>

      <!-- 打开文件夹按钮 -->
      <button
        class="btn-file"
        :class="{
          'btn-file-sm': size === 'small',
          'btn-file-lg': size === 'large',
        }"
        :title="`打开${fileTypeText}所在文件夹`"
        @click="handleOpenFolder"
        :disabled="!exists"
      >
        <svg
          viewBox="0 0 24 24"
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"
          ></path>
          <line x1="12" y1="11" x2="12" y2="17"></line>
          <line x1="9" y1="14" x2="15" y2="14"></line>
        </svg>
        <span v-if="showText">文件夹</span>
      </button>
    </template>

    <!-- 文件不存在提示 -->
    <span v-if="hasFile && !exists" class="text-xs text-text-muted italic">
      文件不存在
    </span>

    <!-- 无文件提示 -->
    <span v-else-if="!hasFile" class="text-xs text-text-muted italic">
      无{{ fileTypeText }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { ExecutionHistoryAPI } from "../../services/api";
import { useToast } from "../../utils/useToast";
import type { FileType } from "../../types/executionHistory";

const props = defineProps<{
  runId: string;
  unitId?: string;
  fileType: FileType;
  hasFile?: boolean; // 是否有该类型文件
  exists?: boolean; // 文件是否存在
  size?: "small" | "medium" | "large";
  showText?: boolean;
}>();

const toast = useToast();

const fileTypeText = computed(() => {
  const textMap: Record<string, string> = {
    detail: "详细日志",
    raw: "原始日志",
    report: "报告",
    summary: "摘要日志",
    journal: "流水日志",
  };
  return textMap[props.fileType] || "文件";
});

// 打开文件 - 使用系统默认应用打开
const handleOpenFile = async () => {
  try {
    const result = await ExecutionHistoryAPI.openFileWithDefaultApp({
      runId: props.runId,
      unitId: props.unitId || "",
      fileType: props.fileType,
    });

    if (result && !result.success) {
      toast.error(result.message || "打开文件失败");
    }
  } catch (error) {
    toast.error(`打开${fileTypeText.value}失败: ${error}`);
  }
};

// 打开文件夹 - 打开所在文件夹并选中文件
const handleOpenFolder = async () => {
  try {
    const result = await ExecutionHistoryAPI.openFileLocation({
      runId: props.runId,
      unitId: props.unitId || "",
      fileType: props.fileType,
    });

    if (result && !result.success) {
      toast.error(result.message || "打开文件夹失败");
    }
  } catch (error) {
    toast.error(`打开文件夹失败: ${error}`);
  }
};
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 组件已完全使用 index.css 中的类，无需额外样式 */
</style>
