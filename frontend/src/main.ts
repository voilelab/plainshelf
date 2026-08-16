import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppZoom } from '@/composables/useAppZoom';
import {
  bookshelfWriter,
  getBookshelfProvider,
  isMobileRuntime,
  type BookshelfProvider,
  type WritableBookshelfProvider
} from './providers';
import '@fontsource-variable/noto-serif-tc/wght.css';
import '@fontsource-variable/noto-sans-tc/wght.css';
import './styles.css';

declare global {
  interface Window {
    // e2e-only hook exposing the active provider so Playwright can drive
    // downloadBook/removeDownload/getDownloadState, which have no UI entry
    // point. Gated by isMobileRuntime() — the same exposure level as the
    // existing __plainshelfZoom Wails test hook (useAppZoom.ts) — so it also
    // ships in native Android builds, not just this browser preview.
    //
    // bookshelfWriter comes with it so a test can assert that the shelf write
    // surface is refused. Reaching for a write method on `provider` no longer
    // demonstrates that: on mobile the method is absent, so the call fails as a
    // TypeError rather than as the refusal being tested for.
    __plainshelfTestHooks?: {
      provider: BookshelfProvider;
      bookshelfWriter: () => WritableBookshelfProvider;
    };
  }
}

async function bootstrap(): Promise<void> {
  initAppZoom();

  // On the native mobile shell, restore the saved server URL, token, and shelf
  // before mounting so the first API call already has a configured client.
  if (isMobileRuntime()) {
    const { initMobileConfig } = await import('@/providers/mobileConfig');
    await initMobileConfig();

    // e2e-only: expose the provider for driving download/removeDownload/
    // getDownloadState, which have no UI trigger to click in tests.
    window.__plainshelfTestHooks = { provider: getBookshelfProvider(), bookshelfWriter };
  }

  const app = createApp(App);

  // Vue swallows component errors by default, so a bug in a render function or
  // a lifecycle hook leaves no trace in the console. App.vue's onErrorCaptured
  // handles what the user sees; this makes the cause diagnosable.
  app.config.errorHandler = (err, _instance, info) => {
    console.error(`Unhandled Vue error (${info})`, err);
  };

  app.use(router).mount('#app');
}

void bootstrap();
