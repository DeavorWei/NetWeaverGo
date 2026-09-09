<script setup lang="ts">
/**
 * SNMP 即时查询页面
 *
 * 面向交付现场的设备上线确认与资产信息采集：
 * - 单台设备查询：设备基本信息 / GET / WALK
 * - 批量上线确认：批量采集 sysDescr / sysUpTime / sysName
 * - 查询凭据管理：v1 / v2c / v3 凭据的增删改查
 *
 * 刻意不提供周期性轮询与 Trap 值守能力。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { getLogger } from "@/utils/logger";
import { SNMPQueryAPI } from "@/services/snmpApi";
import {
  createEmptyCredentialForm,
  createEmptyCredentialInput,
  SNMP_AUTH_PROTOCOL_OPTIONS,
  SNMP_OPERATION_OPTIONS,
  SNMP_PRIV_PROTOCOL_OPTIONS,
  SNMP_SECURITY_LEVEL_OPTIONS,
  SNMP_VERSION_OPTIONS,
} from "@/types/snmp";
import type {
  SNMPBatchResultVM,
  SNMPCredentialInputVM,
  SNMPCredentialVM,
  SNMPOperation,
  SNMPResultVM,
} from "@/types/snmp";

const logger = getLogger();

// ==================== 状态 ====================

const activeTab = ref<"query" | "batch" | "credential">("query");
const credentialLoading = ref(false);

/** 已保存凭据列表 */
const credentials = ref<SNMPCredentialVM[]>([]);

/** 单台查询 */
const queryLoading = ref(false);
const queryAddress = ref("");
const queryOperation = ref<SNMPOperation>("device_info");
const queryOID = ref("");
const queryCredentialId = ref<number | null>(null);
const useTempCredential = ref(false);
const tempCredential = ref<SNMPCredentialInputVM>(createEmptyCredentialInput());
const queryResults = ref<SNMPResultVM[]>([]);
const queryAddressDone = ref("");
const queryLatency = ref(0);

/** 批量上线确认 */
const batchLoading = ref(false);
const batchAddressInput = ref("");
const batchCredentialId = ref<number | null>(null);
const batchUseTempCredential = ref(false);
const batchTempCredential = ref<SNMPCredentialInputVM>(createEmptyCredentialInput());
const batchResults = ref<SNMPBatchResultVM[]>([]);
const batchLatency = ref(0);

/** 凭据编辑 */
const credentialDialogVisible = ref(false);
const credentialSaving = ref(false);
const credentialForm = ref<SNMPCredentialVM>(createEmptyCredentialForm());
const credentialEditing = ref(false);

/** 设备基本信息 OID 提示 */
const deviceInfoOIDs = ref<SNMPResultVM[]>([]);

// ==================== 计算属性 ====================

const isV3 = computed(() => tempCredential.value.version === "v3");
const isBatchV3 = computed(() => batchTempCredential.value.version === "v3");
const isCredentialFormV3 = computed(() => credentialForm.value.version === "v3");

const batchAddressList = computed(() =>
  batchAddressInput.value
    .split(/[,;\s]+/)
    .map((s) => s.trim())
    .filter(Boolean),
);

const batchReachableCount = computed(() => batchResults.value.filter((r) => r.reachable).length);

// ==================== 数据加载 ====================

async function loadCredentials() {
  credentialLoading.value = true;
  try {
    credentials.value = await SNMPQueryAPI.getCredentials();
  } catch (err) {
    logger.error("加载凭据列表失败", "SNMP-Query", err);
    ElMessage.error(`加载凭据列表失败: ${err}`);
  } finally {
    credentialLoading.value = false;
  }
}

async function loadDeviceInfoOIDs() {
  try {
    deviceInfoOIDs.value = await SNMPQueryAPI.getDeviceInfoOIDs();
  } catch (err) {
    logger.warn("加载设备信息 OID 失败", "SNMP-Query");
  }
}

// ==================== 查询操作 ====================

/** 获取当前生效的凭据参数 */
function resolveCredential(useTemp: boolean, credId: number | null, temp: SNMPCredentialInputVM) {
  return {
    credentialId: useTemp ? null : credId,
    tempCredential: useTemp ? temp : null,
  };
}

async function runQuery() {
  const address = queryAddress.value.trim();
  if (!address) {
    ElMessage.warning("请输入目标设备地址");
    return;
  }
  if (queryOperation.value !== "device_info" && !queryOID.value.trim()) {
    ElMessage.warning("请指定 OID");
    return;
  }

  queryLoading.value = true;
  queryResults.value = [];
  try {
    const cred = resolveCredential(useTempCredential.value, queryCredentialId.value, tempCredential.value);
    const resp = await SNMPQueryAPI.query({
      address,
      operation: queryOperation.value,
      oid: queryOID.value,
      ...cred,
    });
    queryResults.value = resp.results ?? [];
    queryAddressDone.value = resp.address;
    queryLatency.value = resp.latencyMs;
    ElMessage.success(`查询完成，返回 ${queryResults.value.length} 条结果`);
  } catch (err) {
    logger.error("查询失败", "SNMP-Query", err);
    ElMessage.error(`查询失败: ${err}`);
  } finally {
    queryLoading.value = false;
  }
}

async function runBatchQuery() {
  if (batchAddressList.value.length === 0) {
    ElMessage.warning("请输入至少一个目标地址");
    return;
  }

  batchLoading.value = true;
  batchResults.value = [];
  try {
    const cred = resolveCredential(
      batchUseTempCredential.value,
      batchCredentialId.value,
      batchTempCredential.value,
    );
    const resp = await SNMPQueryAPI.batchQueryDeviceInfo({
      addresses: batchAddressList.value,
      ...cred,
    });
    batchResults.value = resp.results ?? [];
    batchLatency.value = resp.latencyMs;
    ElMessage.success(`检测完成：${resp.reachable}/${resp.total} 台可达`);
  } catch (err) {
    logger.error("批量检测失败", "SNMP-Query", err);
    ElMessage.error(`批量检测失败: ${err}`);
  } finally {
    batchLoading.value = false;
  }
}

/** 从批量结果中提取指定 OID 的值 */
function extractValue(results: SNMPResultVM[] | undefined, name: string): string {
  if (!results) return "-";
  const hit = results.find((r) => r.oidName === name);
  if (!hit || hit.error) return "-";
  return hit.value || "-";
}

/** 截断过长的 sysDescr，便于表格展示 */
function truncate(value: string, max = 60): string {
  if (!value || value === "-") return value;
  return value.length > max ? `${value.slice(0, max)}…` : value;
}

// ==================== 凭据管理 ====================

function openCreateCredential() {
  credentialEditing.value = false;
  credentialForm.value = createEmptyCredentialForm();
  credentialDialogVisible.value = true;
}

function openEditCredential(cred: SNMPCredentialVM) {
  credentialEditing.value = true;
  credentialForm.value = { ...cred, community: "", authPassword: "", privPassword: "" };
  credentialDialogVisible.value = true;
}

async function saveCredential() {
  if (!credentialForm.value.name.trim()) {
    ElMessage.warning("请输入凭据名称");
    return;
  }

  credentialSaving.value = true;
  try {
    if (credentialEditing.value) {
      await SNMPQueryAPI.updateCredential(credentialForm.value);
      ElMessage.success("凭据已更新");
    } else {
      await SNMPQueryAPI.createCredential(credentialForm.value);
      ElMessage.success("凭据已创建");
    }
    credentialDialogVisible.value = false;
    await loadCredentials();
  } catch (err) {
    logger.error("保存凭据失败", "SNMP-Query", err);
    ElMessage.error(`保存凭据失败: ${err}`);
  } finally {
    credentialSaving.value = false;
  }
}

async function removeCredential(cred: SNMPCredentialVM) {
  try {
    await ElMessageBox.confirm(`确定删除凭据「${cred.name}」？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
  } catch {
    return;
  }

  try {
    await SNMPQueryAPI.deleteCredential(cred.id);
    ElMessage.success("凭据已删除");
    await loadCredentials();
  } catch (err) {
    logger.error("删除凭据失败", "SNMP-Query", err);
    ElMessage.error(`删除凭据失败: ${err}`);
  }
}

// ==================== 生命周期 ====================

onMounted(async () => {
  await Promise.all([loadCredentials(), loadDeviceInfoOIDs()]);
});
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- 标题栏 -->
    <div class="w-full flex items-center justify-between mb-4 shrink-0">
      <div>
        <h1 class="text-xl font-bold text-text-primary">SNMP 查询</h1>
        <p class="text-xs text-text-muted mt-1">
          交付现场的设备上线确认与资产信息采集，支持 v1 / v2c / v3
        </p>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="snmp-tabs flex-1 min-h-0 flex flex-col">
      <!-- ==================== 单台设备查询 ==================== -->
      <el-tab-pane label="设备查询" name="query">
        <div class="flex flex-col gap-4 h-full">
          <section class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label class="block text-xs text-text-secondary mb-1">目标地址</label>
                <el-input
                  v-model="queryAddress"
                  placeholder="192.168.1.1 或 192.168.1.1:161"
                  clearable
                />
              </div>

              <div>
                <label class="block text-xs text-text-secondary mb-1">查询操作</label>
                <el-select v-model="queryOperation" class="w-full">
                  <el-option
                    v-for="op in SNMP_OPERATION_OPTIONS"
                    :key="op.value"
                    :label="op.label"
                    :value="op.value"
                  />
                </el-select>
              </div>

              <div class="md:col-span-2">
                <label class="block text-xs text-text-secondary mb-1">
                  OID
                  <span v-if="queryOperation === 'device_info'" class="text-text-muted">
                    （设备信息模式使用内置 OID，无需填写）
                  </span>
                  <span v-else-if="queryOperation === 'walk'" class="text-text-muted">
                    （WALK 根 OID）
                  </span>
                  <span v-else class="text-text-muted">（多个 OID 以逗号或换行分隔）</span>
                </label>
                <el-input
                  v-model="queryOID"
                  :disabled="queryOperation === 'device_info'"
                  :placeholder="
                    queryOperation === 'walk'
                      ? '1.3.6.1.2.1.2.2'
                      : '1.3.6.1.2.1.1.1.0, 1.3.6.1.2.1.1.5.0'
                  "
                  :type="queryOperation === 'get' ? 'textarea' : 'text'"
                  :rows="2"
                  clearable
                />
              </div>

              <div class="md:col-span-2 flex items-center gap-3">
                <el-checkbox v-model="useTempCredential">使用临时凭据（不保存）</el-checkbox>
                <el-select
                  v-if="!useTempCredential"
                  v-model="queryCredentialId"
                  placeholder="选择已保存凭据（不选则使用 public）"
                  clearable
                  class="flex-1"
                  :loading="credentialLoading"
                >
                  <el-option
                    v-for="c in credentials"
                    :key="c.id"
                    :label="`${c.name} (${c.version})`"
                    :value="c.id"
                  />
                </el-select>
              </div>

              <!-- 临时凭据表单 -->
              <div
                v-if="useTempCredential"
                class="md:col-span-2 grid grid-cols-1 md:grid-cols-3 gap-3 p-3 bg-bg-tertiary/30 rounded-lg"
              >
                <div>
                  <label class="block text-xs text-text-secondary mb-1">版本</label>
                  <el-select v-model="tempCredential.version" class="w-full">
                    <el-option
                      v-for="v in SNMP_VERSION_OPTIONS"
                      :key="v.value"
                      :label="v.label"
                      :value="v.value"
                    />
                  </el-select>
                </div>
                <div v-if="!isV3">
                  <label class="block text-xs text-text-secondary mb-1">Community</label>
                  <el-input v-model="tempCredential.community" placeholder="public" />
                </div>
                <template v-if="isV3">
                  <div>
                    <label class="block text-xs text-text-secondary mb-1">用户名</label>
                    <el-input v-model="tempCredential.username" />
                  </div>
                  <div>
                    <label class="block text-xs text-text-secondary mb-1">安全级别</label>
                    <el-select v-model="tempCredential.securityLevel" class="w-full">
                      <el-option
                        v-for="s in SNMP_SECURITY_LEVEL_OPTIONS"
                        :key="s.value"
                        :label="s.label"
                        :value="s.value"
                      />
                    </el-select>
                  </div>
                  <div>
                    <label class="block text-xs text-text-secondary mb-1">认证协议</label>
                    <el-select v-model="tempCredential.authProtocol" class="w-full">
                      <el-option v-for="a in SNMP_AUTH_PROTOCOL_OPTIONS" :key="a" :label="a" :value="a" />
                    </el-select>
                  </div>
                  <div>
                    <label class="block text-xs text-text-secondary mb-1">认证密码</label>
                    <el-input v-model="tempCredential.authPassword" type="password" show-password />
                  </div>
                  <div>
                    <label class="block text-xs text-text-secondary mb-1">加密协议</label>
                    <el-select v-model="tempCredential.privProtocol" class="w-full">
                      <el-option v-for="p in SNMP_PRIV_PROTOCOL_OPTIONS" :key="p" :label="p" :value="p" />
                    </el-select>
                  </div>
                  <div>
                    <label class="block text-xs text-text-secondary mb-1">加密密码</label>
                    <el-input v-model="tempCredential.privPassword" type="password" show-password />
                  </div>
                </template>
              </div>
            </div>

            <div class="flex items-center gap-3 mt-4">
              <button
                class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg font-medium transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
                :disabled="queryLoading"
                @click="runQuery"
              >
                {{ queryLoading ? "查询中…" : "开始查询" }}
              </button>
              <button
                class="px-4 py-2 bg-bg-tertiary hover:bg-bg-hover border border-border text-text-primary rounded-lg font-medium transition-all duration-200"
                @click="queryResults = []; queryAddressDone = ''"
              >
                清空结果
              </button>
              <span v-if="queryAddressDone" class="text-xs text-text-muted">
                {{ queryAddressDone }} · 耗时 {{ queryLatency }} ms
              </span>
            </div>
          </section>

          <!-- 查询结果 -->
          <section
            class="flex-1 min-h-0 bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md overflow-hidden flex flex-col"
          >
            <div class="px-4 py-2 border-b border-border">
              <h2 class="text-sm font-semibold text-text-primary">
                查询结果
                <span class="text-xs text-text-muted font-normal">（{{ queryResults.length }} 条）</span>
              </h2>
            </div>
            <div class="flex-1 overflow-auto scrollbar-custom">
              <table v-if="queryResults.length > 0" class="w-full text-sm">
                <thead class="sticky top-0 bg-bg-secondary">
                  <tr class="text-left text-text-muted border-b border-border">
                    <th class="py-2 px-3 font-medium">OID</th>
                    <th class="py-2 px-3 font-medium">名称</th>
                    <th class="py-2 px-3 font-medium">值</th>
                    <th class="py-2 px-3 font-medium">类型</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(r, i) in queryResults"
                    :key="`${r.oid}-${i}`"
                    class="border-b border-border/50 hover:bg-bg-hover/50 transition-colors"
                  >
                    <td class="py-2 px-3 font-mono text-xs text-text-muted">{{ r.oid }}</td>
                    <td class="py-2 px-3 text-text-primary">{{ r.oidName }}</td>
                    <td class="py-2 px-3 text-text-primary break-all">
                      <span v-if="r.error" class="text-error">{{ r.error }}</span>
                      <span v-else>{{ r.value || "-" }}</span>
                    </td>
                    <td class="py-2 px-3 text-text-muted">{{ r.valueType }}</td>
                  </tr>
                </tbody>
              </table>
              <div v-else class="h-full flex items-center justify-center text-sm text-text-muted py-10">
                暂无查询结果
              </div>
            </div>
          </section>
        </div>
      </el-tab-pane>

      <!-- ==================== 批量上线确认 ==================== -->
      <el-tab-pane label="批量上线确认" name="batch">
        <div class="flex flex-col gap-4 h-full">
          <section class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4">
            <label class="block text-xs text-text-secondary mb-1">
              目标地址列表（换行 / 逗号 / 空格分隔，共 {{ batchAddressList.length }} 个）
            </label>
            <el-input
              v-model="batchAddressInput"
              type="textarea"
              :rows="4"
              placeholder="192.168.1.1&#10;192.168.1.2&#10;10.0.0.1-254 需自行展开"
            />

            <div class="flex items-center gap-3 mt-3">
              <el-checkbox v-model="batchUseTempCredential">使用临时凭据（不保存）</el-checkbox>
              <el-select
                v-if="!batchUseTempCredential"
                v-model="batchCredentialId"
                placeholder="选择已保存凭据（不选则使用 public）"
                clearable
                class="flex-1"
                :loading="credentialLoading"
              >
                <el-option
                  v-for="c in credentials"
                  :key="c.id"
                  :label="`${c.name} (${c.version})`"
                  :value="c.id"
                />
              </el-select>
            </div>

            <div
              v-if="batchUseTempCredential"
              class="grid grid-cols-1 md:grid-cols-3 gap-3 mt-3 p-3 bg-bg-tertiary/30 rounded-lg"
            >
              <div>
                <label class="block text-xs text-text-secondary mb-1">版本</label>
                <el-select v-model="batchTempCredential.version" class="w-full">
                  <el-option
                    v-for="v in SNMP_VERSION_OPTIONS"
                    :key="v.value"
                    :label="v.label"
                    :value="v.value"
                  />
                </el-select>
              </div>
              <div v-if="!isBatchV3">
                <label class="block text-xs text-text-secondary mb-1">Community</label>
                <el-input v-model="batchTempCredential.community" placeholder="public" />
              </div>
              <template v-if="isBatchV3">
                <div>
                  <label class="block text-xs text-text-secondary mb-1">用户名</label>
                  <el-input v-model="batchTempCredential.username" />
                </div>
                <div>
                  <label class="block text-xs text-text-secondary mb-1">安全级别</label>
                  <el-select v-model="batchTempCredential.securityLevel" class="w-full">
                    <el-option
                      v-for="s in SNMP_SECURITY_LEVEL_OPTIONS"
                      :key="s.value"
                      :label="s.label"
                      :value="s.value"
                    />
                  </el-select>
                </div>
                <div>
                  <label class="block text-xs text-text-secondary mb-1">认证密码</label>
                  <el-input v-model="batchTempCredential.authPassword" type="password" show-password />
                </div>
                <div>
                  <label class="block text-xs text-text-secondary mb-1">加密密码</label>
                  <el-input v-model="batchTempCredential.privPassword" type="password" show-password />
                </div>
              </template>
            </div>

            <div class="flex items-center gap-3 mt-4">
              <button
                class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg font-medium transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
                :disabled="batchLoading"
                @click="runBatchQuery"
              >
                {{ batchLoading ? "检测中…" : "开始检测" }}
              </button>
              <span v-if="batchResults.length > 0" class="text-xs text-text-muted">
                可达 {{ batchReachableCount }}/{{ batchResults.length }} · 耗时 {{ batchLatency }} ms
              </span>
            </div>
          </section>

          <section
            class="flex-1 min-h-0 bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md overflow-hidden flex flex-col"
          >
            <div class="flex-1 overflow-auto scrollbar-custom">
              <table v-if="batchResults.length > 0" class="w-full text-sm">
                <thead class="sticky top-0 bg-bg-secondary">
                  <tr class="text-left text-text-muted border-b border-border">
                    <th class="py-2 px-3 font-medium">地址</th>
                    <th class="py-2 px-3 font-medium">状态</th>
                    <th class="py-2 px-3 font-medium">sysName</th>
                    <th class="py-2 px-3 font-medium">sysDescr</th>
                    <th class="py-2 px-3 font-medium">sysUpTime</th>
                    <th class="py-2 px-3 font-medium">耗时</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="r in batchResults"
                    :key="r.address"
                    class="border-b border-border/50 hover:bg-bg-hover/50 transition-colors"
                  >
                    <td class="py-2 px-3 font-mono text-xs text-text-primary">{{ r.address }}</td>
                    <td class="py-2 px-3">
                      <span
                        class="px-2 py-0.5 rounded-full text-xs"
                        :class="r.reachable ? 'bg-success/20 text-success' : 'bg-error/20 text-error'"
                      >
                        {{ r.reachable ? "可达" : "不可达" }}
                      </span>
                    </td>
                    <td class="py-2 px-3 text-text-primary">
                      {{ extractValue(r.results, "sysName") }}
                    </td>
                    <td class="py-2 px-3 text-text-secondary" :title="extractValue(r.results, 'sysDescr')">
                      {{ truncate(extractValue(r.results, "sysDescr")) }}
                    </td>
                    <td class="py-2 px-3 text-text-muted">
                      {{ extractValue(r.results, "sysUpTime") }}
                    </td>
                    <td class="py-2 px-3 text-text-muted">{{ r.latencyMs }} ms</td>
                  </tr>
                </tbody>
              </table>
              <div v-else class="h-full flex items-center justify-center text-sm text-text-muted py-10">
                暂无检测结果
              </div>
            </div>
          </section>
        </div>
      </el-tab-pane>

      <!-- ==================== 凭据管理 ==================== -->
      <el-tab-pane label="凭据管理" name="credential">
        <section class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4">
          <div class="flex items-center justify-between mb-3">
            <h2 class="text-sm font-semibold text-text-primary">SNMP 查询凭据</h2>
            <button
              class="px-3 py-1.5 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors"
              @click="openCreateCredential"
            >
              新增凭据
            </button>
          </div>

          <table v-if="credentials.length > 0" class="w-full text-sm">
            <thead>
              <tr class="text-left text-text-muted border-b border-border">
                <th class="py-2 px-3 font-medium">名称</th>
                <th class="py-2 px-3 font-medium">版本</th>
                <th class="py-2 px-3 font-medium">用户名 / Community</th>
                <th class="py-2 px-3 font-medium">安全级别</th>
                <th class="py-2 px-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="c in credentials"
                :key="c.id"
                class="border-b border-border/50 hover:bg-bg-hover/50 transition-colors"
              >
                <td class="py-2 px-3 text-text-primary">{{ c.name }}</td>
                <td class="py-2 px-3 text-text-secondary">{{ c.version }}</td>
                <td class="py-2 px-3 text-text-secondary">
                  {{ c.version === "v3" ? c.username || "-" : "••••••" }}
                </td>
                <td class="py-2 px-3 text-text-muted">
                  {{ c.version === "v3" ? c.securityLevel : "-" }}
                </td>
                <td class="py-2 px-3">
                  <button
                    class="text-accent hover:underline text-xs mr-3"
                    @click="openEditCredential(c)"
                  >
                    编辑
                  </button>
                  <button class="text-error hover:underline text-xs" @click="removeCredential(c)">
                    删除
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else class="text-sm text-text-muted py-8 text-center">
            暂无凭据，点击「新增凭据」创建，或直接在查询时使用临时凭据
          </div>
        </section>
      </el-tab-pane>
    </el-tabs>

    <!-- 凭据编辑对话框 -->
    <el-dialog
      v-model="credentialDialogVisible"
      :title="credentialEditing ? '编辑凭据' : '新增凭据'"
      width="520px"
    >
      <el-form label-width="110px">
        <el-form-item label="名称" required>
          <el-input v-model="credentialForm.name" placeholder="如：交付现场-只读" />
        </el-form-item>
        <el-form-item label="版本">
          <el-select v-model="credentialForm.version" class="w-full">
            <el-option
              v-for="v in SNMP_VERSION_OPTIONS"
              :key="v.value"
              :label="v.label"
              :value="v.value"
            />
          </el-select>
        </el-form-item>

        <template v-if="!isCredentialFormV3">
          <el-form-item label="Community">
            <el-input
              v-model="credentialForm.community"
              :placeholder="credentialEditing ? '留空表示保持原值不变' : 'public'"
            />
          </el-form-item>
        </template>

        <template v-else>
          <el-form-item label="用户名">
            <el-input v-model="credentialForm.username" />
          </el-form-item>
          <el-form-item label="安全级别">
            <el-select v-model="credentialForm.securityLevel" class="w-full">
              <el-option
                v-for="s in SNMP_SECURITY_LEVEL_OPTIONS"
                :key="s.value"
                :label="s.label"
                :value="s.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="认证协议">
            <el-select v-model="credentialForm.authProtocol" class="w-full">
              <el-option v-for="a in SNMP_AUTH_PROTOCOL_OPTIONS" :key="a" :label="a" :value="a" />
            </el-select>
          </el-form-item>
          <el-form-item label="认证密码">
            <el-input
              v-model="credentialForm.authPassword"
              type="password"
              show-password
              :placeholder="credentialEditing ? '留空表示保持原值不变' : ''"
            />
          </el-form-item>
          <el-form-item label="加密协议">
            <el-select v-model="credentialForm.privProtocol" class="w-full">
              <el-option v-for="p in SNMP_PRIV_PROTOCOL_OPTIONS" :key="p" :label="p" :value="p" />
            </el-select>
          </el-form-item>
          <el-form-item label="加密密码">
            <el-input
              v-model="credentialForm.privPassword"
              type="password"
              show-password
              :placeholder="credentialEditing ? '留空表示保持原值不变' : ''"
            />
          </el-form-item>
          <el-form-item label="Context Name">
            <el-input v-model="credentialForm.contextName" />
          </el-form-item>
          <el-form-item label="Engine ID">
            <el-input v-model="credentialForm.contextEngineId" />
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="credentialDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="credentialSaving" @click="saveCredential">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
