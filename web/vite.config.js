import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    // In Wails dev mode, proxy API calls to the Go backend
    proxy: {
      '/api': {
        target: 'http://localhost:32567',
        changeOrigin: true
      }
    }
  }
})
