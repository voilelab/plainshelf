import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { initAppTheme } from '@/composables/useAppTheme';
import { initAppZoom } from '@/composables/useAppZoom';
import { reportUnhandledError } from '@/composables/useErrorIncident';
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
import '@fontsource-variable/noto-serif-sc/wght.css';
import '@fontsource-variable/noto-sans-sc/wght.css';
import './styles.css';

declare global {
  interface Window {
    __plainshelfTestHooks?: {
      provider: BookshelfProvider;
      bookshelfWriter: () => WritableBookshelfProvider;
    };
  }
}

export async function bootstrap(): Promise<void> {
  initAppTheme();
  initAppZoom();

  if (isReaderRuntime()) {
    const { installReaderShell } = await import('@/shells/reader');
    installReaderShell();
  }

  if (isMobileRuntime()) {
    const { installMobileShell } = await import('@/shells/mobile');
    await installMobileShell();

    if (isMobileShellPreview()) {
      window.__plainshelfTestHooks = { provider: getBookshelfProvider(), bookshelfWriter };
    }
  }

  getShell()?.installRouterGuards?.(router);

  const app = createApp(App);

  app.config.errorHandler = (err, _instance, info) => {
    reportUnhandledError(err, info);
  };

  app.use(router).mount('#app');
}

