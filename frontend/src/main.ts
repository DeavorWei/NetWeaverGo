import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './styles/index.css'
import App from './App.vue'
import router from './router'

// ==================== 日志系统初始化 ====================
import { configureLogger, setupGlobalErrorCapture, getLogger } from './utils/logger'

// 配置日志系统
configureLogger({
  enableBackendLog: true,
  enableConsole: import.meta.env.DEV, // 开发环境启用控制台输出
  captureGlobalErrors: true,
})

// 设置全局错误捕获 (window.onerror, unhandledrejection)
setupGlobalErrorCapture()

// ==================== Vue 应用初始化 ====================
const app = createApp(App)
app.use(createPinia())

// 全局错误处理器 - 捕获 Vue 组件渲染错误，并记录到日志系统
app.config.errorHandler = (err, instance, info) => {
  const componentName = instance?.$options?.name || 'Anonymous'
  
  // 记录到日志系统
  getLogger().error(err instanceof Error ? err.message : String(err), 'Vue', err instanceof Error ? err : new Error(String(err)))
  getLogger().error(String(info), 'Vue')
  if (import.meta.env.DEV) {
    getLogger().error(`组件: ${componentName}`, 'Vue')
  }
}

// 全局警告处理器（仅在开发环境）
if (import.meta.env.DEV) {
  app.config.warnHandler = (msg, _instance, trace) => {
    // 记录警告日志
    getLogger().warn(msg, 'Vue')
    getLogger().warn(String(trace), 'Vue')
  }
}

app.use(router)
app.mount('#app')

// 应用启动完成日志
getLogger().info('Vue application mounted successfully', 'App')
