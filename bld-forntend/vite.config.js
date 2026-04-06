import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/userapi': {
        target: 'http://127.0.0.1:9004',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/userapi/, ''),
      },
      '/walletapi': {
        target: 'http://127.0.0.1:9005',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/walletapi/, ''),
      },
      '/ordersapi': {
        target: 'http://127.0.0.1:9006',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/ordersapi/, ''),
      },
      // market-ws：HTTP 快照 + WebSocket /ws（端口见 docs/service-ports.md）
      '/marketws': {
        target: 'http://127.0.0.1:9201',
        changeOrigin: true,
        ws: true,
        rewrite: (path) => path.replace(/^\/marketws/, ''),
      },
    },
  },
})
