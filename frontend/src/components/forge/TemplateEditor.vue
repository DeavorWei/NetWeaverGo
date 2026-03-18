<script setup lang="ts">
defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const updateValue = (event: Event) => {
  const target = event.target as HTMLTextAreaElement;
  emit("update:modelValue", target.value);
};
</script>

<template>
  <div
    class="min-h-[300px] md:h-full md:min-h-[700px] glass-panel border border-border flex flex-col overflow-hidden"
  >
    <div class="card-header">
      <h2 class="card-header-title">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-accent"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
        配置模版
      </h2>
    </div>
    <div class="flex-1 p-4 flex flex-col">
      <textarea
        :value="modelValue"
        @input="updateValue"
        class="flex-1 w-full resize-none outline-none premium-textarea p-4 font-mono text-sm leading-relaxed"
        spellcheck="false"
        placeholder="在此处输入配置模板，例如：&#10;interface GigabitEthernet0/0/[A]&#10; description [B]&#10;..."
      ></textarea>
    </div>
  </div>
</template>
