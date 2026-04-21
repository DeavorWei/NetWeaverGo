<template>
  <div class="action-buttons">
    <button
      class="action-btn primary"
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
      class="action-btn primary"
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
      class="action-btn"
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
      class="action-btn"
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

<style scoped>
.action-buttons {
  @apply flex flex-col items-center gap-3 w-[80px];
}

.action-btn {
  @apply w-10 h-10 flex items-center justify-center rounded-lg border transition-all duration-200;
  @apply bg-bg-card border-border text-text-secondary hover:text-accent hover:border-accent/50 hover:bg-accent/5;
}

.action-btn:disabled {
  @apply opacity-40 cursor-not-allowed hover:text-text-secondary hover:border-border hover:bg-bg-card;
}

.action-btn.primary {
  @apply bg-accent text-white border-accent hover:bg-accent/90;
}

.action-btn.primary:disabled {
  @apply bg-accent/50 border-accent/50;
}
</style>
