/**
 * 主题管理 Composable
 * Theme Management Composable
 * 
 * 提供统一的主题管理功能，包括：
 * - 主题初始化（从后端加载）
 * - 主题切换
 * - 主题持久化（同步到后端数据库）
 * - 系统主题跟随
 * 
 * 数据流设计：
 * 1. index.html 早期脚本从 localStorage 快速读取主题（防止闪烁）
 * 2. Vue 应用初始化后，从后端 GlobalSettings.theme 获取权威主题值
 * 3. 用户切换主题时，同时更新 localStorage 和后端数据库
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  type ThemeName,
  type ThemeConfig,
  AVAILABLE_THEMES,
  DEFAULT_THEME,
  THEME_STORAGE_KEY,
} from '@/types/theme'
import { SettingsAPI } from '@/services/api'
import { getLogger } from '@/utils/logger'

const logger = getLogger()

// 全局状态（单例模式）
const currentTheme = ref<ThemeName>(DEFAULT_THEME)
const isInitialized = ref(false)
const isLoadingFromBackend = ref(false)

// 系统主题变化监听器
let systemThemeListener: ((e: MediaQueryListEvent) => void) | null = null

/**
 * 解析主题值
 * - "light" -> "light"
 * - "dark" -> "dark"  
 * - "system" -> 根据系统偏好返回 "light" 或 "dark"
 */
const resolveThemeValue = (theme: string): ThemeName => {
  if (theme === 'light' || theme === 'dark') {
    return theme
  }
  // "system" 或其他值：跟随系统偏好
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/**
 * 主题管理 Hook
 */
export function useTheme() {
  /**
   * 验证主题名称是否有效
   */
  const isValidTheme = (theme: string): boolean => {
    return AVAILABLE_THEMES.some((t: ThemeConfig) => t.name === theme)
  }

  /**
   * 应用主题到 DOM
   */
  const applyTheme = (theme: ThemeName): void => {
    const html = document.documentElement

    // 移除旧主题属性
    html.removeAttribute('data-theme')
    html.classList.remove('dark', 'light')

    // 应用新主题
    html.setAttribute('data-theme', theme)
    if (theme === 'dark') {
      html.classList.add('dark')
    }

    // 更新 meta theme-color（移动端浏览器地址栏颜色）
    const metaThemeColor = document.querySelector('meta[name="theme-color"]')
    if (metaThemeColor) {
      // 从 CSS 变量读取背景色，确保与主题一致
      const bgColor = getComputedStyle(document.documentElement)
        .getPropertyValue('--color-bg-primary').trim()
      metaThemeColor.setAttribute('content', bgColor)
    }

    // 更新 CSS 自定义属性供 Tailwind 使用
    // 这确保与 Tailwind 的 dark: 变体兼容
  }

  /**
   * 从后端加载主题设置
   * 优先级：后端数据库 > localStorage > 系统偏好 > 默认主题
   */
  const loadThemeFromBackend = async (): Promise<ThemeName> => {
    try {
      isLoadingFromBackend.value = true
      const settings = await SettingsAPI.loadSettings()
      
      if (settings && settings.theme) {
        // 后端有主题设置，解析并返回
        return resolveThemeValue(settings.theme)
      }
    } catch (error) {
      logger.warn('Failed to load theme from backend', 'useTheme')
    } finally {
      isLoadingFromBackend.value = false
    }

    // 后端无主题设置或加载失败，回退到 localStorage
    const savedTheme = localStorage.getItem(THEME_STORAGE_KEY) as ThemeName | null
    if (savedTheme && isValidTheme(savedTheme)) {
      return savedTheme
    }

    // 最后回退到系统偏好
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }

  /**
   * 保存主题到后端
   */
  const saveThemeToBackend = async (theme: ThemeName): Promise<void> => {
    try {
      const settings = await SettingsAPI.loadSettings()
      if (settings) {
        // 更新主题字段并保存
        settings.theme = theme
        await SettingsAPI.saveSettings(settings)
      }
    } catch (error) {
      logger.warn('Failed to save theme to backend', 'useTheme')
    }
  }

  /**
   * 初始化主题（异步）
   * 优先级：后端数据库 > localStorage > 系统偏好 > 默认主题
   */
  const initTheme = async (): Promise<void> => {
    if (isInitialized.value) return

    // 先从 localStorage 快速读取，立即应用（防止闪烁）
    const savedTheme = localStorage.getItem(THEME_STORAGE_KEY) as ThemeName | null
    if (savedTheme && isValidTheme(savedTheme)) {
      currentTheme.value = savedTheme
      applyTheme(currentTheme.value)
    }

    // 然后从后端加载权威值
    const backendTheme = await loadThemeFromBackend()
    
    // 如果后端值与当前值不同，更新并应用
    if (backendTheme !== currentTheme.value) {
      currentTheme.value = backendTheme
      applyTheme(currentTheme.value)
      // 同步到 localStorage
      localStorage.setItem(THEME_STORAGE_KEY, backendTheme)
    }

    isInitialized.value = true
  }

  /**
   * 设置主题
   * 同时更新 localStorage 和后端数据库
   */
  const setTheme = (theme: ThemeName): void => {
    if (!isValidTheme(theme)) {
      logger.warn(`Invalid theme: ${theme}`, 'useTheme')
      return
    }

    currentTheme.value = theme
    localStorage.setItem(THEME_STORAGE_KEY, theme)
    applyTheme(theme)

    // 异步保存到后端（不阻塞 UI）
    saveThemeToBackend(theme)
  }

  /**
   * 切换主题（明暗切换）
   */
  const toggleTheme = (): void => {
    setTheme(currentTheme.value === 'dark' ? 'light' : 'dark')
  }

  /**
   * 获取系统偏好主题
   */
  const getSystemTheme = (): ThemeName => {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light'
  }

  /**
   * 跟随系统主题
   */
  const followSystemTheme = (): void => {
    localStorage.removeItem(THEME_STORAGE_KEY)
    const systemTheme = getSystemTheme()
    setTheme(systemTheme)
    // 同时更新后端为 "system"
    saveThemeToBackend('system')
  }

  // 计算属性
  const isDark = computed(() => currentTheme.value === 'dark')
  const isLight = computed(() => currentTheme.value === 'light')
  const themeName = computed(() => currentTheme.value)
  const themeLabel = computed(
    () => AVAILABLE_THEMES.find((t: ThemeConfig) => t.name === currentTheme.value)?.label || currentTheme.value
  )

  // 生命周期
  onMounted(() => {
    // 初始化主题
    initTheme()

    // 监听系统主题变化
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    systemThemeListener = (e: MediaQueryListEvent) => {
      // 仅当用户没有手动设置主题时才跟随系统
      if (!localStorage.getItem(THEME_STORAGE_KEY)) {
        setTheme(e.matches ? 'dark' : 'light')
      }
    }
    mediaQuery.addEventListener('change', systemThemeListener)
  })

  onUnmounted(() => {
    if (systemThemeListener) {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
      mediaQuery.removeEventListener('change', systemThemeListener)
      systemThemeListener = null
    }
  })

  return {
    // 状态
    currentTheme,
    themeName,
    themeLabel,
    isDark,
    isLight,
    isLoadingFromBackend,

    // 方法
    setTheme,
    toggleTheme,
    initTheme,
    getSystemTheme,
    followSystemTheme,

    // 常量
    availableThemes: AVAILABLE_THEMES,
  }
}

/**
 * 获取当前主题状态（非响应式，用于初始化）
 */
export function getThemeState() {
  return {
    currentTheme: currentTheme.value,
    isDark: currentTheme.value === 'dark',
    isLight: currentTheme.value === 'light',
  }
}

// 默认导出
export default useTheme
