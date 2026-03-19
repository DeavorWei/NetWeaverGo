<template>
  <div class="flex items-center justify-between">
    <!-- 搜索组件 -->
    <div class="flex items-center gap-2">
      <select
        v-model="localSearchType"
        class="px-3 h-9 text-sm bg-bg-panel border border-border rounded-lg text-text-primary focus:border-accent focus:outline-none transition-colors cursor-pointer"
      >
        <option
          v-for="opt in searchOptions"
          :key="opt.value"
          :value="opt.value"
        >
          {{ opt.label }}
        </option>
      </select>
      <div class="relative">
        <input
          v-model="localSearchQuery"
          type="text"
          :placeholder="'搜索' + currentSearchLabel + '...'"
          class="w-64 pl-10 pr-10 h-9 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
        />
        <!-- 搜索图标 -->
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted pointer-events-none"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <!-- 重置按钮 -->
        <button
          v-if="localSearchQuery"
          @click="handleReset"
          class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text-primary hover:bg-bg-hover rounded transition-all"
          title="清空搜索"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
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
    </div>

    <!-- 新增按钮 -->
    <button
      @click="$emit('add')"
      class="flex items-center gap-2 px-4 h-9 text-sm font-medium text-white bg-accent hover:bg-accent/90 rounded-lg transition-all duration-200 shadow-sm"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="w-4 h-4"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <line x1="12" y1="5" x2="12" y2="19" />
        <line x1="5" y1="12" x2="19" y2="12" />
      </svg>
      新增设备
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import type { SearchType, SearchOption } from "@/composables/useDeviceSearch";

// Props
interface Props {
  searchType: SearchType;
  searchQuery: string;
  searchOptions: SearchOption[];
}

const props = defineProps<Props>();

// Emits
interface Emits {
  (e: "update:searchType", value: SearchType): void;
  (e: "update:searchQuery", value: string): void;
  (e: "reset"): void;
  (e: "add"): void;
}

const emit = defineEmits<Emits>();

// 本地状态
const localSearchType = ref<SearchType>(props.searchType);
const localSearchQuery = ref(props.searchQuery);

// 当前搜索类型的标签
const currentSearchLabel = computed(() => {
  const opt = props.searchOptions.find(
    (o) => o.value === localSearchType.value,
  );
  return opt ? opt.label : "";
});

// 同步外部 props
watch(
  () => props.searchType,
  (val) => {
    localSearchType.value = val;
  },
);
watch(
  () => props.searchQuery,
  (val) => {
    localSearchQuery.value = val;
  },
);

// 同步内部状态到外部
watch(localSearchType, (val) => {
  emit("update:searchType", val);
});
watch(localSearchQuery, (val) => {
  emit("update:searchQuery", val);
});

// 重置搜索
function handleReset() {
  localSearchQuery.value = "";
  emit("reset");
}
</script>
