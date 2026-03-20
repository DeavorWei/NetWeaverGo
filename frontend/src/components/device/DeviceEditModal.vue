<template>
  <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center">
    <!-- 背景遮罩 -->
    <div
      class="absolute inset-0 bg-black/50 backdrop-blur-sm"
      @click="$emit('close')"
    ></div>

    <!-- 弹窗内容 -->
    <div
      class="relative bg-bg-card border border-border rounded-xl shadow-2xl w-full max-w-md mx-4 animate-slide-in"
    >
      <!-- 标题栏 -->
      <div
        class="flex items-center justify-between px-6 py-4 border-b border-border"
      >
        <h3 class="text-lg font-semibold text-text-primary">
          {{ isEditing ? "编辑设备" : "新增设备" }}
        </h3>
        <button
          @click="$emit('close')"
          class="p-1 text-text-muted hover:text-text-primary transition-colors"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-5 h-5"
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

      <!-- 表单 -->
      <form @submit.prevent="handleSubmit" class="p-6 space-y-4">
        <!-- 分组 -->
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">
            分组 <span class="text-text-muted">(可选)</span>
          </label>
          <input
            v-model="localForm.group"
            type="text"
            placeholder="设备分组名称"
            class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
          />
        </div>

        <!-- IP 地址 -->
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">
            IP 地址
          </label>
          <input
            v-model="localForm.ip"
            type="text"
            placeholder="例如: 192.168.1.10 或 192.168.1.10-20"
            :class="[
              'w-full px-3 py-2 text-sm bg-bg-panel border rounded-lg text-text-primary placeholder-text-muted/50 focus:outline-none transition-colors',
              ipValidationError
                ? 'border-error focus:border-error'
                : 'border-border focus:border-accent',
            ]"
            required
          />
          <!-- IP 输入错误提示 -->
          <div
            v-if="ipValidationError"
            class="mt-2 px-3 py-2 text-xs bg-error-bg border border-error/30 rounded-lg text-error flex items-center gap-2"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-4 h-4 flex-shrink-0"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="12" cy="12" r="10" />
              <line x1="15" y1="9" x2="9" y2="15" />
              <line x1="9" y1="9" x2="15" y2="15" />
            </svg>
            <span>{{ ipValidationError }}</span>
          </div>
          <!-- IP 语法糖提示 -->
          <div
            v-if="ipRangeHint"
            class="mt-2 px-3 py-2 text-xs bg-accent/10 border border-accent/20 rounded-lg text-accent flex items-center gap-2"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-4 h-4 flex-shrink-0"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="16" x2="12" y2="12" />
              <line x1="12" y1="8" x2="12.01" y2="8" />
            </svg>
            <span>
              语法糖：将新增 <strong>{{ ipRangeHint.count }}</strong> 台设备 ({{
                ipRangeHint.start
              }}
              - {{ ipRangeHint.end }})
            </span>
          </div>
        </div>

        <!-- 协议和端口 -->
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5">
              协议
            </label>
            <select
              v-model="localForm.protocol"
              @change="onProtocolChange"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary focus:border-accent focus:outline-none transition-colors"
            >
              <option v-for="p in validProtocols" :key="p" :value="p">
                {{ p }}
              </option>
            </select>
          </div>
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5">
              端口
            </label>
            <input
              v-model.number="localForm.port"
              type="number"
              placeholder="端口号"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
              min="1"
              max="65535"
            />
          </div>
        </div>

        <!-- 用户名和密码 -->
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5">
              用户名 <span class="text-text-muted">(可选)</span>
            </label>
            <input
              v-model="localForm.username"
              type="text"
              placeholder="登录用户名"
              class="w-full px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
            />
          </div>
          <div>
            <label class="block text-xs font-medium text-text-secondary mb-1.5">
              密码 <span class="text-text-muted">(可选)</span>
            </label>
            <div class="relative">
              <input
                v-model="localForm.password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="登录密码"
                autocomplete="off"
                class="w-full px-3 py-2 pr-10 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
              />
              <button
                type="button"
                @click="showPassword = !showPassword"
                :title="showPassword ? '隐藏密码' : '查看密码'"
                class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text-primary transition-colors"
              >
                <svg
                  v-if="showPassword"
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-4 h-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
                  />
                  <line x1="1" y1="1" x2="23" y2="23" />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-4 h-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                  <circle cx="12" cy="12" r="3" />
                </svg>
              </button>
            </div>
          </div>
        </div>

        <!-- 标签 -->
        <div>
          <label class="block text-xs font-medium text-text-secondary mb-1.5">
            标签 <span class="text-text-muted">(可选)</span>
          </label>
          <div class="flex flex-wrap gap-2 mb-2">
            <span
              v-for="(tag, index) in localForm.tags"
              :key="index"
              class="inline-flex items-center gap-1 px-2 py-1 text-xs bg-accent/10 text-accent rounded-md"
            >
              {{ tag }}
              <button
                type="button"
                @click="removeTag(index)"
                class="hover:text-error transition-colors"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-3 h-3"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </span>
          </div>
          <div class="flex gap-2">
            <input
              v-model="newTag"
              type="text"
              placeholder="添加标签"
              class="flex-1 px-3 py-2 text-sm bg-bg-panel border border-border rounded-lg text-text-primary placeholder-text-muted/50 focus:border-accent focus:outline-none transition-colors"
              @keyup.enter="addTag"
            />
            <button
              type="button"
              @click="addTag"
              class="px-3 py-2 text-sm text-accent border border-accent/30 rounded-lg hover:bg-accent/10 transition-colors"
            >
              添加
            </button>
          </div>
        </div>

        <!-- 错误消息 -->
        <div
          v-if="errorMessage"
          class="px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg"
        >
          {{ errorMessage }}
        </div>

        <!-- 操作按钮 -->
        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            @click="$emit('close')"
            class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover transition-colors"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="isSaving"
            class="px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent/90 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ isSaving ? "保存中..." : "确定" }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from "vue";
import type { DeviceFormData, IpRangeHint } from "@/composables/useDeviceForm";

// Props
interface Props {
  show: boolean;
  isEditing: boolean;
  formData?: DeviceFormData;
  validProtocols?: string[];
  protocolDefaultPorts?: Record<string, number>;
}

const props = withDefaults(defineProps<Props>(), {
  formData: undefined,
  validProtocols: () => ["SSH", "SNMP", "TELNET"],
  protocolDefaultPorts: () => ({ SSH: 22, SNMP: 161, TELNET: 23 }),
});

// Emits
interface Emits {
  (e: "close"): void;
  (e: "save", data: DeviceFormData): void;
}

const emit = defineEmits<Emits>();

// 本地表单状态
const localForm = ref<DeviceFormData>({
  ip: "",
  port: 22,
  protocol: "SSH",
  username: "",
  password: "",
  group: "",
  tags: [],
  vendor: "",
  role: "",
  site: "",
  displayName: "",
  description: "",
});

// 其他状态
const showPassword = ref(false);
const errorMessage = ref("");
const isSaving = ref(false);
const newTag = ref("");
const lastProtocol = ref("SSH");

// IP 验证相关
const ipValidationError = ref("");
const ipRangeHint = ref<IpRangeHint | null>(null);

// 监听 formData 变化，更新本地表单
watch(
  () => props.formData,
  (newData) => {
    if (newData) {
      localForm.value = {
        ...newData,
        tags: [...newData.tags],
      };
      lastProtocol.value = newData.protocol;
    } else {
      resetForm();
    }
  },
  { immediate: true },
);

// 监听 show 变化，重置状态并清空密码
watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      // 关闭时清空敏感数据
      localForm.value.password = "";
      showPassword.value = false;
      errorMessage.value = "";
      ipValidationError.value = "";
      ipRangeHint.value = null;
    }
  },
);

// 组件卸载时清理密码
onUnmounted(() => {
  localForm.value.password = "";
  showPassword.value = false;
});

// 监听 IP 输入，解析语法糖并验证
watch(
  () => localForm.value.ip,
  (newIp) => {
    ipRangeHint.value = parseIpRange(newIp);
    const validation = validateIpInput(newIp);
    ipValidationError.value = validation.error;
  },
);

// IP 验证函数
function isValidIp(ip: string): boolean {
  const parts = ip.split(".");
  if (parts.length !== 4) return false;
  return parts.every((part) => {
    if (part === "" || part.length > 3) return false;
    const num = parseInt(part, 10);
    return !isNaN(num) && num >= 0 && num <= 255 && part === num.toString();
  });
}

function parseIpRange(ip: string): IpRangeHint | null {
  if (!ip) return null;
  const match = ip.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/);
  if (match && match[1] && match[2] && match[3]) {
    const prefix = match[1];
    const start = parseInt(match[2], 10);
    const end = parseInt(match[3], 10);
    if (start < end && start >= 0 && end <= 255) {
      return {
        count: end - start + 1,
        start: prefix + start,
        end: prefix + end,
      };
    }
  }
  return null;
}

function validateIpInput(ip: string): { valid: boolean; error: string } {
  if (!ip) return { valid: false, error: "" };
  if (isValidIp(ip)) return { valid: true, error: "" };
  if (parseIpRange(ip)) return { valid: true, error: "" };

  const rangeMatch = ip.match(
    /^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/,
  );
  if (rangeMatch && rangeMatch[2] && rangeMatch[3]) {
    const start = parseInt(rangeMatch[2], 10);
    const end = parseInt(rangeMatch[3], 10);
    if (start > 255 || end > 255) {
      return { valid: false, error: "IP 段值必须在 0-255 范围内" };
    }
    if (start >= end) {
      return { valid: false, error: "起始值必须小于结束值" };
    }
  }

  return {
    valid: false,
    error: "请输入有效 IP（如 192.168.1.10）或范围格式（如 192.168.1.10-20）",
  };
}

// 协议变更处理
function onProtocolChange() {
  const oldDefaultPort = props.protocolDefaultPorts[lastProtocol.value] || 22;
  const newDefaultPort =
    props.protocolDefaultPorts[localForm.value.protocol] || 22;

  if (localForm.value.port === oldDefaultPort) {
    localForm.value.port = newDefaultPort;
  }

  lastProtocol.value = localForm.value.protocol;
}

// 标签操作
function addTag() {
  const tag = newTag.value.trim();
  if (tag && !localForm.value.tags.includes(tag)) {
    localForm.value.tags.push(tag);
  }
  newTag.value = "";
}

function removeTag(index: number) {
  localForm.value.tags.splice(index, 1);
}

// 重置表单
function resetForm() {
  localForm.value = {
    ip: "",
    port: 22,
    protocol: "SSH",
    username: "",
    password: "",
    group: "",
    tags: [],
    vendor: "",
    role: "",
    site: "",
    displayName: "",
    description: "",
  };
  newTag.value = "";
  lastProtocol.value = "SSH";
}

// 提交表单
function handleSubmit() {
  if (ipValidationError.value) {
    return;
  }
  emit("save", { ...localForm.value, tags: [...localForm.value.tags] });
}

// 暴露方法供父组件使用
defineExpose({
  setSaving: (value: boolean) => {
    isSaving.value = value;
  },
  setError: (value: string) => {
    errorMessage.value = value;
  },
  resetForm,
});
</script>
