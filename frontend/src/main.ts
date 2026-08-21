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
  isWailsRuntime,
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
  } else if (!isWailsRuntime()) {
    // A browser cannot tell an ordinary server from the standalone reading
    // binary, which serves this same bundle, so it asks: installReaderShell
    // probes GET /api/mode and installs the shell only when the answer is
    // `reader`. Skipped on the desktop shell, which is its own full server and
    // has no such question to ask.
    //
    // Awaited here rather than left to MainLayout because the route policy has
    // to be in place before the first navigation, and because the provider must
    // be chosen before anything reaches for one. The answer is cached in
    // api/mode.ts, so MainLayout's own read of the mode costs no second
    // request.
    const { installReaderShell } = await import('@/shells/reader');
    await installReaderShell();
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
