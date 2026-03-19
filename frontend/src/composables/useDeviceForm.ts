/**
 * 设备表单逻辑 Composable
 * 
 * 提供设备新增/编辑表单的状态和验证逻辑
 */

import { ref, watch } from 'vue'

// 表单数据类型
export interface DeviceFormData {
  ip: string
  port: number
  protocol: string
  username: string
  password: string
  group: string
  tags: string[]
  vendor: string
  role: string
  site: string
  displayName: string
  description: string
}

// IP 范围提示类型
export interface IpRangeHint {
  count: number
  start: string
  end: string
}

// 协议默认端口
const DEFAULT_PORTS: Record<string, number> = {
  SSH: 22,
  SNMP: 161,
  TELNET: 23,
}

// 有效协议列表
const DEFAULT_PROTOCOLS = ['SSH', 'SNMP', 'TELNET']

/**
 * 创建默认表单数据
 */
function createDefaultForm(): DeviceFormData {
  return {
    ip: '',
    port: 22,
    protocol: 'SSH',
    username: '',
    password: '',
    group: '',
    tags: [],
    vendor: '',
    role: '',
    site: '',
    displayName: '',
    description: '',
  }
}

/**
 * 设备表单 Hook
 */
export function useDeviceForm() {
  // 表单数据
  const form = ref<DeviceFormData>(createDefaultForm())
  
  // 密码显示状态
  const showPassword = ref(false)
  
  // 错误消息
  const errorMessage = ref('')
  
  // IP 验证错误
  const ipValidationError = ref('')
  
  // IP 范围提示
  const ipRangeHint = ref<IpRangeHint | null>(null)
  
  // 新标签输入
  const newTag = ref('')
  
  // 上次的协议（用于判断端口是否需要自动更新）
  const lastProtocol = ref('SSH')
  
  // 协议默认端口配置
  const protocolDefaultPorts = ref<Record<string, number>>({ ...DEFAULT_PORTS })
  
  // 有效协议列表
  const validProtocols = ref<string[]>([...DEFAULT_PROTOCOLS])
  
  // ==================== IP 验证相关 ====================
  
  /**
   * 检查是否为有效 IP 地址
   */
  function isValidIp(ip: string): boolean {
    const parts = ip.split('.')
    if (parts.length !== 4) return false
    return parts.every((part) => {
      if (part === '' || part.length > 3) return false
      const num = parseInt(part, 10)
      return !isNaN(num) && num >= 0 && num <= 255 && part === num.toString()
    })
  }
  
  /**
   * 解析 IP 范围语法
   * 支持格式: 192.168.1.10-20
   */
  function parseIpRange(ip: string): IpRangeHint | null {
    if (!ip) return null
    
    const match = ip.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/)
    if (match && match[1] && match[2] && match[3]) {
      const prefix = match[1]
      const start = parseInt(match[2], 10)
      const end = parseInt(match[3], 10)
      
      if (start < end && start >= 0 && end <= 255) {
        return {
          count: end - start + 1,
          start: prefix + start,
          end: prefix + end,
        }
      }
    }
    
    return null
  }
  
  /**
   * 验证 IP 输入
   */
  function validateIpInput(ip: string): { valid: boolean; error: string } {
    if (!ip) return { valid: false, error: '' }
    
    if (isValidIp(ip)) {
      return { valid: true, error: '' }
    }
    
    const rangeHint = parseIpRange(ip)
    if (rangeHint) {
      return { valid: true, error: '' }
    }
    
    const rangeMatch = ip.match(
      /^(\d{1,3}\.\d{1,3}\.\d{1,3}\.)(\d{1,3})-(\d{1,3})$/
    )
    if (rangeMatch && rangeMatch[2] && rangeMatch[3]) {
      const start = parseInt(rangeMatch[2], 10)
      const end = parseInt(rangeMatch[3], 10)
      if (start > 255 || end > 255) {
        return { valid: false, error: 'IP 段值必须在 0-255 范围内' }
      }
      if (start >= end) {
        return { valid: false, error: '起始值必须小于结束值' }
      }
    }
    
    return {
      valid: false,
      error: '请输入有效 IP（如 192.168.1.10）或范围格式（如 192.168.1.10-20）',
    }
  }
  
  // 监听 IP 输入，解析语法糖并验证
  watch(
    () => form.value.ip,
    (newIp) => {
      ipRangeHint.value = parseIpRange(newIp)
      const validation = validateIpInput(newIp)
      ipValidationError.value = validation.error
    }
  )
  
  // ==================== 协议相关 ====================
  
  /**
   * 协议变更处理
   */
  function onProtocolChange() {
    const oldDefaultPort = protocolDefaultPorts.value[lastProtocol.value] || 22
    const newDefaultPort = protocolDefaultPorts.value[form.value.protocol] || 22
    
    // 如果当前端口是旧协议的默认端口，则自动更新为新协议的默认端口
    if (form.value.port === oldDefaultPort) {
      form.value.port = newDefaultPort
    }
    
    lastProtocol.value = form.value.protocol
  }
  
  /**
   * 获取协议徽章样式
   */
  function getProtocolBadgeClass(protocol: string): string {
    const classes: Record<string, string> = {
      SSH: 'bg-success-bg text-success',
      SNMP: 'bg-info-bg text-info',
      TELNET: 'bg-warning-bg text-warning',
    }
    return classes[protocol] || 'bg-bg-hover text-text-muted'
  }
  
  // ==================== 标签相关 ====================
  
  /**
   * 添加标签
   */
  function addTag() {
    const tag = newTag.value.trim()
    if (tag && !form.value.tags.includes(tag)) {
      form.value.tags.push(tag)
    }
    newTag.value = ''
  }
  
  /**
   * 移除标签
   */
  function removeTag(index: number) {
    form.value.tags.splice(index, 1)
  }
  
  // ==================== 表单操作 ====================
  
  /**
   * 重置表单
   */
  function resetForm() {
    form.value = createDefaultForm()
    newTag.value = ''
    lastProtocol.value = 'SSH'
    errorMessage.value = ''
    showPassword.value = false
    ipRangeHint.value = null
    ipValidationError.value = ''
  }
  
  /**
   * 设置表单数据（用于编辑）
   */
  function setFormData(data: Partial<DeviceFormData>) {
    form.value = {
      ...createDefaultForm(),
      ...data,
      tags: data.tags ? [...data.tags] : [],
    }
    lastProtocol.value = data.protocol || 'SSH'
  }
  
  /**
   * 更新协议配置
   */
  function updateProtocolConfig(ports: Record<string, number>, protocols: string[]) {
    if (ports) {
      const normalized: Record<string, number> = {}
      Object.entries(ports).forEach(([key, value]) => {
        if (typeof value === 'number') {
          normalized[key] = value
        }
      })
      protocolDefaultPorts.value = normalized
    }
    if (protocols) {
      validProtocols.value = protocols
    }
  }

  return {
    // 状态
    form,
    showPassword,
    errorMessage,
    ipValidationError,
    ipRangeHint,
    newTag,
    lastProtocol,
    protocolDefaultPorts,
    validProtocols,
    
    // IP 验证
    isValidIp,
    parseIpRange,
    validateIpInput,
    
    // 协议相关
    onProtocolChange,
    getProtocolBadgeClass,
    
    // 标签相关
    addTag,
    removeTag,
    
    // 表单操作
    resetForm,
    setFormData,
    updateProtocolConfig,
  }
}
