<template>
  <div
    class="bg-bg-panel border border-border rounded-xl shadow-card overflow-hidden"
  >
    <!-- 表格容器 -->
    <div class="overflow-auto scrollbar-custom max-h-[calc(100vh-220px)]">
      <table class="w-full text-sm">
        <!-- 表头 -->
        <thead class="sticky top-0 z-10">
          <tr class="bg-bg-panel border-b border-border">
            <!-- 全选复选框 -->
            <th
              class="px-4 py-3.5 text-center text-xs font-semibold text-text-muted uppercase tracking-wider w-12"
            >
              <button
                @click="$emit('toggle-select-all')"
                :disabled="isSelectingAll"
                class="flex items-center justify-center w-4 h-4 mx-auto rounded border transition-all duration-200"
                :class="[
                  isAllSelected
                    ? 'bg-accent border-accent text-white'
                    : isIndeterminate
                      ? 'bg-accent/30 border-accent/50'
                      : 'border-border hover:border-accent',
                ]"
                :title="isAllSelected ? '取消全选' : '全选全部设备'"
              >
                <svg
                  v-if="isAllSelected"
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-3 h-3"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="3"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              </button>
            </th>
            <!-- 序号 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-12"
            >
              #
            </th>
            <!-- 分组 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-20"
            >
              <div class="flex items-center gap-1">
                分组
                <BatchEditButton
                  :disabled="selectedCount === 0"
                  field="group"
                  @click="$emit('batch-edit', 'group')"
                />
              </div>
            </th>
            <!-- IP 地址 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-28"
            >
              IP 地址
            </th>
            <!-- 协议 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
            >
              <div class="flex items-center gap-1">
                协议
                <BatchEditButton
                  :disabled="selectedCount === 0"
                  field="protocol"
                  @click="$emit('batch-edit', 'protocol')"
                />
              </div>
            </th>
            <!-- 端口 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-14"
            >
              <div class="flex items-center gap-1">
                端口
                <BatchEditButton
                  :disabled="selectedCount === 0"
                  field="port"
                  @click="$emit('batch-edit', 'port')"
                />
              </div>
            </th>
            <!-- 用户名 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
            >
              <div class="flex items-center gap-1">
                用户名
                <BatchEditButton
                  :disabled="selectedCount === 0"
                  field="username"
                  @click="$emit('batch-edit', 'username')"
                />
              </div>
            </th>
            <!-- 密码 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
            >
              <div class="flex items-center gap-1">
                密码
                <BatchEditButton
                  :disabled="selectedCount === 0"
                  field="password"
                  @click="$emit('batch-edit', 'password')"
                />
              </div>
            </th>
            <!-- 标签 -->
            <th
              class="px-4 py-3.5 text-left text-xs font-semibold text-text-muted uppercase tracking-wider w-16"
            >
              <div class="flex items-center gap-1">
                Tag
                <BatchEditButton
                  :disabled="selectedCount === 0"
                  field="tag"
                  @click="$emit('batch-edit', 'tag')"
                />
              </div>
            </th>
            <!-- 操作 -->
            <th
              class="px-4 py-3.5 text-center text-xs font-semibold text-text-muted uppercase tracking-wider w-24"
            >
              操作
            </th>
          </tr>
        </thead>

        <!-- 表体 -->
        <tbody class="divide-y divide-border">
          <!-- 加载中 -->
          <tr v-if="loading">
            <td colspan="10" class="px-5 py-12 text-center text-text-muted">
              <div class="flex flex-col items-center gap-3">
                <svg
                  class="animate-spin w-8 h-8 text-accent"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                <span class="text-sm">加载中...</span>
              </div>
            </td>
          </tr>

          <!-- 无数据 -->
          <tr v-else-if="devices.length === 0">
            <td colspan="10" class="px-5 py-12 text-center text-text-muted">
              <div class="flex flex-col items-center gap-3">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-10 h-10 text-text-muted/40"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <rect x="2" y="2" width="20" height="8" rx="2" />
                  <rect x="2" y="14" width="20" height="8" rx="2" />
                </svg>
                <span class="text-sm">暂无设备数据，点击上方按钮新增</span>
              </div>
            </td>
          </tr>

          <!-- 数据行 -->
          <tr
            v-else
            v-for="(row, idx) in devices"
            :key="row.id"
            :class="[
              'transition-colors duration-150 group',
              isSelected(row.id)
                ? 'bg-accent/8 hover:bg-accent/12'
                : 'hover:bg-bg-hover',
            ]"
          >
            <!-- 选择框 -->
            <td class="px-4 py-3 text-center">
              <button
                @click="$emit('toggle-select', row.id)"
                class="flex items-center justify-center w-4 h-4 mx-auto rounded border transition-all duration-200"
                :class="[
                  isSelected(row.id)
                    ? 'bg-accent border-accent text-white'
                    : 'border-border hover:border-accent',
                ]"
              >
                <svg
                  v-if="isSelected(row.id)"
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-3 h-3"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="3"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              </button>
            </td>
            <!-- 序号 -->
            <td class="px-4 py-3 text-text-muted font-mono text-xs">
              {{ (page - 1) * pageSize + idx + 1 }}
            </td>
            <!-- 分组 -->
            <td class="px-4 py-3 text-text-secondary text-xs">
              {{ row.group || "-" }}
            </td>
            <!-- IP -->
            <td class="px-4 py-3">
              <span class="font-mono text-accent font-medium">{{
                row.ip
              }}</span>
            </td>
            <!-- 协议 -->
            <td class="px-4 py-3">
              <span
                class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                :class="getProtocolBadgeClass(row.protocol)"
              >
                {{ row.protocol }}
              </span>
            </td>
            <!-- 端口 -->
            <td class="px-4 py-3 text-text-secondary font-mono">
              {{ row.port }}
            </td>
            <!-- 用户名 -->
            <td class="px-4 py-3 text-text-secondary">
              {{ row.username || "-" }}
            </td>
            <!-- 密码 -->
            <td class="px-4 py-3">
              <span class="font-mono text-text-muted tracking-widest text-xs"
                >-</span
              >
            </td>
            <!-- 标签 -->
            <td class="px-4 py-3 text-text-secondary">
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="tag in row.tags"
                  :key="tag"
                  class="px-1.5 py-0.5 text-[10px] rounded bg-accent/10 text-accent border border-accent/20"
                >
                  {{ tag }}
                </span>
                <span v-if="row.tags.length === 0" class="text-text-muted/50"
                  >-</span
                >
              </div>
            </td>
            <!-- 操作 -->
            <td class="px-4 py-3">
              <div class="flex items-center justify-center gap-2">
                <button
                  @click="$emit('edit', row)"
                  class="p-1.5 text-text-muted hover:text-accent hover:bg-accent/10 rounded transition-all duration-200"
                  title="编辑"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-4 h-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"
                    />
                    <path
                      d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"
                    />
                  </svg>
                </button>
                <button
                  @click="$emit('delete', row)"
                  class="p-1.5 text-text-muted hover:text-error hover:bg-error-bg rounded transition-all duration-200"
                  title="删除"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="w-4 h-4"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <polyline points="3,6 5,6 21,6" />
                    <path
                      d="M19,6v14a2,2,0,0,1-2,2H7a2,2,0,0,1-2-2V6m3,0V4a2,2,0,0,1,2-2h4a2,2,0,0,1,2,2v2"
                    />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 选中提示 -->
    <div
      v-if="selectedCount > 0"
      class="flex items-center justify-between px-5 py-2.5 bg-accent/5 border-t border-accent/20"
    >
      <div class="flex items-center gap-2">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-4 h-4 text-accent"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <polyline points="9 11 12 14 22 4" />
          <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
        </svg>
        <span class="text-sm text-accent font-medium">
          已选中 <strong>{{ selectedCount }}</strong> 台设备
        </span>
        <button
          @click="$emit('clear-selection')"
          class="ml-2 px-2 py-0.5 text-xs text-text-muted hover:text-text-primary hover:bg-bg-hover rounded transition-all"
        >
          清空选择
        </button>
      </div>
      <button
        @click="$emit('batch-delete')"
        class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-error hover:bg-error/90 rounded-lg transition-all duration-200"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-3.5 h-3.5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <polyline points="3,6 5,6 21,6" />
          <path
            d="M19,6v14a2,2,0,0,1-2,2H7a2,2,0,0,1-2-2V6m3,0V4a2,2,0,0,1,2-2h4a2,2,0,0,1,2,2v2"
          />
        </svg>
        批量删除
      </button>
    </div>

    <!-- 分页 -->
    <div
      class="flex items-center justify-between px-5 py-3.5 border-t border-border bg-bg-panel"
    >
      <div class="flex items-center gap-6">
        <span class="text-xs text-text-muted">
          第 {{ page }} / {{ totalPages }} 页，共 {{ total }} 条
        </span>
        <!-- 页面跳转 -->
        <div class="flex items-center gap-2 border-l border-border pl-6">
          <span class="text-xs text-text-muted">前往</span>
          <input
            :value="jumpPageInput"
            type="text"
            class="w-12 h-7 text-xs text-center bg-bg-panel border border-border rounded focus:border-accent focus:outline-none transition-colors font-mono"
            placeholder="页码"
            @input="
              $emit(
                'update:jumpPageInput',
                ($event.target as HTMLInputElement).value,
              )
            "
            @keyup.enter="$emit('jump-page')"
          />
          <button
            @click="$emit('jump-page')"
            class="px-2 h-7 text-xs text-accent hover:bg-accent/10 rounded transition-colors font-medium"
          >
            跳转
          </button>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button
          @click="$emit('prev-page')"
          :disabled="page === 1 || loading"
          class="px-3 py-1.5 text-xs rounded-lg border border-border text-text-secondary hover:border-accent/50 hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed transition-all duration-200"
        >
          上一页
        </button>
        <button
          @click="$emit('next-page')"
          :disabled="page === totalPages || loading"
          class="px-3 py-1.5 text-xs rounded-lg border border-border text-text-secondary hover:border-accent/50 hover:text-accent disabled:opacity-30 disabled:cursor-not-allowed transition-all duration-200"
        >
          下一页
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { DeviceAsset } from "@/services/api";

// Props
interface Props {
  devices: DeviceAsset[];
  loading: boolean;
  selectedIds: Set<number>;
  isSelectingAll: boolean;
  isAllSelected: boolean;
  isIndeterminate: boolean;
  selectedCount: number;
  page: number;
  totalPages: number;
  total: number;
  pageSize: number;
  jumpPageInput: string;
}

const props = defineProps<Props>();

// Emits
interface Emits {
  (e: "toggle-select", id: number): void;
  (e: "toggle-select-all"): void;
  (e: "clear-selection"): void;
  (e: "edit", device: DeviceAsset): void;
  (e: "delete", device: DeviceAsset): void;
  (e: "batch-edit", field: string): void;
  (e: "batch-delete"): void;
  (e: "prev-page"): void;
  (e: "next-page"): void;
  (e: "jump-page"): void;
  (e: "update:jumpPageInput", value: string): void;
}

defineEmits<Emits>();

// 检查是否选中
function isSelected(id: number): boolean {
  return props.selectedIds.has(id);
}

// 获取协议徽章样式
function getProtocolBadgeClass(protocol: string): string {
  const classes: Record<string, string> = {
    SSH: "bg-success-bg text-success",
    SNMP: "bg-info-bg text-info",
    TELNET: "bg-warning-bg text-warning",
  };
  return classes[protocol] || "bg-bg-hover text-text-muted";
}
</script>

<script lang="ts">
// 批量编辑按钮子组件
import { defineComponent, h } from "vue";

const BatchEditButton = defineComponent({
  props: {
    disabled: { type: Boolean, default: false },
    field: { type: String, required: true },
  },
  emits: ["click"],
  setup(props, { emit }) {
    return () =>
      h(
        "button",
        {
          onClick: () => emit("click"),
          disabled: props.disabled,
          class: [
            "p-0.5 transition-colors disabled:opacity-40 disabled:cursor-not-allowed",
            props.disabled
              ? "text-text-muted/60"
              : "text-text-muted hover:text-accent",
          ],
          title: props.disabled
            ? `请先勾选设备后再批量修改${props.field}`
            : `批量修改${props.field}`,
        },
        [
          h(
            "svg",
            {
              xmlns: "http://www.w3.org/2000/svg",
              class: "w-3.5 h-3.5",
              viewBox: "0 0 24 24",
              fill: "none",
              stroke: "currentColor",
              "stroke-width": "2",
            },
            [
              h("path", {
                d: "M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7",
              }),
              h("path", {
                d: "M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z",
              }),
            ],
          ),
        ],
      );
  },
});

export default {
  components: { BatchEditButton },
};
</script>
