<template>
  <el-dialog
    v-model="visible"
    title="Trap 监听器配置"
    width="500px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-width="100px"
      label-position="right"
    >
      <!-- 监听端口 -->
      <el-form-item label="监听端口" prop="trapPort">
        <el-input-number
          v-model="formData.trapPort"
          :min="1"
          :max="65535"
          :step="1"
          controls-position="right"
          class="w-full"
        />
      </el-form-item>

      <!-- Community -->
      <el-form-item label="Community" prop="trapCommunity">
        <el-input
          v-model="formData.trapCommunity"
          placeholder="默认: public"
          clearable
        />
      </el-form-item>

      <!-- 存储天数 -->
      <el-form-item label="存储天数" prop="maxStorageDays">
        <el-input-number
          v-model="formData.maxStorageDays"
          :min="1"
          :max="365"
          :step="1"
          controls-position="right"
          class="w-full"
        />
      </el-form-item>

      <!-- 启用 Trap -->
      <el-form-item label="启用 Trap">
        <el-switch v-model="formData.trapEnabled" />
      </el-form-item>

      <!-- SNMPv3 配置 -->
      <el-divider content-position="left">SNMPv3 配置</el-divider>

      <el-form-item label="启用 v3">
        <el-switch v-model="formData.enableV3" />
      </el-form-item>

      <template v-if="formData.enableV3">
        <!-- 用户名 -->
        <el-form-item label="用户名" prop="v3Username">
          <el-input
            v-model="formData.v3Username"
            placeholder="SNMPv3 用户名"
            clearable
          />
        </el-form-item>

        <!-- 认证协议 -->
        <el-form-item label="认证协议" prop="v3AuthProtocol">
          <el-select v-model="formData.v3AuthProtocol" class="w-full">
            <el-option
              v-for="opt in authProtocolOptions"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-select>
        </el-form-item>

        <!-- 认证密码 -->
        <el-form-item label="认证密码" prop="v3AuthPassword">
          <el-input
            v-model="formData.v3AuthPassword"
            type="password"
            placeholder="认证密码"
            show-password
          />
        </el-form-item>

        <!-- 加密协议 -->
        <el-form-item label="加密协议" prop="v3PrivProtocol">
          <el-select v-model="formData.v3PrivProtocol" class="w-full">
            <el-option
              v-for="opt in privProtocolOptions"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-select>
        </el-form-item>

        <!-- 加密密码 -->
        <el-form-item label="加密密码" prop="v3PrivPassword">
          <el-input
            v-model="formData.v3PrivPassword"
            type="password"
            placeholder="加密密码"
            show-password
          />
        </el-form-item>

        <!-- 引擎ID -->
        <el-form-item label="引擎ID" prop="v3EngineId">
          <el-input
            v-model="formData.v3EngineId"
            placeholder="可选，留空自动生成"
            clearable
          />
        </el-form-item>
      </template>
    </el-form>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" @click="handleSave" :loading="saving">
        保存
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { V3_AUTH_PROTOCOLS, V3_PRIV_PROTOCOLS } from '@/types/snmp'
import type { ServerConfig, UpdateServerConfigRequest } from '@/types/snmp'

// Props
interface Props {
  modelValue: boolean
  config?: ServerConfig | null
  saving?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  config: null,
  saving: false,
})

// Emits
const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'save', config: UpdateServerConfigRequest): void
}>()

// 表单引用
const formRef = ref<FormInstance>()

// 对话框可见性
const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
})

// 选项列表
const authProtocolOptions = V3_AUTH_PROTOCOLS
const privProtocolOptions = V3_PRIV_PROTOCOLS

// 表单数据
const formData = reactive<{
  trapPort: number
  trapCommunity: string
  maxStorageDays: number
  trapEnabled: boolean
  enableV3: boolean
  v3Username: string
  v3AuthProtocol: string
  v3AuthPassword: string
  v3PrivProtocol: string
  v3PrivPassword: string
  v3EngineId: string
}>({
  trapPort: 162,
  trapCommunity: 'public',
  maxStorageDays: 30,
  trapEnabled: true,
  enableV3: false,
  v3Username: '',
  v3AuthProtocol: 'MD5',
  v3AuthPassword: '',
  v3PrivProtocol: 'AES',
  v3PrivPassword: '',
  v3EngineId: '',
})

// 表单验证规则
const formRules = computed<FormRules>(() => ({
  trapPort: [
    { required: true, message: '请输入监听端口', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535', trigger: 'blur' },
  ],
  trapCommunity: [
    { required: true, message: '请输入 Community', trigger: 'blur' },
  ],
  maxStorageDays: [
    { required: true, message: '请输入存储天数', trigger: 'blur' },
    { type: 'number', min: 1, max: 365, message: '天数范围 1-365', trigger: 'blur' },
  ],
  v3Username: formData.enableV3
    ? [{ required: true, message: '请输入用户名', trigger: 'blur' }]
    : [],
  v3AuthPassword: formData.enableV3
    ? [{ required: true, message: '请输入认证密码', trigger: 'blur' }]
    : [],
  v3PrivPassword: formData.enableV3
    ? [{ required: true, message: '请输入加密密码', trigger: 'blur' }]
    : [],
}))

// 监听配置变化，填充表单
watch(
  () => props.config,
  (config) => {
    if (config) {
      formData.trapPort = config.trapPort
      formData.trapCommunity = config.trapCommunity
      formData.maxStorageDays = config.maxStorageDays
      formData.trapEnabled = config.trapEnabled
      formData.enableV3 = false
      formData.v3Username = ''
      formData.v3AuthProtocol = 'MD5'
      formData.v3AuthPassword = ''
      formData.v3PrivProtocol = 'AES'
      formData.v3PrivPassword = ''
      formData.v3EngineId = ''
    }
  },
  { immediate: true }
)

// 保存配置
async function handleSave() {
  if (!formRef.value) return

  try {
    const valid = await formRef.value.validate()
    if (!valid) return
  } catch {
    return
  }

  const requestData: UpdateServerConfigRequest = {
    trapEnabled: formData.trapEnabled,
    trapPort: formData.trapPort,
    trapCommunity: formData.trapCommunity,
    maxStorageDays: formData.maxStorageDays,
  }

  emit('save', requestData)
}

// 关闭对话框
function handleClose() {
  formRef.value?.resetFields()
  visible.value = false
}
</script>

<style scoped>
.w-full {
  width: 100%;
}

.el-divider {
  margin: 16px 0;
}
</style>