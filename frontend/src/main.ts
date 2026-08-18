import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppZoom } from '@/composables/useAppZoom';
import {
  bookshelfWriter,
  getBookshelfProvider,
  isMobileRuntime,
  isMobileShellPreview,
  type BookshelfProvider,
  type WritableBookshelfProvider
} from './providers';
import '@fontsource-variable/noto-serif-tc/wght.css';
import '@fontsource-variable/noto-sans-tc/wght.css';
import './styles.css';

declare global {
  interface Window {
    // e2e-only hook exposing the active provider so Playwright can assert on
    // state that has no UI to read it from: getDownloadState, getReadProgress
    // and listDownloadedBookEntries. (Download and removal themselves are
    // driven through the UI now — BookDetailPage's offline button, the library
    // batch action, and DownloadsPage — so the hook is an observation point,
    // not a remote control.)
    //
    // Gated on isMobileShellPreview(), not isMobileRuntime(): every mobile spec
    // reaches the app through `?mobile-shell-preview=1`, and a native shell
    // never carries that query, so a shipped app exposes no provider on window.
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

    // Browser preview only — see the declaration above. Attached after
    // initMobileConfig() so the provider the tests read is the configured one.
    if (isMobileShellPreview()) {
      window.__plainshelfTestHooks = { provider: getBookshelfProvider(), bookshelfWriter };
    }
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
