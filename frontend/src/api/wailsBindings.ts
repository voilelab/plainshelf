import { isDesktopHost, isReaderRuntime } from '@/providers/runtime';

type WailsApp = 'DesktopApp' | 'ReaderApp';

type BindingMethod = (...args: never[]) => Promise<unknown>;

interface WailsWindow extends Window {
  go?: { main?: Partial<Record<WailsApp, Record<string, unknown>>> };
}

export function wailsHost(): WailsApp | null {
  if (isDesktopHost()) return 'DesktopApp';
  if (isReaderRuntime()) return 'ReaderApp';
  return null;
}

function lookup(app: WailsApp, method: string): unknown {
  return (window as WailsWindow).go?.main?.[app]?.[method];
}

export function hasWailsMethod(app: WailsApp, method: string): boolean {
  return wailsHost() === app && typeof lookup(app, method) === 'function';
}

export function callWails<B extends Record<keyof B, BindingMethod>, M extends keyof B & string>(
  app: WailsApp,
  method: M,
  ...args: Parameters<B[M]>
): ReturnType<B[M]> {
  if (wailsHost() !== app) {
    throw new Error(`Wails binding ${app}.${method} called outside the ${app} window`);
  }
  const binding = lookup(app, method);
  if (typeof binding !== 'function') {
    throw new Error(`Wails binding ${app}.${method} is missing`);
  }
  return binding(...args) as ReturnType<B[M]>;
}
