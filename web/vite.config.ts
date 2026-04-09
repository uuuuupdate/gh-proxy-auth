import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (['vue', 'vue-router', 'pinia'].some(pkg => id.includes(`node_modules/${pkg}`))) {
            return 'vendor-vue'
          }
          if (['radix-vue', 'lucide-vue-next'].some(pkg => id.includes(`node_modules/${pkg}`))) {
            return 'vendor-ui'
          }
          if (['axios', '@vueuse/core'].some(pkg => id.includes(`node_modules/${pkg}`))) {
            return 'vendor-utils'
          }
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
