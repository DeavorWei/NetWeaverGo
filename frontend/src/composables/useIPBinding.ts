import { ref, watch, computed, type Ref } from 'vue'
import { ForgeAPI, DeviceAPI } from '../services/api'
import type { VarInput } from '../services/api'

/**
 * IP 绑定相关逻辑
 * 处理 IP 验证、绑定预览等功能
 */
export function useIPBinding(
  variables: Ref<VarInput[]>,
  templateText: Ref<string>,
) {
  // IP绑定模式开关
  const ipBindingEnabled = ref(false)

  // IP 列表验证结果缓存
  const ipValidationCache = ref<Map<string, { isValid: boolean }>>(new Map())

  // 获取无效的 IP 列表
  const invalidIPs = computed(() => {
    if (!ipBindingEnabled.value || variables.value.length === 0) return []
    const firstVar = variables.value[0]
    if (!firstVar) return []

    const ipValues = firstVar.valueString
      .split(/,|\n/)
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '')

    return ipValues.filter((ip) => {
      const cached = ipValidationCache.value.get(ip)
      return cached && !cached.isValid
    })
  })

  const hasInvalidIP = computed(() => invalidIPs.value.length > 0)

  // 更新 IP 验证缓存
  watch(
    () => variables.value[0]?.valueString,
    async (newValue) => {
      if (!ipBindingEnabled.value || !newValue) return

      const ipValues = newValue
        .split(/,|\n/)
        .map((s: string) => s.trim())
        .filter((s: string) => s !== '')

      for (const ip of ipValues) {
        if (!ipValidationCache.value.has(ip)) {
          try {
            const result = await ForgeAPI.validateIP(ip)
            if (result) {
              ipValidationCache.value.set(ip, result)
            }
          } catch {
            // ignore
          }
        }
      }
    },
    { immediate: true },
  )

  // 绑定预览
  const bindingPreview = ref<{ ip: string; commands: string }[]>([])

  watch(
    [ipBindingEnabled, variables, templateText],
    async () => {
      if (!ipBindingEnabled.value || variables.value.length === 0) {
        bindingPreview.value = []
        return
      }

      try {
        const result = await ForgeAPI.generateBindingPreview(
          templateText.value,
          variables.value,
          ipBindingEnabled.value,
        )
        bindingPreview.value = result
      } catch (err) {
        console.error('生成绑定预览失败:', err)
        bindingPreview.value = []
      }
    },
    { deep: true },
  )

  // 关闭 IP 绑定（当删除第一个变量时调用）
  const disableIPBinding = () => {
    ipBindingEnabled.value = false
  }

  // 获取设备映射 (IP -> ID)
  const getDeviceMap = async () => {
    const devices = await DeviceAPI.listDevices()
    return new Map(devices.map((d: any) => [d.ip, d.id]))
  }

  return {
    // 状态
    ipBindingEnabled,
    ipValidationCache,
    invalidIPs,
    hasInvalidIP,
    bindingPreview,
    // 方法
    disableIPBinding,
    getDeviceMap,
  }
}
