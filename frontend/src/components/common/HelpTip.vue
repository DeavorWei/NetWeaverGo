<template>
  <span class="help-tip" tabindex="0" aria-label="查看设置说明">
    <span class="help-tip-icon" aria-hidden="true">?</span>
    <span class="help-tip-bubble" role="tooltip">{{ text }}</span>
  </span>
</template>

<script setup lang="ts">
defineProps<{
  text: string
}>()
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

.help-tip {
  @apply relative inline-flex items-center justify-center align-middle;
  @apply cursor-help outline-none z-[120];
}

.help-tip-icon {
  @apply w-4 h-4 rounded-full border border-border bg-bg-panel;
  @apply text-text-muted text-[11px] leading-none font-bold;
  @apply inline-flex items-center justify-center;
  @apply transition-colors duration-200;
}

.help-tip-bubble {
  @apply absolute left-0 w-max max-w-[260px] p-2 px-2.5;
  @apply rounded-[10px] border border-border bg-bg-primary;
  @apply text-text-primary text-xs leading-relaxed;
  @apply whitespace-normal opacity-0 pointer-events-none z-[200];
  @apply transition-opacity duration-200;
  bottom: calc(100% + 10px);
  transform: translateY(4px);
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.15);
}

/* 伪元素 - 需保留原始 CSS */
.help-tip-bubble::after {
  content: "";
  position: absolute;
  left: 8px;
  top: 100%;
  border: 6px solid transparent;
  border-top-color: var(--color-bg-primary);
}

.help-tip:hover .help-tip-icon,
.help-tip:focus-visible .help-tip-icon {
  @apply border-accent text-accent;
}

.help-tip:hover .help-tip-bubble,
.help-tip:focus-visible .help-tip-bubble {
  @apply opacity-100;
  transform: translateY(0);
}
</style>
