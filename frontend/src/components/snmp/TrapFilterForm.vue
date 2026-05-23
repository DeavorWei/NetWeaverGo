<template>
  <div class="trap-filter-form">
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-width="100px"
      label-position="right"
      size="default"
    >
      <!-- 规则名称 -->
      <el-form-item label="规则名称" prop="name">
        <el-input
          v-model="formData.name"
          placeholder="输入过滤规则名称"
          clearable
        />
      </el-form-item>

      <!-- 启用开关 -->
      <el-form-item label="启用">
        <el-switch v-model="formData.enabled" />
      </el-form-item>

      <!-- 优先级 -->
      <el-form-item label="优先级" prop="priority">
        <el-input-number
          v-model="formData.priority"
          :min="0"
          :max="9999"
          :step="1"
          controls-position="right"
          class="w-full"
        />
        <div class="form-tip">数值越小优先级越高</div>
      </el-form-item>

      <!-- 动作 -->
      <el-form-item label="动作" prop="action">
        <el-select v-model="formData.action" class="w-full" @change="handleActionChange">
          <el-option label="接受 (Accept)" value="accept" />
          <el-option label="丢弃 (Drop)" value="drop" />
          <el-option label="覆盖严重级别 (Severity Override)" value="severity_override" />
        </el-select>
      </el-form-item>

      <!-- 覆盖严重级别（仅 severity_override 时显示） -->
      <el-form-item
        v-if="formData.action === 'severity_override'"
        label="覆盖严重级别"
        prop="overrideSeverity"
      >
        <el-select v-model="formData.overrideSeverity" class="w-full">
          <el-option label="严重 (Critical)" value="critical" />
          <el-option label="重要 (Major)" value="major" />
          <el-option label="次要 (Minor)" value="minor" />
          <el-option label="信息 (Info)" value="info" />
        </el-select>
      </el-form-item>

      <!-- 来源IP模式 -->
      <el-form-item label="来源 IP" prop="sourceIPPattern">
        <el-input
          v-model="formData.sourceIPPattern"
          placeholder="支持 CIDR，如: 192.168.1.0/24 或 10.*"
          clearable
        />
      </el-form-item>

      <!-- OID模式 -->
      <el-form-item label="OID 模式" prop="oidPattern">
        <el-input
          v-model="formData.oidPattern"
          placeholder="OID 匹配模式，如: 1.3.6.1.4.1.*"
          clearable
        />
      </el-form-item>

      <!-- Community模式 -->
      <el-form-item label="Community" prop="communityPattern">
        <el-input
          v-model="formData.communityPattern"
          placeholder="Community 匹配模式"
          clearable
        />
      </el-form-item>

      <!-- 描述 -->
      <el-form-item label="描述">
        <el-input
          v-model="formData.description"
          type="textarea"
          :rows="3"
          placeholder="规则描述（可选）"
        />
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
import type { FilterRule, CreateFilterRuleRequest, UpdateFilterRuleRequest } from '@/types/snmp'

// Props
interface Props {
  editingRule?: FilterRule | null
  submitting?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  editingRule: null,
  submitting: false,
})

// Emits
const emit = defineEmits<{
  (e: 'submit', data: CreateFilterRuleRequest | UpdateFilterRuleRequest, isEditing: boolean, id?: number): void
  (e: 'cancel'): void
}>()

// 表单引用
const formRef = ref<FormInstance>()

// 是否编辑模式
const isEditing = computed(() => props.editingRule !== null)

// 表单数据
const formData = reactive<{
  name: string
  enabled: boolean
  priority: number
  action: string
  sourceIPPattern: string
  oidPattern: string
  communityPattern: string
  overrideSeverity: string
  description: string
}>({
  name: '',
  enabled: true,
  priority: 100,
  action: 'accept',
  sourceIPPattern: '',
  oidPattern: '',
  communityPattern: '',
  overrideSeverity: 'info',
  description: '',
})

// 表单验证规则
const formRules = computed<FormRules>(() => ({
  name: [
    { required: true, message: '请输入规则名称', trigger: 'blur' },
  ],
  priority: [
    { required: true, message: '请输入优先级', trigger: 'blur' },
  ],
  action: [
    { required: true, message: '请选择动作', trigger: 'change' },
  ],
  overrideSeverity: formData.action === 'severity_override'
    ? [{ required: true, message: '请选择覆盖严重级别', trigger: 'change' }]
    : [],
}))

// 监听编辑规则变化，填充表单
watch(
  () => props.editingRule,
  (rule) => {
    if (rule) {
      formData.name = rule.name
      formData.enabled = rule.enabled
      formData.priority = rule.priority
      formData.action = rule.action
      formData.sourceIPPattern = rule.sourceIPPattern
      formData.oidPattern = rule.oidPattern
      formData.communityPattern = rule.communityPattern
      formData.overrideSeverity = rule.overrideSeverity
      formData.description = rule.description
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// 动作切换处理
function handleActionChange(action: string) {
  if (action !== 'severity_override') {
    formData.overrideSeverity = ''
  } else {
    formData.overrideSeverity = 'info'
  }
  formRef.value?.clearValidate('overrideSeverity')
}

// 重置表单
function resetForm() {
  formData.name = ''
  formData.enabled = true
  formData.priority = 100
  formData.action = 'accept'
  formData.sourceIPPattern = ''
  formData.oidPattern = ''
  formData.communityPattern = ''
  formData.overrideSeverity = ''
  formData.description = ''
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
    name: formData.name,
    enabled: formData.enabled,
    priority: formData.priority,
    action: formData.action,
    sourceIPPattern: formData.sourceIPPattern,
    oidPattern: formData.oidPattern,
    communityPattern: formData.communityPattern,
    overrideSeverity: formData.overrideSeverity,
    description: formData.description,
  }

  if (isEditing.value && props.editingRule) {
    emit('submit', requestData as UpdateFilterRuleRequest, true, props.editingRule.id)
  } else {
    emit('submit', requestData as CreateFilterRuleRequest, false)
  }
}

// 取消
function handleCancel() {
  emit('cancel')
}
</script>

<style scoped>
.trap-filter-form {
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