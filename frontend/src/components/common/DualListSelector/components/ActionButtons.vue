<template>
  <div class="flex flex-col items-center gap-3 w-[80px]">
    <button
      class="w-10 h-10 flex items-center justify-center rounded-lg border transition-all duration-200 bg-accent text-white border-accent hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed disabled:bg-accent/50 disabled:border-accent/50"
      :disabled="!canMoveRight"
      @click="$emit('move-right')"
      title="右移选中项"
    >
      <svg
        class="w-5 h-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M9 18l6-6-6-6" />
      </svg>
    </button>

    <button
      class="w-10 h-10 flex items-center justify-center rounded-lg border transition-all duration-200 bg-accent text-white border-accent hover:bg-accent/90 disabled:opacity-40 disabled:cursor-not-allowed disabled:bg-accent/50 disabled:border-accent/50"
      :disabled="!canMoveLeft"
      @click="$emit('move-left')"
      title="左移选中项"
    >
      <svg
        class="w-5 h-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M15 18l-6-6 6-6" />
      </svg>
    </button>

    <div class="w-full h-px bg-border my-1"></div>

    <button
      class="w-10 h-10 flex items-center justify-center rounded-lg border transition-all duration-200 bg-bg-card border-border text-text-secondary hover:text-accent hover:border-accent/50 hover:bg-accent/5 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:text-text-secondary disabled:hover:border-border disabled:hover:bg-bg-card"
      :disabled="!canMoveAllRight"
      @click="$emit('move-all-right')"
      title="全部右移"
    >
      <svg
        class="w-5 h-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M13 5l7 7-7 7M5 5l7 7-7 7" />
      </svg>
    </button>

    <button
      class="w-10 h-10 flex items-center justify-center rounded-lg border transition-all duration-200 bg-bg-card border-border text-text-secondary hover:text-accent hover:border-accent/50 hover:bg-accent/5 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:text-text-secondary disabled:hover:border-border disabled:hover:bg-bg-card"
      :disabled="!canMoveAllLeft"
      @click="$emit('move-all-left')"
      title="全部左移"
    >
      <svg
        class="w-5 h-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M11 19l-7-7 7-7M19 19l-7-7 7-7" />
      </svg>
    </button>
  </div>
</template>

<script setup lang="ts">
interface Props {
  canMoveRight: boolean;
  canMoveLeft: boolean;
  canMoveAllRight: boolean;
  canMoveAllLeft: boolean;
}

defineProps<Props>();

defineEmits<{
  "move-right": [];
  "move-left": [];
  "move-all-right": [];
  "move-all-left": [];
}>();
</script>


