<template>
  <el-dialog
    v-model="visible"
    :title="isEditing ? '编辑 MIB 节点' : '新建 MIB 节点'"
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
      <!-- OID -->
      <el-form-item label="OID" prop="oid">
        <el-input
          v-model="formData.oid"
          placeholder="例如: 1.3.6.1.4.1.9.9.23.1.2.1.1"
          :disabled="isEditing"
          clearable
        />
        <div class="form-tip">OID 格式: 数字点分格式，如 1.3.6.1.4.1.*</div>
      </el-form-item>

      <!-- 名称 -->
      <el-form-item label="名称" prop="name">
        <el-input
          v-model="formData.name"
          placeholder="节点名称，如 cdpInterfaceName"
          clearable
        />
      </el-form-item>

      <!-- 类型 -->
      <el-form-item label="类型" prop="type">
        <el-select v-model="formData.type" class="w-full">
          <el-option label="标量 (Scalar)" value="scalar" />
          <el-option label="表 (Table)" value="table" />
          <el-option label="行 (Row)" value="row" />
          <el-option label="列 (Column)" value="column" />
          <el-option label="通知 (Notification)" value="notification" />
        </el-select>
      </el-form-item>

      <!-- 语法 -->
      <el-form-item label="语法">
        <el-input
          v-model="formData.syntax"
          placeholder="例如: OCTET STRING, INTEGER"
          clearable
        />
      </el-form-item>

      <!-- 访问权限 -->
      <el-form-item label="访问权限">
        <el-select v-model="formData.access" class="w-full" clearable>
          <el-option label="只读 (read-only)" value="read-only" />
          <el-option label="读写 (read-write)" value="read-write" />
          <el-option label="只写 (write-only)" value="write-only" />
          <el-option label="不可访问 (not-accessible)" value="not-accessible" />
        </el-select>
      </el-form-item>

      <!-- 状态 -->
      <el-form-item label="状态">
        <el-select v-model="formData.status" class="w-full" clearable>
          <el-option label="当前 (current)" value="current" />
          <el-option label="已弃用 (deprecated)" value="deprecated" />
          <el-option label="已废弃 (obsolete)" value="obsolete" />
        </el-select>
      </el-form-item>

      <!-- 描述 -->
      <el-form-item label="描述">
        <el-input
          v-model="formData.description"
          type="textarea"
          :rows="3"
          placeholder="节点描述（可选）"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" @click="handleSubmit" :loading="submitting">
        {{ isEditing ? '更新' : '创建' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import type { MIBNode, CreateMIBNodeRequest, UpdateMIBNodeRequest } from '@/types/snmp'

// Props
interface Props {
  modelValue: boolean
  editingNode?: MIBNode | null
  moduleId?: number | null
  submitting?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  editingNode: null,
  moduleId: null,
  submitting: false,
})

// Emits
const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'submit', data: CreateMIBNodeRequest | UpdateMIBNodeRequest, isEditing: boolean, id?: number): void
}>()

// 表单引用
const formRef = ref<FormInstance>()

// 对话框可见性
const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
})

// 是否编辑模式
const isEditing = computed(() => props.editingNode !== null)

// 表单数据
const formData = reactive<{
  oid: string
  name: string
  type: string
  syntax: string
  access: string
  status: string
  description: string
}>({
  oid: '',
  name: '',
  type: 'scalar',
  syntax: '',
  access: 'read-only',
  status: 'current',
  description: '',
})

// 表单验证规则
const formRules: FormRules = {
  oid: [
    { required: true, message: '请输入 OID', trigger: 'blur' },
    {
      pattern: /^(\d+\.)*\d+$/,
      message: 'OID 格式无效，应为数字点分格式',
      trigger: 'blur',
    },
  ],
  name: [
    { required: true, message: '请输入节点名称', trigger: 'blur' },
    {
      pattern: /^[a-zA-Z][a-zA-Z0-9_-]*$/,
      message: '名称应以字母开头，只能包含字母、数字、下划线和连字符',
      trigger: 'blur',
    },
  ],
  type: [
    { required: true, message: '请选择类型', trigger: 'change' },
  ],
}

// 监听编辑节点变化，填充表单
watch(
  () => props.editingNode,
  (node) => {
    if (node) {
      formData.oid = node.oid
      formData.name = node.name
      formData.type = node.type || 'scalar'
      formData.syntax = node.description || ''
      formData.access = node.access || 'read-only'
      formData.status = node.status || 'current'
      formData.description = node.description || ''
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// 重置表单
function resetForm() {
  formData.oid = ''
  formData.name = ''
  formData.type = 'scalar'
  formData.syntax = ''
  formData.access = 'read-only'
  formData.status = 'current'
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

  if (isEditing.value && props.editingNode) {
    const requestData: UpdateMIBNodeRequest = {
      name: formData.name,
      description: formData.description,
      type: formData.type,
      access: formData.access,
      status: formData.status,
    }
    emit('submit', requestData, true, props.editingNode.id)
  } else {
    if (!props.moduleId) {
      return
    }
    const requestData: CreateMIBNodeRequest = {
      moduleId: props.moduleId,
      oid: formData.oid,
      name: formData.name,
      description: formData.description,
      type: formData.type,
      access: formData.access,
      status: formData.status,
      parentId: null,
    }
    emit('submit', requestData, false)
  }
}

// 关闭对话框
function handleClose() {
  formRef.value?.resetFields()
  visible.value = false
}
</script>

<style scoped>
.form-tip {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 4px;
}

.w-full {
  width: 100%;
}
</style>