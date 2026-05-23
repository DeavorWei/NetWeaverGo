<template>
  <div class="polling-target-form">
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-width="100px"
      label-position="right"
      size="default"
    >
      <!-- 目标IP -->
      <el-form-item label="目标 IP" prop="targetIP">
        <el-input
          v-model="formData.targetIP"
          placeholder="例如: 192.168.1.1"
          clearable
        />
      </el-form-item>

      <!-- 端口 -->
      <el-form-item label="端口" prop="targetPort">
        <el-input-number
          v-model="formData.targetPort"
          :min="1"
          :max="65535"
          :step="1"
          controls-position="right"
          class="w-full"
        />
      </el-form-item>

      <!-- 显示名称 -->
      <el-form-item label="显示名称" prop="displayName">
        <el-input
          v-model="formData.displayName"
          placeholder="可选，留空则使用 IP 地址"
          clearable
        />
      </el-form-item>

      <!-- 凭据选择 -->
      <el-form-item label="SNMP 凭据" prop="credentialId">
        <el-select
          v-model="formData.credentialId"
          placeholder="选择凭据"
          clearable
          class="w-full"
        >
          <el-option
            v-for="cred in credentials"
            :key="cred.id"
            :label="`${cred.name} (${cred.version})`"
            :value="cred.id"
          />
        </el-select>
        <div v-if="credentials.length === 0" class="form-tip">
          暂无凭据，请先创建 SNMP 凭据
        </div>
      </el-form-item>

      <!-- 模板选择 -->
      <el-form-item label="采集模板" prop="templateId">
        <el-select
          v-model="formData.templateId"
          placeholder="选择模板"
          clearable
          class="w-full"
        >
          <el-option
            v-for="tpl in templates"
            :key="tpl.id"
            :label="`${tpl.name} (${tpl.category || '未分类'})`"
            :value="tpl.id"
          />
        </el-select>
        <div v-if="templates.length === 0" class="form-tip">
          暂无模板，请先创建采集模板
        </div>
      </el-form-item>

      <!-- 轮询间隔 -->
      <el-form-item label="轮询间隔" prop="pollInterval">
        <el-input-number
          v-model="formData.pollInterval"
          :min="10"
          :max="86400"
          :step="10"
          controls-position="right"
          class="w-full"
        />
        <div class="form-tip">单位：秒（10 ~ 86400）</div>
      </el-form-item>

      <!-- 启用开关 -->
      <el-form-item label="启用">
        <el-switch v-model="formData.enabled" />
      </el-form-item>
    </el-form>

    <!-- 底部按钮 -->
    <div class="form-footer">
      <el-button @click="handleCancel">取消</el-button>
      <el-button type="primary" @click="handleSubmit" :loading="submitting">
        {{ isEditing ? '更新' : '创建' }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import type { Credential, PollingTemplate, CreatePollingTargetRequest, UpdatePollingTargetRequest } from '@/types/snmp'

// Props
interface Props {
  credentials: Credential[]
  templates: PollingTemplate[]
  editingTarget?: {
    id: number
    targetIP: string
    targetPort: number
    displayName: string
    credentialId: number | null
    templateId: number | null
    pollInterval: number
    enabled: boolean
  } | null
  submitting?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  editingTarget: null,
  submitting: false,
})

// Emits
const emit = defineEmits<{
  (e: 'submit', data: CreatePollingTargetRequest | UpdatePollingTargetRequest, isEditing: boolean, id?: number): void
  (e: 'cancel'): void
}>()

// 表单引用
const formRef = ref<FormInstance>()

// 是否编辑模式
const isEditing = computed(() => props.editingTarget !== null)

// 表单数据
const formData = reactive<{
  targetIP: string
  targetPort: number
  displayName: string
  credentialId: number | null
  templateId: number | null
  pollInterval: number
  enabled: boolean
}>({
  targetIP: '',
  targetPort: 161,
  displayName: '',
  credentialId: null,
  templateId: null,
  pollInterval: 60,
  enabled: true,
})

// 表单验证规则
const formRules: FormRules = {
  targetIP: [
    { required: true, message: '请输入目标 IP 地址', trigger: 'blur' },
    {
      pattern: /^(\d{1,3}\.){3}\d{1,3}$/,
      message: '请输入有效的 IP 地址',
      trigger: 'blur',
    },
  ],
  targetPort: [
    { required: true, message: '请输入端口号', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535', trigger: 'blur' },
  ],
  pollInterval: [
    { required: true, message: '请输入轮询间隔', trigger: 'blur' },
    { type: 'number', min: 10, max: 86400, message: '轮询间隔范围 10-86400 秒', trigger: 'blur' },
  ],
}

// 监听编辑目标变化，填充表单
watch(
  () => props.editingTarget,
  (target) => {
    if (target) {
      formData.targetIP = target.targetIP
      formData.targetPort = target.targetPort
      formData.displayName = target.displayName
      formData.credentialId = target.credentialId
      formData.templateId = target.templateId
      formData.pollInterval = target.pollInterval
      formData.enabled = target.enabled
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// 重置表单
function resetForm() {
  formData.targetIP = ''
  formData.targetPort = 161
  formData.displayName = ''
  formData.credentialId = props.credentials[0]?.id ?? null
  formData.templateId = props.templates[0]?.id ?? null
  formData.pollInterval = 60
  formData.enabled = true
  formRef.value?.clearValidate()
}

// 提交表单
async function handleSubmit() {
  if (!formRef.value) return

  try {
    const valid = await formRef.value.validate()
    if (!valid) return
  } catch {
    return
  }

  const requestData = {
    targetIP: formData.targetIP,
    targetPort: formData.targetPort,
    displayName: formData.displayName || formData.targetIP,
    credentialId: formData.credentialId,
    templateId: formData.templateId,
    pollInterval: formData.pollInterval,
    enabled: formData.enabled,
  }

  if (isEditing.value && props.editingTarget) {
    emit('submit', requestData as UpdatePollingTargetRequest, true, props.editingTarget.id)
  } else {
    emit('submit', requestData as CreatePollingTargetRequest, false)
  }
}

// 取消
function handleCancel() {
  emit('cancel')
}
</script>

<style scoped>
.polling-target-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

.w-full {
  width: 100%;
}
</style>