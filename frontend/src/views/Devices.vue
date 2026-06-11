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



    <!-- 数据表格 -->
    <DeviceTable
      :devices="data"
      :loading="loading"
      :selected-ids="selectedIds"
      :selected-count="selectedCount"
      :page="page"
      :total="total"
      :page-size="pageSize"
      @update-selection="handleUpdateSelection"
      @edit="openEditModal"
      @delete="openDeleteConfirm"
      @batch-edit="openBatchEditModal"
      @batch-reset-ssh-key="openBatchResetSSHKeyConfirm"
      @batch-delete="openBatchDeleteConfirm"
      @page-change="page = $event"
      @size-change="handleSizeChange"
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { QueryAPI, DeviceAPI } from "@/services/api";
import { DeviceAsset } from "@/bindings/github.com/NetWeaverGo/core/internal/models/models";
import { getLogger } from "@/utils/logger";
import { parseErrorMessage } from "@/utils/errorHandler";

// 组件导入
import DeviceSearchBar from "@/components/device/DeviceSearchBar.vue";
import DeviceTable from "@/components/device/DeviceTable.vue";
import DeviceEditModal from "@/components/device/DeviceEditModal.vue";
import DeviceBatchEditModal from "@/components/device/DeviceBatchEditModal.vue";

// Composables 导入
import { useDeviceSearch } from "@/composables/useDeviceSearch";
import { useDeviceSelection } from "@/composables/useDeviceSelection";
import type { DeviceFormData } from "@/composables/useDeviceForm";

const logger = getLogger()

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
  selectedCount,
  clearSelection,
} = useDeviceSelection();

function handleUpdateSelection(ids: number[]) {
  const newSet = new Set<number>();
  ids.forEach(id => newSet.add(id));
  selectedIds.value = newSet;
}

// ==================== 数据状态 ====================

const data = ref<Device[]>([]);
const total = ref(0);
const totalPages = ref(1);
const page = ref(1);
const pageSize = ref(10);
const loading = ref(false);

function handleSizeChange(newSize: number) {
  pageSize.value = newSize;
  page.value = 1;
  loadDevices();
}

// ==================== 弹窗状态 ====================

const showModal = ref(false);
const isEditing = ref(false);
const editingDeviceId = ref<number | null>(null);
const formData = ref<DeviceFormData | undefined>(undefined);

const showBatchModal = ref(false);
const batchField = ref<BatchField | null>(null);

// ==================== 协议配置 ====================

const protocolDefaultPorts = ref<Record<string, number>>({
  SSH: 22,
  SNMP: 161,
  TELNET: 23,
});
const validProtocols = ref<string[]>(["SSH", "SNMP", "TELNET"]);



// ==================== 组件引用 ====================

const editModalRef = ref<InstanceType<typeof DeviceEditModal> | null>(null);
const batchEditModalRef = ref<InstanceType<typeof DeviceBatchEditModal> | null>(
  null,
);

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
      pageSize: pageSize.value,
      sortBy: "ip",
      sortOrder: "asc",
    });

    data.value = result?.data || [];
    total.value = result?.total || 0;
    totalPages.value = result?.totalPages || 1;
  } catch (err) {
    logger.error("加载设备列表失败", 'Devices', err);
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
    logger.error("Failed to load protocol config", 'Devices', e);
  }
}

// ==================== 搜索处理 ====================

function resetSearch() {
  resetSearchState();
  page.value = 1;
  loadDevices();
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
    logger.error("获取设备详情失败", 'Devices', err);
    editModalRef.value?.setError("获取设备详情失败，请重试");
    showModal.value = false;
  }
}

function closeModal() {
  showModal.value = false;
  if (formData.value?.password) {
    formData.value.password = "";
  }
  formData.value = undefined;
}

watch(showModal, (newVal) => {
  if (!newVal && formData.value?.password) {
    formData.value.password = "";
  }
});

async function saveDevice(deviceData: DeviceFormData) {
  editModalRef.value?.setSaving(true);
  editModalRef.value?.setError("");

  const payload = new DeviceAsset({
    ip: deviceData.ip,
    port: deviceData.port,
    protocol: deviceData.protocol,
    username: deviceData.username,
    password: deviceData.password,
    group: deviceData.group,
    tags: deviceData.tags,
    vendor: deviceData.vendor,
    role: deviceData.role,
    site: deviceData.site,
    displayName: deviceData.displayName,
    description: deviceData.description,
  });

  try {
    if (isEditing.value && editingDeviceId.value) {
      await DeviceAPI.updateDevice(editingDeviceId.value, payload);
      ElMessage.success("设备更新成功");
    } else {
      await DeviceAPI.addDevices([payload]);
      ElMessage.success("设备添加成功");
    }

    closeModal();
    loadDevices();
  } catch (err: unknown) {
    logger.error("保存设备失败", 'Devices', err);
    const message = parseErrorMessage(err);
    editModalRef.value?.setError(message || "保存失败，请重试");
  } finally {
    editModalRef.value?.setSaving(false);
  }
}

async function resetSSHHostKey() {
  if (!isEditing.value || !editingDeviceId.value || !formData.value?.ip) {
    editModalRef.value?.setError("仅编辑已有设备时支持重置主机 SSH 密钥");
    return;
  }

  ElMessageBox.confirm(
    `确定要清除设备 ${formData.value.ip} 的 SSH 主机密钥记录吗？\n清除后，下次连接会重新学习该主机密钥。`,
    '重置主机SSH密钥',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(async () => {
    editModalRef.value?.setSaving(true);
    editModalRef.value?.setError("");
    try {
      // @ts-ignore - editingDeviceId.value is checked above
      await DeviceAPI.resetDeviceSSHHostKey(editingDeviceId.value);
      ElMessage.success(`设备 ${formData.value?.ip} 的 SSH 主机密钥已重置`);
    } catch (err: unknown) {
      logger.error("重置 SSH 主机密钥失败", 'Devices', err);
      const message = parseErrorMessage(err);
      editModalRef.value?.setError(message || "重置 SSH 主机密钥失败，请重试");
    } finally {
      editModalRef.value?.setSaving(false);
    }
  }).catch(() => {});
}

// ==================== 删除操作 ====================

function openDeleteConfirm(device: Device) {
  ElMessageBox.confirm(
    `确定要删除设备 ${device.ip} 吗？此操作不可撤销。`,
    '确认删除',
    {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger'
    }
  ).then(async () => {
    try {
      await DeviceAPI.deleteDevice(device.id);
      ElMessage.success("设备删除成功");
      loadDevices();
    } catch (err: any) {
      logger.error("删除设备失败", 'Devices', err);
      ElMessage.error(parseErrorMessage(err) || "删除失败");
    }
  }).catch(() => {});
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

    const fieldPayload: Record<string, string | number | string[]> = {};
    if (field === "tag") {
      fieldPayload.tags = (value as string)
        .split(/[,，]/)
        .map((t) => t.trim())
        .filter((t) => t);
    } else {
      fieldPayload[field] = value;
    }

    const updates = ids.map(
      (id) => new DeviceAsset({ id, ...fieldPayload }),
    );
    await DeviceAPI.updateDevices(updates);

    ElMessage.success(
      `成功修改 ${ids.length} 台设备的${field === "tag" ? "标签" : field}`,
    );
    closeBatchModal();
    clearSelection();
    loadDevices();
  } catch (err: unknown) {
    logger.error("批量修改失败", 'Devices', err);
    const message = parseErrorMessage(err);
    batchEditModalRef.value?.setError(message || "批量修改失败，请重试");
  } finally {
    batchEditModalRef.value?.setSaving(false);
  }
}

function openBatchResetSSHKeyConfirm() {
  if (selectedCount.value === 0) return;
  ElMessageBox.confirm(
    `确定要清除选中的 ${selectedCount.value} 台设备的 SSH 主机密钥记录吗？\n清除后，下次连接会重新学习这些主机的密钥。`,
    '批量重置主机SSH密钥确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(async () => {
    try {
      const ids = Array.from(selectedIds.value);
      for (const id of ids) {
        await DeviceAPI.resetDeviceSSHHostKey(id);
      }
      ElMessage.success(`成功重置 ${ids.length} 台设备的主机SSH密钥`);
      clearSelection();
    } catch (err: any) {
      logger.error("批量重置主机SSH密钥失败", 'Devices', err);
      ElMessage.error(parseErrorMessage(err) || "批量重置主机SSH密钥失败");
    }
  }).catch(() => {});
}

function openBatchDeleteConfirm() {
  if (selectedCount.value === 0) return;
  ElMessageBox.confirm(
    `确定要删除选中的 ${selectedCount.value} 台设备吗？此操作不可撤销。`,
    '批量删除确认',
    {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger'
    }
  ).then(async () => {
    try {
      const ids = Array.from(selectedIds.value);
      await DeviceAPI.deleteDevices(ids);
      ElMessage.success(`成功删除 ${ids.length} 台设备`);
      clearSelection();
      loadDevices();
    } catch (err: any) {
      logger.error("批量删除失败", 'Devices', err);
      ElMessage.error(parseErrorMessage(err) || "批量删除失败");
    }
  }).catch(() => {});
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
