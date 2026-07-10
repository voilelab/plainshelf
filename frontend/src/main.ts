import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppZoom } from './composables/useAppZoom';
import { getBookshelfProvider, isMobileRuntime, type BookshelfProvider } from './providers';
import './styles.css';

declare global {
  interface Window {
    // e2e-only hook exposing the active provider so Playwright can drive
    // downloadBook/removeDownload/getDownloadState, which have no UI entry
    // point. Gated by isMobileRuntime() — the same exposure level as the
    // existing __plainshelfZoom Wails test hook (useAppZoom.ts) — so it also
    // ships in native Android builds, not just this browser preview.
    __plainshelfTestHooks?: { provider: BookshelfProvider };
  }
}

async function bootstrap(): Promise<void> {
  initAppZoom();

  // On the native mobile shell, restore the saved server URL, token, and shelf
  // before mounting so the first API call already has a configured client.
  if (isMobileRuntime()) {
    const { initMobileConfig } = await import('./providers/mobileConfig');
    await initMobileConfig();

    // e2e-only: expose the provider for driving download/removeDownload/
    // getDownloadState, which have no UI trigger to click in tests.
    window.__plainshelfTestHooks = { provider: getBookshelfProvider() };
  }

  createApp(App).use(router).mount('#app');
}

void bootstrap();
