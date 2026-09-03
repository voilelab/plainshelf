import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';
import vue from '@vitejs/plugin-vue';
import Icons from 'unplugin-icons/vite';

export default defineConfig({
  plugins: [vue(), Icons({ compiler: 'vue3', scale: 1 })],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  // PSW-76: `default` keeps Vitest's own output; the second reporter appends the
  // ten slowest cases so a slowdown is readable at the tail of the CI log.
  test: {
    reporters: ['default', './scripts/slowest-tests-reporter.mjs']
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': 'http://localhost:20000'
    }
  }
});
