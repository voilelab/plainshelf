import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': 'http://localhost:20000'
    }
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('reka-ui')) {
              return 'reka-ui';
            }
            if (id.includes('vue-router')) {
              return 'vue-router';
            }
            if (id.includes('/vue/') || id.includes('@vue')) {
              return 'vue';
            }
            return 'vendor';
          }
        }
      }
    }
  }
});
