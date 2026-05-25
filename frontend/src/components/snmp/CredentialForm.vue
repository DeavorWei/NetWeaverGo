<template>
  <div class="credential-form">
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-width="110px"
      label-position="right"
      size="default"
    >
      <!-- 名称 -->
      <el-form-item label="名称" prop="name">
        <el-input
          v-model="formData.name"
          placeholder="例如: 交换机凭据"
          clearable
        />
      </el-form-item>

      <!-- SNMP 版本 -->
      <el-form-item label="SNMP 版本" prop="version">
        <el-radio-group v-model="formData.version" @change="handleVersionChange">
          <el-radio value="v1">v1</el-radio>
          <el-radio value="v2c">v2c</el-radio>
          <el-radio value="v3">v3</el-radio>
        </el-radio-group>
      </el-form-item>

      <!-- v1/v2c: Community 字符串 -->
      <template v-if="formData.version === 'v1' || formData.version === 'v2c'">
        <el-form-item label="Community" prop="community">
          <el-input
            v-model="formData.community"
            placeholder="例如: public"
            show-password
            clearable
          />
        </el-form-item>
      </template>

      <!-- v3: 安全级别 -->
      <template v-if="formData.version === 'v3'">
        <el-form-item label="安全级别" prop="securityLevel">
          <el-select v-model="formData.securityLevel" class="w-full" @change="handleSecurityLevelChange">
            <el-option
              v-for="opt in securityLevelOptions"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-select>
        </el-form-item>

        <!-- v3: 用户名 -->
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="formData.username"
            placeholder="SNMPv3 用户名"
            clearable
          />
        </el-form-item>

        <!-- v3: 认证协议和密码（authNoPriv / authPriv） -->
        <template v-if="formData.securityLevel === 'authNoPriv' || formData.securityLevel === 'authPriv'">
          <el-form-item label="认证协议" prop="authProtocol">
            <el-select v-model="formData.authProtocol" class="w-full">
              <el-option
                v-for="opt in authProtocolOptions"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="认证密码" prop="authPassword">
            <el-input
              v-model="formData.authPassword"
              :placeholder="isEditing ? '留空保持原密码' : '请输入认证密码'"
              show-password
              clearable
            />
          </el-form-item>
        </template>

        <!-- v3: 加密协议和密码（authPriv） -->
        <template v-if="formData.securityLevel === 'authPriv'">
          <el-form-item label="加密协议" prop="privProtocol">
            <el-select v-model="formData.privProtocol" class="w-full">
              <el-option
                v-for="opt in privProtocolOptions"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="加密密码" prop="privPassword">
            <el-input
              v-model="formData.privPassword"
              :placeholder="isEditing ? '留空保持原密码' : '请输入加密密码'"
              show-password
              clearable
            />
          </el-form-item>
        </template>

        <!-- v3: 上下文引擎ID -->
        <el-form-item label="上下文引擎ID">
          <el-input
            v-model="formData.contextEngineId"
            placeholder="可选"
            clearable
          />
        </el-form-item>

        <!-- v3: 上下文名称 -->
        <el-form-item label="上下文名称">
          <el-input
            v-model="formData.contextName"
            placeholder="可选"
            clearable
          />
        </el-form-item>
      </template>
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
import {
  V3_AUTH_PROTOCOLS,
  V3_PRIV_PROTOCOLS,
  V3_SECURITY_LEVELS,
} from '@/types/snmp'
import type { CreateCredentialRequest, UpdateCredentialRequest, Credential } from '@/types/snmp'

// Props
interface Props {
  editingCredential?: Credential | null
  submitting?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  editingCredential: null,
  submitting: false,
})

// Emits
const emit = defineEmits<{
  (e: 'submit', data: CreateCredentialRequest | UpdateCredentialRequest, isEditing: boolean, id?: number): void
  (e: 'cancel'): void
}>()

// 表单引用
const formRef = ref<FormInstance>()

// 是否编辑模式
const isEditing = computed(() => props.editingCredential !== null)

// 选项列表
const securityLevelOptions = V3_SECURITY_LEVELS
const authProtocolOptions = V3_AUTH_PROTOCOLS
const privProtocolOptions = V3_PRIV_PROTOCOLS

// 表单数据
const formData = reactive<{
  name: string
  version: 'v1' | 'v2c' | 'v3'
  community: string
  securityLevel: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv'
  username: string
  authProtocol: string
  authPassword: string
  privProtocol: string
  privPassword: string
  contextName: string
  contextEngineId: string
}>({
  name: '',
  version: 'v2c',
  community: 'public',
  securityLevel: 'noAuthNoPriv',
  username: '',
  authProtocol: 'MD5',
  authPassword: '',
  privProtocol: 'AES',
  privPassword: '',
  contextName: '',
  contextEngineId: '',
})

// 表单验证规则
const formRules = computed<FormRules>(() => ({
  name: [
    { required: true, message: '请输入凭据名称', trigger: 'blur' },
  ],
  version: [
    { required: true, message: '请选择 SNMP 版本', trigger: 'change' },
  ],
  community: formData.version === 'v1' || formData.version === 'v2c'
    ? [{ required: true, message: '请输入 Community 字符串', trigger: 'blur' }]
    : [],
  securityLevel: formData.version === 'v3'
    ? [{ required: true, message: '请选择安全级别', trigger: 'change' }]
    : [],
  username: formData.version === 'v3'
    ? [{ required: true, message: '请输入用户名', trigger: 'blur' }]
    : [],
  authPassword: formData.version === 'v3' && formData.securityLevel !== 'noAuthNoPriv' && !isEditing.value
    ? [{ required: true, message: '请输入认证密码', trigger: 'blur' }]
    : [],
  privPassword: formData.version === 'v3' && formData.securityLevel === 'authPriv' && !isEditing.value
    ? [{ required: true, message: '请输入加密密码', trigger: 'blur' }]
    : [],
}))

// 监听编辑凭据变化，填充表单
watch(
  () => props.editingCredential,
  (credential) => {
    if (credential) {
      formData.name = credential.name
      formData.version = credential.version
      formData.community = credential.community || 'public'
      formData.securityLevel = credential.securityLevel || 'noAuthNoPriv'
      formData.username = credential.username || ''
      formData.authProtocol = credential.authProtocol || 'MD5'
      formData.authPassword = '' // 编辑时不回显密码
      formData.privProtocol = credential.privProtocol || 'AES'
      formData.privPassword = '' // 编辑时不回显密码
      formData.contextName = credential.contextName || ''
      formData.contextEngineId = credential.contextEngineId || ''
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// 版本切换处理
function handleVersionChange(version: 'v1' | 'v2c' | 'v3') {
  if (version === 'v3') {
    formData.community = ''
    formData.securityLevel = 'noAuthNoPriv'
  } else {
    formData.community = formData.community || 'public'
    formData.securityLevel = 'noAuthNoPriv'
    formData.username = ''
    formData.authPassword = ''
    formData.privPassword = ''
    formData.contextName = ''
    formData.contextEngineId = ''
  }
  formRef.value?.clearValidate()
}

// 安全级别切换处理
function handleSecurityLevelChange(level: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv') {
  if (level === 'noAuthNoPriv') {
    formData.authPassword = ''
    formData.privPassword = ''
  } else if (level === 'authNoPriv') {
    formData.privPassword = ''
  }
  formRef.value?.clearValidate()
}

// 重置表单
function resetForm() {
  formData.name = ''
  formData.version = 'v2c'
  formData.community = 'public'
  formData.securityLevel = 'noAuthNoPriv'
  formData.username = ''
  formData.authProtocol = 'MD5'
  formData.authPassword = ''
  formData.privProtocol = 'AES'
  formData.privPassword = ''
  formData.contextName = ''
  formData.contextEngineId = ''
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

  const baseData = {
    name: formData.name,
    version: formData.version,
  }

  if (formData.version === 'v1' || formData.version === 'v2c') {
    const requestData = {
      ...baseData,
      community: formData.community,
    }
    if (isEditing.value && props.editingCredential) {
      emit('submit', requestData as UpdateCredentialRequest, true, props.editingCredential.id)
    } else {
      emit('submit', requestData as CreateCredentialRequest, false)
    }
  } else {
    // v3
    const requestData = {
      ...baseData,
      securityLevel: formData.securityLevel,
      username: formData.username,
      authProtocol: formData.securityLevel !== 'noAuthNoPriv' ? formData.authProtocol : undefined,
      authPassword: formData.securityLevel !== 'noAuthNoPriv' ? formData.authPassword : undefined,
      privProtocol: formData.securityLevel === 'authPriv' ? formData.privProtocol : undefined,
      privPassword: formData.securityLevel === 'authPriv' ? formData.privPassword : undefined,
      contextName: formData.contextName || undefined,
      contextEngineId: formData.contextEngineId || undefined,
    }
    if (isEditing.value && props.editingCredential) {
      emit('submit', requestData as UpdateCredentialRequest, true, props.editingCredential.id)
    } else {
      emit('submit', requestData as CreateCredentialRequest, false)
    }
  }
}

// 取消
function handleCancel() {
  emit('cancel')
}
</script>

<style scoped>
.credential-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
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