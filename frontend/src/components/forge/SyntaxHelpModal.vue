<script setup lang="ts">
defineProps<{
  show: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();
</script>

<template>
  <el-dialog
    :model-value="show"
    @update:model-value="$emit('close')"
    title="语法糖智能展开说明"
    width="800px"
    class="custom-dialog"
    destroy-on-close
  >
    <div class="text-sm text-text-secondary space-y-5">
      <p class="leading-relaxed">
        语法糖展开和等差数列推断由<strong class="text-accent"
          >后端服务</strong
        >处理。输入格式后失去焦点时，后端会自动展开。
      </p>

      <div
        class="bg-accent-bg/50 rounded-xl p-5 border border-accent/20 shadow-inner"
      >
        <h4 class="font-semibold text-accent mb-4 flex items-center">
          <span class="text-base mr-1.5">📌</span> 常规连字符生成：
        </h4>
        <ul class="space-y-4">
          <li class="flex flex-col">
            <span class="text-xs font-medium text-text-muted mb-1.5"
              >▶ 基础数字序列</span
            >
            <div class="flex items-center space-x-2.5">
              <code
                class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border text-text-primary font-mono"
                >1-5</code
              >
              <span class="text-text-muted text-xs">➜</span>
              <code
                class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border text-text-secondary font-mono"
                >1, 2, 3, 4, 5</code
              >
            </div>
          </li>
          <li class="flex flex-col">
            <span class="text-xs font-medium text-text-muted mb-1.5"
              >▶ 携带前后缀</span
            >
            <div class="flex items-center space-x-2.5">
              <code
                class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border text-text-primary font-mono"
                >vlan10-12</code
              >
              <span class="text-text-muted text-xs">➜</span>
              <code
                class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border text-text-secondary font-mono"
                >vlan10, vlan11, vlan12</code
              >
            </div>
          </li>
        </ul>
      </div>

      <div
        class="bg-success-bg/50 rounded-xl p-5 border border-success/20 shadow-inner"
      >
        <h4 class="font-semibold text-success mb-4 flex items-center">
          <span class="text-base mr-1.5">🪄</span> 等差数列智能补全：
        </h4>
        <p class="text-sm text-success leading-relaxed mb-3">
          当其他变量已有确定长度时，输入
          <strong>2个及以上</strong> 构成规律的数列，后端将自动补全对齐。
        </p>
      </div>
    </div>

    <template #footer>
      <el-button type="primary" @click="$emit('close')">我知道了</el-button>
    </template>
  </el-dialog>
</template>
