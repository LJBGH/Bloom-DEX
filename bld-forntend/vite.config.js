import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      // Unified gateway (9003) with /api prefix.
      '/api': {
        target: 'http://127.0.0.1:9003',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
