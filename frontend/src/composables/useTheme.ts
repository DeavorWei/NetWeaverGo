/**
 * 主题管理 Composable
 * Theme Management Composable
 * 
 * 提供统一的主题管理功能，包括：
 * - 主题初始化
 * - 主题切换
 * - 主题持久化
 * - 系统主题跟随
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  type ThemeName,
  type ThemeConfig,
  AVAILABLE_THEMES,
  DEFAULT_THEME,
  THEME_STORAGE_KEY,
} from '@/types/theme'

// 全局状态（单例模式）
const currentTheme = ref<ThemeName>(DEFAULT_THEME)
const isInitialized = ref(false)

// 系统主题变化监听器
let systemThemeListener: ((e: MediaQueryListEvent) => void) | null = null

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
   * 初始化主题
   * 优先级：本地存储 > 系统偏好 > 默认主题
   */
  const initTheme = (): void => {
    if (isInitialized.value) return

    const savedTheme = localStorage.getItem(THEME_STORAGE_KEY) as ThemeName | null

    if (savedTheme && isValidTheme(savedTheme)) {
      currentTheme.value = savedTheme
    } else {
      // 检测系统偏好
      const prefersDark = window.matchMedia(
        '(prefers-color-scheme: dark)'
      ).matches
      currentTheme.value = prefersDark ? 'dark' : 'light'
    }

    applyTheme(currentTheme.value)
    isInitialized.value = true
  }

  /**
   * 设置主题
   */
  const setTheme = (theme: ThemeName): void => {
    if (!isValidTheme(theme)) {
      console.warn(`[useTheme] Invalid theme: ${theme}`)
      return
    }

    currentTheme.value = theme
    localStorage.setItem(THEME_STORAGE_KEY, theme)
    applyTheme(theme)
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
    setTheme(getSystemTheme())
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
