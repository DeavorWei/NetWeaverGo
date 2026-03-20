import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
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
