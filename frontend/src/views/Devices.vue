<template>
  <div class="animate-slide-in space-y-6">
    <!-- 搜索栏 -->
    <DeviceSearchBar
      v-model:search-type="searchType"
      v-model:search-query="searchQuery"
      :search-options="searchOptions"
      @reset="resetSearch"
      @add="openAddModal"
    />

    <!-- 页面提示 -->
    <div
      v-if="pageNotice"
      class="px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg"
    >
      {{ pageNotice }}
    </div>

    <!-- 数据表格 -->
    <DeviceTable
      :devices="data"
      :loading="loading"
      :selected-ids="selectedIds"
      :is-selecting-all="isSelectingAll"
      :is-all-selected="isAllSelected"
      :is-indeterminate="isIndeterminate"
      :selected-count="selectedCount"
      :page="page"
      :total-pages="totalPages"
      :total="total"
      :page-size="pageSize"
      :jump-page-input="jumpPageInput"
      @toggle-select="toggleSelect"
      @toggle-select-all="handleToggleSelectAll(data.map((d) => d.id))"
      @clear-selection="clearSelection"
      @edit="openEditModal"
      @delete="openDeleteConfirm"
      @batch-edit="openBatchEditModal"
      @batch-delete="openBatchDeleteConfirm"
      @prev-page="handlePrevPage"
      @next-page="handleNextPage"
      @jump-page="jumpToPage"
      @update:jumpPageInput="jumpPageInput = $event"
    />

    <!-- 新增/编辑设备弹窗 -->
    <DeviceEditModal
      ref="editModalRef"
      :show="showModal"
      :is-editing="isEditing"
      :form-data="formData"
      :valid-protocols="validProtocols"
      :protocol-default-ports="protocolDefaultPorts"
      @close="closeModal"
      @save="saveDevice"
      @reset-ssh-host-key="resetSSHHostKey"
    />

    <!-- 批量编辑弹窗 -->
    <DeviceBatchEditModal
      ref="batchEditModalRef"
      :show="showBatchModal"
      :field="batchField"
      :selected-count="selectedCount"
      :valid-protocols="validProtocols"
      @close="closeBatchModal"
      @save="saveBatchEdit"
    />

    <!-- 删除确认弹窗 -->
    <DeviceDeleteConfirm
      ref="deleteConfirmRef"
      :show="showDeleteConfirm"
      :is-batch="false"
      :device="deviceToDelete"
      @close="showDeleteConfirm = false"
      @confirm="deleteDevice"
    />

    <!-- 批量删除确认弹窗 -->
    <DeviceDeleteConfirm
      ref="batchDeleteConfirmRef"
      :show="showBatchDeleteConfirm"
      :is-batch="true"
      :selected-count="selectedCount"
      @close="showBatchDeleteConfirm = false"
      @confirm="batchDeleteDevices"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { QueryAPI, DeviceAPI } from "@/services/api";
import type { DeviceAsset } from "@/services/api";

// 组件导入
import DeviceSearchBar from "@/components/device/DeviceSearchBar.vue";
import DeviceTable from "@/components/device/DeviceTable.vue";
import DeviceEditModal from "@/components/device/DeviceEditModal.vue";
import DeviceBatchEditModal from "@/components/device/DeviceBatchEditModal.vue";
import DeviceDeleteConfirm from "@/components/device/DeviceDeleteConfirm.vue";

// Composables 导入
import { useDeviceSearch } from "@/composables/useDeviceSearch";
import { useDeviceSelection } from "@/composables/useDeviceSelection";
import type { DeviceFormData } from "@/composables/useDeviceForm";

// 类型定义
type Device = DeviceAsset;
type BatchField =
  | "group"
  | "protocol"
  | "port"
  | "username"
  | "password"
  | "tag";

// ==================== Composables ====================

const {
  searchQuery,
  searchType,
  searchOptions,
  resetSearch: resetSearchState,
} = useDeviceSearch();

const {
  selectedIds,
  isSelectingAll,
  selectedCount,
  isAllSelected,
  isIndeterminate,
  toggleSelect,
  clearSelection,
} = useDeviceSelection();

// ==================== 数据状态 ====================

const data = ref<Device[]>([]);
const total = ref(0);
const totalPages = ref(1);
const page = ref(1);
const pageSize = 10;
const jumpPageInput = ref("");
const loading = ref(false);

// ==================== 弹窗状态 ====================

const showModal = ref(false);
const isEditing = ref(false);
const editingDeviceId = ref<number | null>(null);
const formData = ref<DeviceFormData | undefined>(undefined);

const showDeleteConfirm = ref(false);
const deviceToDelete = ref<Device | null>(null);

const showBatchDeleteConfirm = ref(false);
const showBatchModal = ref(false);
const batchField = ref<BatchField | null>(null);

// ==================== 协议配置 ====================

const protocolDefaultPorts = ref<Record<string, number>>({
  SSH: 22,
  SNMP: 161,
  TELNET: 23,
});
const validProtocols = ref<string[]>(["SSH", "SNMP", "TELNET"]);

// ==================== 页面提示 ====================

const pageNotice = ref("");
let pageNoticeTimer: ReturnType<typeof setTimeout> | null = null;

function showPageNotice(message: string) {
  pageNotice.value = message;
  if (pageNoticeTimer) {
    clearTimeout(pageNoticeTimer);
  }
  pageNoticeTimer = setTimeout(() => {
    pageNotice.value = "";
  }, 5000);
}

// ==================== 组件引用 ====================

const editModalRef = ref<InstanceType<typeof DeviceEditModal> | null>(null);
const batchEditModalRef = ref<InstanceType<typeof DeviceBatchEditModal> | null>(
  null,
);
const deleteConfirmRef = ref<InstanceType<typeof DeviceDeleteConfirm> | null>(
  null,
);
const batchDeleteConfirmRef = ref<InstanceType<
  typeof DeviceDeleteConfirm
> | null>(null);

// ==================== 数据加载 ====================

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

async function loadDevices() {
  loading.value = true;
  try {
    const result = await QueryAPI.listDevices({
      searchQuery: searchQuery.value,
      filterField: searchType.value,
      filterValue: "",
      page: page.value,
      pageSize: pageSize,
      sortBy: "ip",
      sortOrder: "asc",
    });

    data.value = result?.data || [];
    total.value = result?.total || 0;
    totalPages.value = result?.totalPages || 1;
  } catch (err) {
    console.error("加载设备列表失败:", err);
    data.value = [];
    total.value = 0;
    totalPages.value = 1;
  } finally {
    loading.value = false;
  }
}

async function loadProtocolConfig() {
  try {
    const ports = await DeviceAPI.getProtocolDefaultPorts();
    const protocols = await DeviceAPI.getValidProtocols();
    if (ports) {
      const normalized: Record<string, number> = {};
      Object.entries(ports).forEach(([key, value]) => {
        if (typeof value === "number") {
          normalized[key] = value;
        }
      });
      protocolDefaultPorts.value = normalized;
    }
    if (protocols) validProtocols.value = protocols;
  } catch (e) {
    console.error("Failed to load protocol config", e);
  }
}

// ==================== 搜索处理 ====================

function resetSearch() {
  resetSearchState();
  page.value = 1;
  loadDevices();
}

// ==================== 分页处理 ====================

function handlePrevPage() {
  if (page.value > 1) {
    page.value--;
  }
}

function handleNextPage() {
  if (page.value < totalPages.value) {
    page.value++;
  }
}

function jumpToPage() {
  const target = parseInt(jumpPageInput.value);
  if (isNaN(target)) {
    jumpPageInput.value = "";
    return;
  }

  if (target >= 1 && target <= totalPages.value) {
    page.value = target;
  } else if (target < 1) {
    page.value = 1;
  } else if (target > totalPages.value) {
    page.value = totalPages.value;
  }
  jumpPageInput.value = "";
}

// ==================== 全选处理 ====================

function handleToggleSelectAll(allIds: number[]) {
  // 检查当前页是否全部选中
  const currentPageSelected = allIds.every((id) => selectedIds.value.has(id));

  if (currentPageSelected) {
    // 如果当前页全选，则取消当前页的选择
    const newSet = new Set(selectedIds.value);
    allIds.forEach((id) => newSet.delete(id));
    selectedIds.value = newSet;
  } else {
    // 否则选中当前页所有设备
    const newSet = new Set(selectedIds.value);
    allIds.forEach((id) => newSet.add(id));
    selectedIds.value = newSet;
  }
}

// ==================== 弹窗操作 ====================

function openAddModal() {
  isEditing.value = false;
  editingDeviceId.value = null;
  formData.value = undefined;
  showModal.value = true;
}

async function openEditModal(device: Device) {
  isEditing.value = true;
  editingDeviceId.value = device.id;
  showModal.value = true;

  try {
    const fullDevice = await DeviceAPI.getDeviceById(device.id);
    if (!fullDevice) {
      editModalRef.value?.setError("获取设备详情失败：设备不存在");
      return;
    }
    formData.value = {
      ip: fullDevice.ip,
      port: fullDevice.port,
      protocol: fullDevice.protocol,
      username: fullDevice.username,
      password: fullDevice.password || "",
      group: fullDevice.group,
      tags: [...fullDevice.tags],
      vendor: fullDevice.vendor || "",
      role: fullDevice.role || "",
      site: fullDevice.site || "",
      displayName: fullDevice.displayName || "",
      description: fullDevice.description || "",
    };
  } catch (err) {
    console.error("获取设备详情失败:", err);
    editModalRef.value?.setError("获取设备详情失败，请重试");
    showModal.value = false;
  }
}

function closeModal() {
  showModal.value = false;
  // 显式清空密码，确保内存中不留痕迹
  if (formData.value?.password) {
    formData.value.password = "";
  }
  formData.value = undefined;
}

// 监听弹窗关闭事件，确保密码清理
watch(showModal, (newVal) => {
  if (!newVal && formData.value?.password) {
    // 弹窗关闭时清理密码
    formData.value.password = "";
  }
});

async function saveDevice(deviceData: DeviceFormData) {
  editModalRef.value?.setSaving(true);
  editModalRef.value?.setError("");

  try {
    if (isEditing.value && editingDeviceId.value) {
      // 编辑模式 - 包含密码字段
      await DeviceAPI.updateDevice(editingDeviceId.value, {
        ip: deviceData.ip,
        port: deviceData.port,
        protocol: deviceData.protocol,
        username: deviceData.username,
        password: deviceData.password, // 传递密码字段
        group: deviceData.group,
        tags: deviceData.tags,
        vendor: deviceData.vendor,
        role: deviceData.role,
        site: deviceData.site,
        displayName: deviceData.displayName,
        description: deviceData.description,
      } as unknown as DeviceAsset);
      showPageNotice("设备更新成功");
    } else {
      // 新增模式 - 包含密码字段
      await DeviceAPI.addDevices([
        {
          ip: deviceData.ip,
          port: deviceData.port,
          protocol: deviceData.protocol,
          username: deviceData.username,
          password: deviceData.password, // 传递密码字段
          group: deviceData.group,
          tags: deviceData.tags,
          vendor: deviceData.vendor,
          role: deviceData.role,
          site: deviceData.site,
          displayName: deviceData.displayName,
          description: deviceData.description,
        } as unknown as DeviceAsset,
      ]);
      showPageNotice("设备添加成功");
    }

    closeModal();
    loadDevices();
  } catch (err: unknown) {
    console.error("保存设备失败:", err);
    const message =
      (err as { message?: string })?.message || "保存失败，请重试";
    editModalRef.value?.setError(message);
  } finally {
    editModalRef.value?.setSaving(false);
  }
}

async function resetSSHHostKey() {
  if (!isEditing.value || !editingDeviceId.value || !formData.value?.ip) {
    editModalRef.value?.setError("仅编辑已有设备时支持重置主机 SSH 密钥");
    return;
  }

  const confirmed = confirm(
    `确定要清除设备 ${formData.value.ip} 的 SSH 主机密钥记录吗？\n清除后，下次连接会重新学习该主机密钥。`,
  );
  if (!confirmed) {
    return;
  }

  editModalRef.value?.setSaving(true);
  editModalRef.value?.setError("");
  try {
    await DeviceAPI.resetDeviceSSHHostKey(editingDeviceId.value);
    showPageNotice(`设备 ${formData.value.ip} 的 SSH 主机密钥已重置`);
  } catch (err: unknown) {
    console.error("重置 SSH 主机密钥失败:", err);
    const message =
      (err as { message?: string })?.message || "重置 SSH 主机密钥失败，请重试";
    editModalRef.value?.setError(message);
  } finally {
    editModalRef.value?.setSaving(false);
  }
}

// ==================== 删除操作 ====================

function openDeleteConfirm(device: Device) {
  deviceToDelete.value = device;
  showDeleteConfirm.value = true;
}

async function deleteDevice() {
  if (!deviceToDelete.value) return;

  deleteConfirmRef.value?.setDeleting(true);

  try {
    await DeviceAPI.deleteDevice(deviceToDelete.value.id);
    showPageNotice("设备删除成功");
    showDeleteConfirm.value = false;
    deviceToDelete.value = null;
    loadDevices();
  } catch (err: unknown) {
    console.error("删除设备失败:", err);
    const message =
      (err as { message?: string })?.message || "删除失败，请重试";
    showPageNotice(message);
  } finally {
    deleteConfirmRef.value?.setDeleting(false);
  }
}

// ==================== 批量操作 ====================

function openBatchEditModal(field: string) {
  batchField.value = field as BatchField;
  showBatchModal.value = true;
}

function closeBatchModal() {
  showBatchModal.value = false;
  batchField.value = null;
}

async function saveBatchEdit(field: BatchField, value: string | number) {
  if (selectedCount.value === 0) return;

  batchEditModalRef.value?.setSaving(true);
  batchEditModalRef.value?.setError("");

  try {
    const ids = Array.from(selectedIds.value);

    if (field === "tag") {
      // 标签特殊处理：解析逗号分隔的标签
      const tags = (value as string)
        .split(/[,，]/)
        .map((t) => t.trim())
        .filter((t) => t);

      // 字段级更新：只传递 tags 字段，不展开整个设备对象
      for (const id of ids) {
        await DeviceAPI.updateDevice(id, {
          tags,
        } as unknown as DeviceAsset);
      }
    } else {
      // 其他字段批量更新 - 字段级更新，不展开整个设备对象
      for (const id of ids) {
        await DeviceAPI.updateDevice(id, {
          [field]: value,
        } as unknown as DeviceAsset);
      }
    }

    showPageNotice(
      `成功修改 ${ids.length} 台设备的${field === "tag" ? "标签" : field}`,
    );
    closeBatchModal();
    clearSelection();
    loadDevices();
  } catch (err: unknown) {
    console.error("批量修改失败:", err);
    const message =
      (err as { message?: string })?.message || "批量修改失败，请重试";
    batchEditModalRef.value?.setError(message);
  } finally {
    batchEditModalRef.value?.setSaving(false);
  }
}

function openBatchDeleteConfirm() {
  showBatchDeleteConfirm.value = true;
}

async function batchDeleteDevices() {
  if (selectedCount.value === 0) return;

  batchDeleteConfirmRef.value?.setDeleting(true);

  try {
    const ids = Array.from(selectedIds.value);
    await DeviceAPI.deleteDevices(ids);
    showPageNotice(`成功删除 ${ids.length} 台设备`);
    showBatchDeleteConfirm.value = false;
    clearSelection();
    loadDevices();
  } catch (err: unknown) {
    console.error("批量删除失败:", err);
    const message =
      (err as { message?: string })?.message || "批量删除失败，请重试";
    showPageNotice(message);
  } finally {
    batchDeleteConfirmRef.value?.setDeleting(false);
  }
}

// ==================== 监听器 ====================

// 监听搜索输入，实现防抖并触发后端查询
watch(searchQuery, () => {
  if (searchTimeout) {
    clearTimeout(searchTimeout);
  }
  searchTimeout = setTimeout(() => {
    page.value = 1;
    loadDevices();
  }, 300);
});

// 监听搜索类型变化，重新查询
watch(searchType, () => {
  page.value = 1;
  loadDevices();
});

// 监听页码变化，触发后端查询
watch(page, () => {
  loadDevices();
});

// ==================== 生命周期 ====================

onMounted(() => {
  loadDevices();
  loadProtocolConfig();
});
</script>
