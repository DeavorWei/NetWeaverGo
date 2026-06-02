import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '/wails/runtime.js': '@wailsio/runtime'
    }
  },
  server: {
    host: '127.0.0.1',
    port: 9245,
    strictPort: true,
    open: false, // Wails 应用不需要自动打开浏览器
    hmr: {
      overlay: true // 显示错误覆盖层
    },
    watch: {
      usePolling: false // 使用原生文件系统事件，更高效
    }
  },
  build: {
    sourcemap: true, // 生成 source map 便于调试
    rollupOptions: {
      external: ['/wails/runtime.js'],
      output: {
        globals: {
          '/wails/runtime.js': 'wailsRuntime'
        },
        manualChunks: (id) => {
          // Vue 全家桶打包到 vendor
          if (id.includes('node_modules/vue/') ||
              id.includes('node_modules/vue-router/') ||
              id.includes('node_modules/pinia/')) {
            return 'vendor'
          }
        }
      }
    },
    // chunk 大小警告阈值
    chunkSizeWarningLimit: 500
  }
})
