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

<style scoped>
.help-tip {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  vertical-align: middle;
  cursor: help;
  outline: none;
  z-index: 120;
}

.help-tip-icon {
  width: 16px;
  height: 16px;
  border-radius: 9999px;
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-panel);
  color: var(--color-text-muted);
  font-size: 11px;
  line-height: 1;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: border-color 0.2s ease, color 0.2s ease, background-color 0.2s ease;
}

.help-tip-bubble {
  position: absolute;
  left: 0;
  bottom: calc(100% + 10px);
  transform: translateY(4px);
  width: max-content;
  max-width: 260px;
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-primary);
  color: var(--color-text-primary);
  font-size: 12px;
  line-height: 1.45;
  white-space: normal;
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.15);
  opacity: 0;
  pointer-events: none;
  z-index: 200;
  transition: opacity 0.2s ease, transform 0.2s ease;
}

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
  border-color: var(--color-accent);
  color: var(--color-accent);
}

.help-tip:hover .help-tip-bubble,
.help-tip:focus-visible .help-tip-bubble {
  opacity: 1;
  transform: translateY(0);
}
</style>
