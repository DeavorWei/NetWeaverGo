/**
 * 主题类型定义
 * Theme Type Definitions
 */

export interface ThemeColors {
  bg: {
    primary: string
    secondary: string
    tertiary: string
    hover: string
  }
  text: {
    primary: string
    secondary: string
    muted: string
  }
  border: {
    default: string
    focus: string
  }
  accent: {
    primary: string
    secondary: string
    glow: string
  }
  status: {
    success: string
    warning: string
    error: string
    info: string
  }
}

export interface Theme {
  name: ThemeName
  label: string
  description?: string
  colors: ThemeColors
}

export type ThemeName = 'light' | 'dark' | string

export type ThemeMode = ThemeName | 'system'

export interface ThemeConfig {
  name: ThemeName
  label: string
  cssFile?: string
}

// 预定义主题列表
export const AVAILABLE_THEMES: ThemeConfig[] = [
  { name: 'light', label: '明亮模式' },
  { name: 'dark', label: '黑暗模式' },
  // 可在此添加新主题
  // { name: 'ocean', label: '海洋主题', cssFile: 'themes/ocean.css' },
]

// 默认主题
export const DEFAULT_THEME: ThemeName = 'dark'

// 本地存储键名
export const THEME_STORAGE_KEY = 'netweaver-theme'
