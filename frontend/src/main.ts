import { createApp } from 'vue'
import './styles/index.css'
import App from './App.vue'
import router from './router'

const app = createApp(App)

// 全局错误处理器 - 捕获 Vue 组件渲染错误
app.config.errorHandler = (err, instance, info) => {
  console.error('[Vue Error]', err)
  console.error('[Vue Error Info]', info)
  
  // 可选：在开发环境下显示更详细的错误信息
  if (import.meta.env.DEV) {
    console.error('[Vue Error Component]', instance?.$options?.name || 'Anonymous')
  }
}

// 全局警告处理器（仅在开发环境）
if (import.meta.env.DEV) {
  app.config.warnHandler = (msg, _instance, trace) => {
    console.warn('[Vue Warning]', msg)
    console.warn('[Vue Warning Trace]', trace)
  }
}

app.use(router)
app.mount('#app')
