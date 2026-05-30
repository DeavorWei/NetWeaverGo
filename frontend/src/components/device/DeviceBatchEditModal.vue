<template>
  <el-dialog
    :model-value="show"
    @update:model-value="$emit('close')"
    :title="`批量修改${batchFieldLabel}`"
    width="400px"
    destroy-on-close
    :close-on-click-modal="false"
  >
    <template #header="{ titleId, titleClass }">
      <div class="flex items-center">
        <h4 :id="titleId" :class="titleClass">
          批量修改{{ batchFieldLabel }}
          <span v-if="selectedCount > 0" class="ml-2 text-sm font-normal text-accent">
            ({{ selectedCount }} 台设备)
          </span>
        </h4>
      </div>
    </template>

    <div class="mb-4 text-sm text-text-secondary">
      将选中的设备的{{ batchFieldLabel }}修改为：
    </div>

    <el-form @submit.prevent="handleSubmit" label-width="0">
      <el-form-item v-if="field === 'protocol'">
        <el-select v-model="localValue" class="w-full">
          <el-option v-for="p in validProtocols" :key="p" :label="p" :value="p" />
        </el-select>
      </el-form-item>

      <el-form-item v-else-if="field === 'port'">
        <el-input-number
          v-model="localValue"
          :min="1"
          :max="65535"
          controls-position="right"
          class="w-full"
          placeholder="端口号"
        />
      </el-form-item>

      <el-form-item v-else-if="field === 'password'">
        <el-input
          v-model="localValue"
          type="password"
          show-password
          :placeholder="`请输入${batchFieldLabel}`"
        />
      </el-form-item>
      
      <el-form-item v-else-if="field === 'tag'">
        <div class="w-full">
          <el-input
            v-model="localValue"
            :placeholder="`请输入${batchFieldLabel} (多个用逗号分隔)`"
          />
          <div class="mt-1 text-[10px] text-text-muted leading-tight">
            提示：输入多个标签时请使用英文或中文逗号分隔。
          </div>
        </div>
      </el-form-item>

      <el-form-item v-else>
        <el-input
          v-model="localValue"
          :placeholder="`请输入${batchFieldLabel}`"
        />
      </el-form-item>
    </el-form>

    <div v-if="errorMessage" class="mb-4 px-3 py-2 text-sm text-error bg-error-bg border border-error/30 rounded-lg">
      {{ errorMessage }}
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <el-button @click="$emit('close')">取消</el-button>
        <el-button type="primary" :loading="isSaving" @click="handleSubmit">
          确定
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";

// 批量编辑字段类型
export type BatchField =
  | "group"
  | "protocol"
  | "port"
  | "username"
  | "password"
  | "tag";

// Props
interface Props {
  show: boolean;
  field: BatchField | null;
  selectedCount: number;
  validProtocols?: string[];
}

const props = withDefaults(defineProps<Props>(), {
  validProtocols: () => ["SSH", "SNMP", "TELNET"],
});

// Emits
interface Emits {
  (e: "close"): void;
  (e: "save", field: BatchField, value: string | number): void;
}

const emit = defineEmits<Emits>();

// 本地状态
const localValue = ref<string | number>("");
const errorMessage = ref("");
const isSaving = ref(false);

// 字段标签映射
const fieldLabels: Record<BatchField, string> = {
  protocol: "协议",
  port: "端口",
  username: "用户名",
  password: "密码",
  group: "分组",
  tag: "标签",
};

// 计算属性
const batchFieldLabel = computed(() => {
  return props.field ? fieldLabels[props.field] : "";
});

// 监听 show 变化，重置状态
watch(
  () => props.show,
  (newShow) => {
    if (newShow) {
      localValue.value = "";
      errorMessage.value = "";
      isSaving.value = false;
    }
  },
);

// 提交表单
function handleSubmit() {
  if (!props.field) return;

  // 验证
  if (props.field === "port") {
    const port = Number(localValue.value);
    if (isNaN(port) || port < 1 || port > 65535) {
      errorMessage.value = "端口号必须在 1-65535 之间";
      return;
    }
  }

  if (props.field === "tag" && !localValue.value) {
    errorMessage.value = "请输入标签";
    return;
  }

  emit("save", props.field, localValue.value);
}

// 暴露方法供父组件使用
defineExpose({
  setSaving: (value: boolean) => {
    isSaving.value = value;
  },
  setError: (value: string) => {
    errorMessage.value = value;
  },
});
</script>
