<template>
  <div class="polling-template-form">
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-width="100px"
      label-position="right"
      size="default"
    >
      <!-- 名称 -->
      <el-form-item label="名称" prop="name">
        <el-input
          v-model="formData.name"
          placeholder="例如: 接口监控模板"
          clearable
        />
      </el-form-item>

      <!-- 描述 -->
      <el-form-item label="描述">
        <el-input
          v-model="formData.description"
          placeholder="模板描述（可选）"
          clearable
        />
      </el-form-item>

      <!-- 类别 -->
      <el-form-item label="类别" prop="category">
        <el-select v-model="formData.category" class="w-full" @change="handleCategoryChange">
          <el-option label="系统 (System)" value="system" />
          <el-option label="接口 (Interface)" value="interface" />
          <el-option label="CPU" value="cpu" />
          <el-option label="内存 (Memory)" value="memory" />
          <el-option label="存储 (Storage)" value="storage" />
          <el-option label="自定义 (Custom)" value="custom" />
        </el-select>
      </el-form-item>

      <!-- OID 项列表 -->
      <el-form-item label="OID 项列表" prop="oidItems">
        <div class="oid-items-container">
          <div
            v-for="(item, index) in formData.oidItems"
            :key="index"
            class="oid-item-row"
          >
            <div class="oid-item-fields">
              <el-input
                v-model="item.oid"
                placeholder="OID，如 1.3.6.1.2.1.1.1"
                size="small"
                class="oid-field"
              />
              <el-input
                v-model="item.name"
                placeholder="名称"
                size="small"
                class="name-field"
              />
              <el-select
                v-model="item.type"
                placeholder="类型"
                size="small"
                class="type-field"
                clearable
              >
                <el-option label="String" value="string" />
                <el-option label="Integer" value="integer" />
                <el-option label="Gauge" value="gauge" />
                <el-option label="Counter" value="counter" />
                <el-option label="TimeTicks" value="timeticks" />
              </el-select>
              <el-select
                v-model="item.operation"
                placeholder="操作"
                size="small"
                class="operation-field"
              >
                <el-option label="GET" value="get" />
                <el-option label="WALK" value="walk" />
                <el-option label="BULK" value="bulk" />
              </el-select>
              <el-input
                v-model="item.description"
                placeholder="描述"
                size="small"
                class="desc-field"
              />
            </div>
            <el-button
              type="danger"
              size="small"
              link
              @click="removeOIDItem(index)"
            >
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>

          <!-- 添加 OID 按钮 -->
          <el-button
            type="primary"
            size="small"
            plain
            @click="addOIDItem"
            class="add-oid-btn"
          >
            <el-icon><Plus /></el-icon>
            添加 OID
          </el-button>

          <div v-if="formData.oidItems.length === 0" class="oid-empty">
            暂无 OID 项，请添加或选择类别预设
          </div>
        </div>
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
import { Delete, Plus } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import type { OIDItem, PollingTemplate, CreatePollingTemplateRequest, UpdatePollingTemplateRequest } from '@/types/snmp'

// Props
interface Props {
  editingTemplate?: PollingTemplate | null
  submitting?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  editingTemplate: null,
  submitting: false,
})

// Emits
const emit = defineEmits<{
  (e: 'submit', data: CreatePollingTemplateRequest | UpdatePollingTemplateRequest, isEditing: boolean, id?: number): void
  (e: 'cancel'): void
}>()

// 表单引用
const formRef = ref<FormInstance>()

// 是否编辑模式
const isEditing = computed(() => props.editingTemplate !== null)

// 类别预设 OID 项
const CATEGORY_PRESETS: Record<string, OIDItem[]> = {
  system: [
    { oid: '1.3.6.1.2.1.1.1', name: 'sysDescr', type: 'string', operation: 'get', description: '系统描述' },
    { oid: '1.3.6.1.2.1.1.3', name: 'sysUpTime', type: 'timeticks', operation: 'get', description: '系统运行时间' },
    { oid: '1.3.6.1.2.1.1.5', name: 'sysName', type: 'string', operation: 'get', description: '系统名称' },
    { oid: '1.3.6.1.2.1.1.6', name: 'sysLocation', type: 'string', operation: 'get', description: '系统位置' },
  ],
  interface: [
    { oid: '1.3.6.1.2.1.2.2.1.1', name: 'ifIndex', type: 'integer', operation: 'walk', description: '接口索引' },
    { oid: '1.3.6.1.2.1.2.2.1.2', name: 'ifDescr', type: 'string', operation: 'walk', description: '接口描述' },
    { oid: '1.3.6.1.2.1.2.2.1.5', name: 'ifSpeed', type: 'gauge', operation: 'walk', description: '接口速率' },
    { oid: '1.3.6.1.2.1.2.2.1.10', name: 'ifInOctets', type: 'counter', operation: 'walk', description: '入站字节数' },
    { oid: '1.3.6.1.2.1.2.2.1.16', name: 'ifOutOctets', type: 'counter', operation: 'walk', description: '出站字节数' },
    { oid: '1.3.6.1.2.1.2.2.1.8', name: 'ifOperStatus', type: 'integer', operation: 'walk', description: '接口操作状态' },
  ],
  cpu: [
    { oid: '1.3.6.1.4.1.9.9.109.1.1.1.1.8', name: 'cpmCPUTotal5minRev', type: 'gauge', operation: 'get', description: 'CPU 5分钟平均利用率' },
  ],
  memory: [
    { oid: '1.3.6.1.4.1.9.9.48.1.1.1.5', name: 'ciscoMemoryPoolUsed', type: 'gauge', operation: 'walk', description: '已用内存' },
    { oid: '1.3.6.1.4.1.9.9.48.1.1.1.6', name: 'ciscoMemoryPoolFree', type: 'gauge', operation: 'walk', description: '空闲内存' },
  ],
  storage: [
    { oid: '1.3.6.1.2.1.25.2.3.1.1', name: 'hrStorageIndex', type: 'integer', operation: 'walk', description: '存储索引' },
    { oid: '1.3.6.1.2.1.25.2.3.1.2', name: 'hrStorageType', type: 'string', operation: 'walk', description: '存储类型' },
    { oid: '1.3.6.1.2.1.25.2.3.1.5', name: 'hrStorageSize', type: 'gauge', operation: 'walk', description: '存储大小' },
    { oid: '1.3.6.1.2.1.25.2.3.1.6', name: 'hrStorageUsed', type: 'gauge', operation: 'walk', description: '已用存储' },
  ],
  custom: [],
}

// 表单数据
const formData = reactive<{
  name: string
  description: string
  category: string
  oidItems: OIDItem[]
}>({
  name: '',
  description: '',
  category: '',
  oidItems: [],
})

// 表单验证规则
const formRules: FormRules = {
  name: [
    { required: true, message: '请输入模板名称', trigger: 'blur' },
  ],
  category: [
    { required: true, message: '请选择类别', trigger: 'change' },
  ],
}

// 监听编辑模板变化，填充表单
watch(
  () => props.editingTemplate,
  (template) => {
    if (template) {
      formData.name = template.name
      formData.description = template.description
      formData.category = template.category
      formData.oidItems = [...template.oidItems.map(item => ({ ...item }))]
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// 类别切换处理
function handleCategoryChange(category: string) {
  // 仅在新建模式且 OID 列表为空时自动填充预设
  if (!isEditing.value && formData.oidItems.length === 0) {
    const preset = CATEGORY_PRESETS[category]
    if (preset) {
      formData.oidItems = preset.map(item => ({ ...item }))
    }
  }
}

// 添加 OID 项
function addOIDItem() {
  formData.oidItems.push({
    oid: '',
    name: '',
    type: '',
    operation: 'get',
    description: '',
  })
}

// 移除 OID 项
function removeOIDItem(index: number) {
  formData.oidItems.splice(index, 1)
}

// 重置表单
function resetForm() {
  formData.name = ''
  formData.description = ''
  formData.category = ''
  formData.oidItems = []
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

  // 验证 OID 项
  const hasEmptyOID = formData.oidItems.some(item => !item.oid.trim())
  if (formData.oidItems.length > 0 && hasEmptyOID) {
    return
  }

  const requestData = {
    name: formData.name,
    description: formData.description,
    category: formData.category,
    oidItems: formData.oidItems.filter(item => item.oid.trim()),
  }

  if (isEditing.value && props.editingTemplate) {
    emit('submit', requestData as UpdatePollingTemplateRequest, true, props.editingTemplate.id)
  } else {
    emit('submit', requestData as CreatePollingTemplateRequest, false)
  }
}

// 取消
function handleCancel() {
  emit('cancel')
}
</script>

<style scoped>
.polling-template-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.oid-items-container {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.oid-item-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  background-color: var(--color-bg-tertiary);
  border-radius: 6px;
  border: 1px solid var(--color-border-default);
}

.oid-item-fields {
  flex: 1;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.oid-field {
  flex: 2;
  min-width: 140px;
}

.name-field {
  flex: 1;
  min-width: 80px;
}

.type-field {
  width: 100px;
}

.operation-field {
  width: 90px;
}

.desc-field {
  flex: 1;
  min-width: 80px;
}

.add-oid-btn {
  align-self: flex-start;
}

.oid-empty {
  text-align: center;
  padding: 12px;
  color: var(--color-text-muted);
  font-size: 12px;
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 16px;
  border-top: 1px solid var(--color-border-default);
}

.w-full {
  width: 100%;
}
</style>