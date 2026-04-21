<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-50 flex items-center justify-center"
        @keydown.esc="handleCancel"
      >
        <!-- 遮罩层 -->
        <div
          class="absolute inset-0 bg-black/60 backdrop-blur-sm"
          @click="handleCancel"
        ></div>

        <!-- 弹窗主体 -->
        <div
          class="relative w-[900px] max-h-[90vh] bg-bg-card border border-border rounded-2xl shadow-2xl flex flex-col overflow-hidden"
        >
          <!-- 头部 -->
          <div
            class="flex items-center justify-between px-6 py-4 border-b border-border bg-bg-panel"
          >
            <h3 class="text-base font-semibold text-text-primary">
              {{ mergedConfig.modalTitle }}
            </h3>
            <button
              @click="handleCancel"
              class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
            >
              <svg
                class="w-5 h-5"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- 内容区 -->
          <div class="flex-1 p-6 overflow-hidden">
            <!-- 筛选栏 -->
            <div class="filter-bar">
              <div class="flex items-center gap-3">
                <span
                  class="text-xs font-medium text-text-muted uppercase tracking-wider"
                  >筛选方式:</span
                >
                <div class="flex gap-1.5">
                  <button
                    v-for="filter in filterOptions"
                    :key="filter.value"
                    @click="handleFilterChange(filter.value)"
                    :class="[
                      'px-2.5 py-1.5 text-xs font-medium rounded-md border transition-all duration-200',
                      currentFilter === filter.value
                        ? 'bg-accent border-accent text-white shadow-sm'
                        : 'bg-bg-card border-border text-text-muted hover:text-text-primary hover:border-accent/30',
                    ]"
                  >
                    {{ filter.label }}
                  </button>
                </div>
              </div>

              <!-- 分组/标签选择 -->
              <div
                v-if="showSubFilter"
                class="flex items-center gap-3 pl-4 border-l border-border/50"
              >
                <span class="text-xs font-medium text-text-muted"
                  >{{ subFilterLabel }}:</span
                >
                <select
                  v-model="filterValue"
                  class="px-3 py-1.5 rounded-md bg-bg-card border border-border text-xs text-text-primary focus:outline-none focus:border-accent/50 min-w-[120px]"
                >
                  <option value="">全部</option>
                  <option
                    v-for="opt in subFilterOptions"
                    :key="opt.value"
                    :value="opt.value"
                  >
                    {{ opt.label }} ({{ opt.count }})
                  </option>
                </select>
              </div>

              <!-- 搜索框 -->
              <div v-if="mergedConfig.enableSearch" class="flex-1 flex justify-end">
                <div class="relative">
                  <input
                    v-model="searchKeyword"
                    type="text"
                    placeholder="搜索..."
                    class="search-input pl-9"
                  />
                  <svg
                    class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <circle cx="11" cy="11" r="8" />
                    <path d="m21 21-4.35-4.35" />
                  </svg>
                </div>
              </div>
            </div>

            <!-- 主选择区 -->
            <div class="selector-content">
              <!-- 左栏 -->
              <SourcePanel
                :title="mergedConfig.sourceTitle"
                :items="filteredSourceData"
                :selected="sourceSelected"
                :stats="sourceStats"
                @toggle="toggleSourceSelection"
              />

              <!-- 操作按钮 -->
              <ActionButtons
                :can-move-right="canMoveRight"
                :can-move-left="canMoveLeft"
                :can-move-all-right="canMoveAllRight"
                :can-move-all-left="canMoveAllLeft"
                @move-right="handleMoveRight"
                @move-left="handleMoveLeft"
                @move-all-right="handleMoveAllRight"
                @move-all-left="handleMoveAllLeft"
              />

              <!-- 右栏 -->
              <TargetPanel
                :title="mergedConfig.targetTitle"
                :items="targetData"
                :selected="targetSelected"
                :stats="targetStats"
                @toggle="toggleTargetSelection"
              />
            </div>

            <!-- 底部信息 -->
            <div class="footer-info">
              <span class="text-sm text-text-secondary">
                已选择
                <span class="font-semibold text-accent">{{
                  targetData.length
                }}</span>
                / {{ sourceData.length }} 项
              </span>
              <span v-if="mergedConfig.maxSelection" class="text-xs text-text-muted">
                最多可选择 {{ mergedConfig.maxSelection }} 项
              </span>
            </div>
          </div>

          <!-- 底部按钮 -->
          <div
            class="footer-actions px-6 py-4 border-t border-border bg-bg-panel"
          >
            <button class="btn-cancel" @click="handleCancel">
              {{ mergedConfig.cancelText }}
            </button>
            <button class="btn-confirm" @click="handleConfirm">
              {{ mergedConfig.confirmText }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch, toRef } from "vue";
import SourcePanel from "./components/SourcePanel.vue";
import TargetPanel from "./components/TargetPanel.vue";
import ActionButtons from "./components/ActionButtons.vue";
import { useDualListSelection } from "./composables/useDualListSelection";
import type {
  ListItem,
  SelectorConfig,
  GroupData,
} from "./types/dualListSelector";
import { DEFAULT_CONFIG } from "./types/dualListSelector";

// Props
interface Props {
  /** 是否显示弹窗 */
  visible: boolean;
  /** 源数据列表 */
  sourceData: ListItem[];
  /** 初始已选数据 */
  targetData?: ListItem[];
  /** 分组数据（可选） */
  groupData?: GroupData[];
  /** 标签数据（可选） */
  tagData?: { key: string; label: string }[];
  /** 协议数据（可选） */
  protocolData?: { key: string; label: string }[];
  /** 组件配置 */
  config?: Partial<SelectorConfig>;
  /** 加载状态 */
  loading?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  targetData: () => [],
  groupData: () => [],
  tagData: () => [],
  protocolData: () => [],
  config: () => ({}),
  loading: false,
});

// Emits
const emit = defineEmits<{
  "update:visible": [value: boolean];
  confirm: [items: ListItem[]];
  cancel: [];
  change: [items: ListItem[]];
}>();

// 合并配置
const mergedConfig = computed<SelectorConfig>(() => ({
  ...DEFAULT_CONFIG,
  ...props.config,
}));

// 目标数据（已选）
const targetDataRef = ref<ListItem[]>([...props.targetData]);

// 监听 props.targetData 变化
watch(
  () => props.targetData,
  (newVal) => {
    targetDataRef.value = [...newVal];
  },
  { deep: true }
);

// 源数据 ref
const sourceDataRef = toRef(props, "sourceData");

// 使用组合式函数
const {
  currentFilter,
  filterValue,
  searchKeyword,
  sourceSelected,
  targetSelected,
  filteredSourceData,
  sourceStats,
  targetStats,
  canMoveRight,
  canMoveLeft,
  canMoveAllRight,
  canMoveAllLeft,
  toggleSourceSelection,
  toggleTargetSelection,
  moveRight,
  moveLeft,
  moveAllRight,
  moveAllLeft,
  resetState,
  applyFilter,
} = useDualListSelection(sourceDataRef, targetDataRef, mergedConfig);

// 筛选选项
const filterOptions = computed(() => {
  const options = [{ label: "全部", value: "all" }];
  if (mergedConfig.value.enableGrouping && props.groupData.length > 0) {
    options.push({ label: "按分组", value: "group" });
  }
  if (mergedConfig.value.enableTagFilter && props.tagData.length > 0) {
    options.push({ label: "按标签", value: "tag" });
  }
  if (props.protocolData.length > 0) {
    options.push({ label: "按协议", value: "protocol" });
  }
  return options;
});

// 是否显示子筛选
const showSubFilter = computed(() => {
  return (
    (currentFilter.value === "group" && props.groupData.length > 0) ||
    (currentFilter.value === "tag" && props.tagData.length > 0) ||
    (currentFilter.value === "protocol" && props.protocolData.length > 0)
  );
});

// 子筛选标签
const subFilterLabel = computed(() => {
  if (currentFilter.value === "group") return "分组";
  if (currentFilter.value === "tag") return "标签";
  if (currentFilter.value === "protocol") return "协议";
  return "";
});

// 子筛选选项
const subFilterOptions = computed(() => {
  if (currentFilter.value === "group") {
    return props.groupData.map((group) => ({
      label: group.label,
      value: group.key,
      count: group.items.length,
    }));
  }
  if (currentFilter.value === "tag") {
    // 统计每个标签的数量
    const tagCounts = new Map<string, number>();
    props.sourceData.forEach((item) => {
      item.tags?.forEach((tag) => {
        tagCounts.set(tag, (tagCounts.get(tag) || 0) + 1);
      });
    });
    return props.tagData.map((tag) => ({
      label: tag.label,
      value: tag.key,
      count: tagCounts.get(tag.key) || 0,
    }));
  }
  if (currentFilter.value === "protocol") {
    // 统计每个协议的数量
    const protocolCounts = new Map<string, number>();
    props.sourceData.forEach((item) => {
      if (item.protocol) {
        protocolCounts.set(item.protocol, (protocolCounts.get(item.protocol) || 0) + 1);
      }
    });
    return props.protocolData.map((protocol) => ({
      label: protocol.label,
      value: protocol.key,
      count: protocolCounts.get(protocol.key) || 0,
    }));
  }
  return [];
});

// 处理筛选变化
const handleFilterChange = (filterType: string) => {
  applyFilter(filterType, "");
};

// 处理右移
const handleMoveRight = () => {
  const itemsToMove = moveRight();
  const existingKeys = new Set(targetDataRef.value.map((item) => item.key));
  const newItems = itemsToMove.filter((item) => !existingKeys.has(item.key));
  targetDataRef.value = [...targetDataRef.value, ...newItems];
  emit("change", targetDataRef.value);
};

// 处理左移
const handleMoveLeft = () => {
  const keysToRemove = new Set(moveLeft());
  targetDataRef.value = targetDataRef.value.filter(
    (item) => !keysToRemove.has(item.key)
  );
  emit("change", targetDataRef.value);
};

// 处理全右移
const handleMoveAllRight = () => {
  const itemsToMove = moveAllRight();
  const existingKeys = new Set(targetDataRef.value.map((item) => item.key));
  const newItems = itemsToMove.filter((item) => !existingKeys.has(item.key));
  targetDataRef.value = [...targetDataRef.value, ...newItems];
  emit("change", targetDataRef.value);
};

// 处理全左移
const handleMoveAllLeft = () => {
  moveAllLeft();
  targetDataRef.value = [];
  emit("change", targetDataRef.value);
};

// 处理确认
const handleConfirm = () => {
  emit("confirm", targetDataRef.value);
  emit("update:visible", false);
  resetState();
};

// 处理取消
const handleCancel = () => {
  emit("cancel");
  emit("update:visible", false);
  resetState();
  // 恢复原始数据
  targetDataRef.value = [...props.targetData];
};
</script>

<style scoped>
.filter-bar {
  @apply flex items-center gap-4 p-4 bg-bg-panel/50 rounded-lg border border-border/50 mb-4;
}

.selector-content {
  @apply flex items-center justify-center gap-4 flex-1 min-h-0;
}

.search-input {
  @apply w-48 px-3 py-1.5 text-sm rounded-md bg-bg-card border border-border text-text-primary placeholder:text-text-muted;
  @apply focus:outline-none focus:border-accent/50 transition-all;
}

.footer-info {
  @apply flex items-center justify-between px-4 py-3 mt-4 bg-bg-panel/50 rounded-lg border border-border/50;
}

.footer-actions {
  @apply flex items-center justify-end gap-3;
}

.btn-cancel {
  @apply px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors;
}

.btn-confirm {
  @apply px-4 py-2 text-sm font-medium text-white bg-accent rounded-lg hover:bg-accent/90 transition-colors;
}

/* Modal 动画 */
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from > div:last-child,
.modal-leave-to > div:last-child {
  transform: scale(0.95);
}
</style>
