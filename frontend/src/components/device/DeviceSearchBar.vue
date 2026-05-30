<template>
  <div class="flex items-center justify-between">
    <!-- 搜索组件 -->
    <div class="flex items-center gap-2">
      <el-select v-model="localSearchType" style="width: 120px;">
        <el-option
          v-for="opt in searchOptions"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-select>
      
      <el-input
        v-model="localSearchQuery"
        :placeholder="'搜索' + currentSearchLabel + '...'"
        clearable
        @clear="handleReset"
        :prefix-icon="Search"
        style="width: 260px;"
      />
    </div>

    <!-- 新增按钮 -->
    <el-button type="primary" :icon="Plus" @click="$emit('add')">
      新增设备
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Search, Plus } from "@element-plus/icons-vue";
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
