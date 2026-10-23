import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    strictPort: true, // fail loudly instead of quietly moving to another port
    proxy: {
      // The page calls /api/...; in development, Vite passes those calls to the Go server.
      '/api': 'http://localhost:8080',
    },
  },
})
