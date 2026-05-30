<template>
  <div class="bg-bg-panel border border-border rounded-xl shadow-card overflow-hidden flex flex-col">
    <!-- 选中提示与批量操作 -->
    <div
      v-if="selectedCount > 0"
      class="flex items-center justify-between px-5 py-2.5 bg-accent-bg border-b border-accent/20"
    >
      <div class="flex items-center gap-2">
        <el-icon class="text-accent"><Select /></el-icon>
        <span class="text-sm text-accent font-medium">
          已选中 <strong>{{ selectedCount }}</strong> 台设备
        </span>
        <el-button link type="primary" size="small" @click="clearSelection">
          清空选择
        </el-button>
      </div>
      <el-button
        type="danger"
        size="small"
        @click="$emit('batch-delete')"
      >
        批量删除
      </el-button>
    </div>

    <!-- 表格区域 -->
    <el-table
      ref="tableRef"
      :data="devices"
      v-loading="loading"
      row-key="id"
      class="w-full flex-1"
      height="calc(100vh - 280px)"
      @selection-change="handleSelectionChange"
    >
      <el-table-column type="selection" width="50" reserve-selection />
      
      <el-table-column label="#" width="60">
        <template #default="scope">
          <span class="text-text-muted font-mono text-xs">
            {{ (page - 1) * pageSize + scope.$index + 1 }}
          </span>
        </template>
      </el-table-column>
      
      <el-table-column label="分组" min-width="120">
        <template #header>
          <div class="flex items-center gap-1">
            分组
            <BatchEditButton :disabled="selectedCount === 0" @click="$emit('batch-edit', 'group')" />
          </div>
        </template>
        <template #default="{ row }">
          <span class="text-text-secondary text-xs">{{ row.group || "-" }}</span>
        </template>
      </el-table-column>
      
      <el-table-column label="IP 地址" prop="ip" min-width="140">
        <template #default="{ row }">
          <span class="font-mono text-accent font-medium">{{ row.ip }}</span>
        </template>
      </el-table-column>
      
      <el-table-column label="协议" width="100">
        <template #header>
          <div class="flex items-center gap-1">
            协议
            <BatchEditButton :disabled="selectedCount === 0" @click="$emit('batch-edit', 'protocol')" />
          </div>
        </template>
        <template #default="{ row }">
          <el-tag size="small" :type="getProtocolTagType(row.protocol)" effect="light">
            {{ row.protocol }}
          </el-tag>
        </template>
      </el-table-column>
      
      <el-table-column label="端口" width="100">
        <template #header>
          <div class="flex items-center gap-1">
            端口
            <BatchEditButton :disabled="selectedCount === 0" @click="$emit('batch-edit', 'port')" />
          </div>
        </template>
        <template #default="{ row }">
          <span class="text-text-secondary font-mono">{{ row.port }}</span>
        </template>
      </el-table-column>
      
      <el-table-column label="用户名" min-width="120">
        <template #header>
          <div class="flex items-center gap-1">
            用户名
            <BatchEditButton :disabled="selectedCount === 0" @click="$emit('batch-edit', 'username')" />
          </div>
        </template>
        <template #default="{ row }">
          <span class="text-text-secondary">{{ row.username || "-" }}</span>
        </template>
      </el-table-column>
      
      <el-table-column label="密码" width="100">
        <template #header>
          <div class="flex items-center gap-1">
            密码
            <BatchEditButton :disabled="selectedCount === 0" @click="$emit('batch-edit', 'password')" />
          </div>
        </template>
        <template #default>
          <span class="font-mono text-text-muted tracking-widest text-xs">-</span>
        </template>
      </el-table-column>
      
      <el-table-column label="Tag" min-width="150">
        <template #header>
          <div class="flex items-center gap-1">
            Tag
            <BatchEditButton :disabled="selectedCount === 0" @click="$emit('batch-edit', 'tag')" />
          </div>
        </template>
        <template #default="{ row }">
          <div class="flex flex-wrap gap-1">
            <el-tag
              v-for="tag in row.tags"
              :key="tag"
              size="small"
              type="info"
              effect="plain"
            >
              {{ tag }}
            </el-tag>
            <span v-if="!row.tags || row.tags.length === 0" class="text-text-muted/50">-</span>
          </div>
        </template>
      </el-table-column>
      
      <el-table-column label="操作" width="120" align="center" fixed="right">
        <template #default="{ row }">
          <div class="flex items-center justify-center gap-2">
            <el-button link type="primary" :icon="Edit" @click="$emit('edit', row)" title="编辑" />
            <el-button link type="danger" :icon="Delete" @click="$emit('delete', row)" title="删除" />
          </div>
        </template>
      </el-table-column>
      
      <template #empty>
        <el-empty description="暂无设备数据" />
      </template>
    </el-table>

    <!-- 分页 -->
    <div class="flex items-center justify-between px-5 py-3 border-t border-border bg-bg-panel">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="currentPageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Edit, Delete, Select } from '@element-plus/icons-vue'
import type { DeviceAsset } from "@/services/api"
import type { TableInstance } from 'element-plus'

interface Props {
  devices: DeviceAsset[];
  loading: boolean;
  selectedIds: Set<number>;
  selectedCount: number;
  page: number;
  total: number;
  pageSize: number;
}

const props = defineProps<Props>()

interface Emits {
  (e: "update-selection", ids: number[]): void;
  (e: "edit", device: DeviceAsset): void;
  (e: "delete", device: DeviceAsset): void;
  (e: "batch-edit", field: string): void;
  (e: "batch-delete"): void;
  (e: "page-change", page: number): void;
  (e: "size-change", size: number): void;
}

const emit = defineEmits<Emits>()

const tableRef = ref<TableInstance>()
const currentPage = ref(props.page)
const currentPageSize = ref(props.pageSize)

watch(() => props.page, (val) => {
  currentPage.value = val
})

watch(() => props.pageSize, (val) => {
  currentPageSize.value = val
})

// 当外部传来 selectedIds 清空等操作时，同步给 el-table
watch(() => props.selectedIds, (newVal) => {
  if (newVal.size === 0 && tableRef.value) {
    tableRef.value.clearSelection()
  }
}, { deep: true })

function handleSelectionChange(val: DeviceAsset[]) {
  const ids = val.map(v => v.id)
  emit("update-selection", ids)
}

function clearSelection() {
  if (tableRef.value) {
    tableRef.value.clearSelection()
  }
}

function handleCurrentChange(val: number) {
  emit("page-change", val)
}

function handleSizeChange(val: number) {
  emit("size-change", val)
}

function getProtocolTagType(protocol: string) {
  const map: Record<string, string> = {
    SSH: "success",
    SNMP: "info",
    TELNET: "warning"
  }
  return map[protocol] || "info"
}
</script>

<script lang="ts">
import { defineComponent, h } from "vue";
import { ElIcon } from 'element-plus';
import { EditPen as EditPenIcon } from '@element-plus/icons-vue';

const BatchEditButton = defineComponent({
  props: {
    disabled: { type: Boolean, default: false },
  },
  emits: ["click"],
  setup(props, { emit }) {
    return () =>
      h(
        "button",
        {
          onClick: () => {
            if (!props.disabled) emit("click");
          },
          class: [
            "ml-1 p-0.5 inline-flex items-center justify-center transition-colors rounded",
            props.disabled
              ? "text-text-muted/40 cursor-not-allowed"
              : "text-text-muted hover:text-accent hover:bg-accent/10 cursor-pointer",
          ],
          title: props.disabled ? "请先勾选设备" : "批量修改",
        },
        [
          h(ElIcon, { size: 14 }, { default: () => h(EditPenIcon) })
        ]
      );
  },
});

export default {
  components: { BatchEditButton },
};
</script>
