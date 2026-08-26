import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppZoom } from '@/composables/useAppZoom';
import {
  bookshelfWriter,
  getBookshelfProvider,
  getShell,
  isMobileRuntime,
  isMobileShellPreview,
  isReaderRuntime,
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

  // The standalone reader app: one book, opened from a folder the user picked,
  // and no library around it. Dynamically imported for the same reason the
  // mobile shell is — its shell stays out of the web and desktop bundles.
  if (isReaderRuntime()) {
    const { installReaderShell } = await import('@/shells/reader');
    installReaderShell();
  }

  // Bring up the mobile shell before mounting: it restores the saved server
  // URL, token and shelf, and registers the provider the app reads through.
  // Dynamically imported so the mobile stack — its provider, its on-device
  // caches, the pCloud client — stays out of the web and desktop bundles.
  if (isMobileRuntime()) {
    const { installMobileShell } = await import('@/shells/mobile');
    await installMobileShell();

    // Browser preview only — see the declaration above. Attached after the
    // shell is installed so the provider the tests read is the configured one.
    if (isMobileShellPreview()) {
      window.__plainshelfTestHooks = { provider: getBookshelfProvider(), bookshelfWriter };
    }
  }

  // Before createApp: app.use(router) triggers the first navigation, and a
  // guard registered after that would let the first route through ungated.
  getShell()?.installRouterGuards?.(router);

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
