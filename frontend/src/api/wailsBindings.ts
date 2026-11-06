import { isDesktopHost, isReaderRuntime } from '@/providers/runtime';

type WailsApp = 'DesktopApp' | 'ReaderApp';

type BindingMethod = (...args: never[]) => Promise<unknown>;

type WailsRuntime = typeof import('@wailsio/runtime');

let wailsRuntime: Promise<WailsRuntime> | undefined;

export function wailsHost(): WailsApp | null {
  if (isDesktopHost()) return 'DesktopApp';
  if (isReaderRuntime()) return 'ReaderApp';
  return null;
}

function loadWailsRuntime(): Promise<WailsRuntime> {
  wailsRuntime ??= import('@wailsio/runtime');
  return wailsRuntime;
}

export function startWailsRuntime(): void {
  if (wailsHost() !== null) {
    void loadWailsRuntime();
  }
}

async function callBound(app: WailsApp, method: string, args: unknown[]): Promise<unknown> {
  const { Call } = await loadWailsRuntime();
  try {
    return await Call.ByName(`main.${app}.${method}`, ...args);
  } catch (err) {
    throw err instanceof ReferenceError
      ? new Error(`Wails binding ${app}.${method} is missing`, { cause: err })
      : err;
  }
}

export function callWails<B extends Record<keyof B, BindingMethod>, M extends keyof B & string>(
  app: WailsApp,
  method: M,
  ...args: Parameters<B[M]>
): ReturnType<B[M]> {
  if (wailsHost() !== app) {
    throw new Error(`Wails binding ${app}.${method} called outside the ${app} window`);
  }
  return callBound(app, method, args) as ReturnType<B[M]>;
}

export function onWailsEvent(name: string, listener: () => void): () => void {
  let unsubscribe: (() => void) | undefined;
  let cancelled = false;
  void loadWailsRuntime().then(({ Events }) => {
    if (!cancelled) {
      unsubscribe = Events.On(name, listener);
    }
  });
  return () => {
    cancelled = true;
    unsubscribe?.();
  };
}
