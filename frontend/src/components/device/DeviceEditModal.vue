<template>
  <el-dialog
    :model-value="show"
    @update:model-value="$emit('close')"
    :title="isEditing ? '编辑设备' : '新增设备'"
    width="500px"
    destroy-on-close
    :close-on-click-modal="false"
  >
    <el-form
      ref="formRef"
      :model="localForm"
      :rules="rules"
      label-width="80px"
      label-position="right"
      class="mt-2"
    >
      <el-form-item label="分组" prop="group">
        <el-input v-model="localForm.group" placeholder="设备分组名称 (可选)" />
      </el-form-item>

      <el-form-item label="IP 地址" prop="ip">
        <el-input v-model="localForm.ip" placeholder="例如: 192.168.1.10 或 192.168.1.10-20" />
        <div v-if="ipRangeHint" class="mt-1 text-xs text-accent w-full">
          语法糖：将新增 {{ ipRangeHint.count }} 台设备 ({{ ipRangeHint.start }} - {{ ipRangeHint.end }})
        </div>
      </el-form-item>

      <el-form-item label="协议" prop="protocol">
        <el-select v-model="localForm.protocol" @change="onProtocolChange" class="w-full">
          <el-option v-for="p in validProtocols" :key="p" :label="p" :value="p" />
        </el-select>
      </el-form-item>

      <el-form-item label="端口" prop="port">
        <el-input-number v-model="localForm.port" :min="1" :max="65535" controls-position="right" class="w-full" />
      </el-form-item>

      <el-form-item label="用户名" prop="username">
        <el-input v-model="localForm.username" placeholder="登录用户名 (可选)" />
      </el-form-item>

      <el-form-item label="密码" prop="password">
        <el-input v-model="localForm.password" type="password" show-password placeholder="登录密码 (可选)" autocomplete="off" />
      </el-form-item>

      <el-form-item label="标签" prop="tags">
        <div class="flex flex-wrap items-center gap-2 w-full">
          <el-tag
            v-for="(tag, index) in localForm.tags"
            :key="index"
            closable
            @close="removeTag(index)"
          >
            {{ tag }}
          </el-tag>
          <div class="flex items-center gap-2 flex-1 min-w-[180px]">
            <el-input
              v-model="newTag"
              placeholder="添加标签"
              @keyup.enter="addTag"
              class="flex-1"
            />
            <el-button @click="addTag">添加</el-button>
          </div>
        </div>
      </el-form-item>
    </el-form>

    <div v-if="errorMessage" class="mb-4 px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg">
      {{ errorMessage }}
    </div>

    <template #footer>
      <div class="flex items-center justify-between">
        <div>
          <el-button
            v-if="isEditing && localForm.protocol === 'SSH'"
            type="danger"
            plain
            @click="emit('reset-ssh-host-key')"
            :loading="isSaving"
          >
            重置主机SSH密钥
          </el-button>
        </div>
        <div class="flex gap-2">
          <el-button @click="$emit('close')">取消</el-button>
          <el-button type="primary" :loading="isSaving" @click="handleSubmit">
            确定
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted, reactive } from "vue";
import type { DeviceFormData, IpRangeHint } from "@/composables/useDeviceForm";
import type { FormInstance, FormRules } from 'element-plus'

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
  (e: "reset-ssh-host-key"): void;
}

const emit = defineEmits<Emits>();

const formRef = ref<FormInstance>()
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

const errorMessage = ref("");
const isSaving = ref(false);
const newTag = ref("");
const lastProtocol = ref("SSH");
const ipRangeHint = ref<IpRangeHint | null>(null);

const validateIp = (_rule: any, value: any, callback: any) => {
  const result = validateIpInput(value);
  if (!result.valid) {
    callback(new Error(result.error));
  } else {
    callback();
  }
}

const rules = reactive<FormRules>({
  ip: [{ required: true, validator: validateIp, trigger: 'blur' }],
  protocol: [{ required: true, message: '请选择协议', trigger: 'change' }],
  port: [{ required: true, message: '请输入端口', trigger: 'blur' }]
})

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

watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      localForm.value.password = "";
      errorMessage.value = "";
      ipRangeHint.value = null;
      if (formRef.value) formRef.value.clearValidate();
    }
  },
);

onUnmounted(() => {
  localForm.value.password = "";
});

watch(
  () => localForm.value.ip,
  (newIp) => {
    ipRangeHint.value = parseIpRange(newIp);
  },
);

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
  const match = ip.match(
    /^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})([-~])(\d{1,3})$/,
  );
  if (match && match[1] && match[2] && match[4]) {
    const prefix = match[1];
    const start = parseInt(match[2], 10);
    const end = parseInt(match[4], 10);
    if (start <= end && start >= 0 && end <= 255) {
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
  if (!ip) return { valid: false, error: "IP不能为空" };
  if (isValidIp(ip)) return { valid: true, error: "" };
  if (parseIpRange(ip)) return { valid: true, error: "" };

  const rangeMatch = ip.match(
    /^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})([-~])(\d{1,3})$/,
  );
  if (rangeMatch && rangeMatch[2] && rangeMatch[4]) {
    const start = parseInt(rangeMatch[2], 10);
    const end = parseInt(rangeMatch[4], 10);
    if (start > 255 || end > 255) {
      return { valid: false, error: "IP 段值必须在 0-255 范围内" };
    }
    if (start > end) {
      return { valid: false, error: "起始值必须小于或等于结束值" };
    }
  }

  return {
    valid: false,
    error: "请输入有效 IP（如 192.168.1.10）或范围格式（如 192.168.1.10-20）",
  };
}

function onProtocolChange() {
  const oldDefaultPort = props.protocolDefaultPorts[lastProtocol.value] || 22;
  const newDefaultPort =
    props.protocolDefaultPorts[localForm.value.protocol] || 22;

  if (localForm.value.port === oldDefaultPort) {
    localForm.value.port = newDefaultPort;
  }

  lastProtocol.value = localForm.value.protocol;
}

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

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate((valid) => {
    if (valid) {
      emit("save", { ...localForm.value, tags: [...localForm.value.tags] });
    }
  })
}

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
