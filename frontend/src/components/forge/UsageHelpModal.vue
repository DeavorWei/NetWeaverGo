<script setup lang="ts">
defineProps<{
  show: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();
</script>

<template>
  <Transition name="modal">
    <div v-if="show" class="modal-container modal-active">
      <div class="modal-overlay" @click="$emit('close')"></div>

      <div class="modal modal-lg modal-glass">
        <div class="modal-header">
          <h3 class="modal-header-title">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-accent"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            ConfigForge 使用简介
          </h3>
          <button @click="$emit('close')" class="modal-close">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        <div class="modal-body text-sm text-text-secondary space-y-5">
          <p class="leading-relaxed text-base">
            <strong class="text-text-primary">ConfigForge</strong>
            是一款强大的自动化配置生成工具，所有核心计算逻辑均由后端处理，前端仅负责表单提交和结果渲染。
          </p>

          <div
            class="bg-accent-bg/50 rounded-xl p-5 border border-accent/20 shadow-inner"
          >
            <h4 class="font-semibold text-accent mb-4 flex items-center">
              <span class="text-base mr-1.5">🚀</span> 核心工作流：
            </h4>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-sm font-bold text-text-primary mb-1.5"
                  >1. Template Input (模板输入)</span
                >
                <span class="text-text-muted leading-relaxed"
                  >在左侧输入配置框架，使用
                  <code
                    class="px-1.5 py-0.5 bg-bg-tertiary rounded-md border border-border text-accent font-mono"
                    >[A]</code
                  >,
                  <code
                    class="px-1.5 py-0.5 bg-bg-tertiary rounded-md border border-border text-accent font-mono"
                    >[B]</code
                  >
                  等变量占位。</span
                >
              </li>
              <li class="flex flex-col">
                <span class="text-sm font-bold text-text-primary mb-1.5"
                  >2. Variables Mapping (变量映射)</span
                >
                <span class="text-text-muted leading-relaxed"
                  >在中列输入变量值。后端自动处理<strong class="text-accent"
                    >语法糖展开</strong
                  >及<strong class="text-accent">等差数列补全</strong>。</span
                >
              </li>
              <li class="flex flex-col">
                <span class="text-sm font-bold text-text-primary mb-1.5"
                  >3. Output Preview (输出预览)</span
                >
                <span class="text-text-muted leading-relaxed"
                  >右侧实时显示后端生成的配置块，前端无需任何计算。</span
                >
              </li>
            </ul>
          </div>

          <div
            class="flex items-start bg-info-bg rounded-xl p-4 border border-info/30 shadow-sm"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-info mt-0.5 shrink-0 mr-2.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 10V3L4 14h7v7l9-11h-7z"
              />
            </svg>
            <p class="text-sm text-info leading-relaxed">
              <strong>后端计算：</strong
              >变量替换、语法糖展开、等差数列推断、IP验证等核心逻辑均在后端执行，前端仅负责表单提交和结果渲染。
            </p>
          </div>
        </div>

        <div class="modal-footer">
          <button @click="$emit('close')" class="btn btn-primary">
            我知道了
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>
