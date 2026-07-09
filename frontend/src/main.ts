import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppZoom } from './composables/useAppZoom';
import { isMobileRuntime } from './providers/runtime';
import './styles.css';

async function bootstrap(): Promise<void> {
  initAppZoom();

  // On the native mobile shell, restore the saved server URL, token, and shelf
  // before mounting so the first API call already has a configured client.
  if (isMobileRuntime()) {
    const { initMobileConfig } = await import('./providers/mobileConfig');
    await initMobileConfig();
  }

  createApp(App).use(router).mount('#app');
}

void bootstrap();
