import { isDesktopHost, isReaderRuntime } from '@/providers/runtime';

type WailsApp = 'DesktopApp' | 'ReaderApp';

type BindingMethod = (...args: never[]) => Promise<unknown>;

interface WailsWindow extends Window {
  go?: { main?: Partial<Record<WailsApp, Record<string, unknown>>> };
}

type WailsRuntime = typeof import('@wailsio/runtime');

let wailsRuntime: Promise<WailsRuntime> | undefined;

export function wailsHost(): WailsApp | null {
  if (isDesktopHost()) return 'DesktopApp';
  if (isReaderRuntime()) return 'ReaderApp';
  return null;
}

function hasV2Runtime(): boolean {
  return (window as WailsWindow).go !== undefined;
}

function lookup(app: WailsApp, method: string): unknown {
  return (window as WailsWindow).go?.main?.[app]?.[method];
}

function loadWailsRuntime(): Promise<WailsRuntime> {
  wailsRuntime ??= import('@wailsio/runtime');
  return wailsRuntime;
}

export function startWailsRuntime(): void {
  if (wailsHost() !== null && !hasV2Runtime()) {
    void loadWailsRuntime();
  }
}

export function hasWailsMethod(app: WailsApp, method: string): boolean {
  if (wailsHost() !== app) return false;
  return !hasV2Runtime() || typeof lookup(app, method) === 'function';
}

function missingBinding(app: WailsApp, method: string, cause?: unknown): Error {
  return new Error(`Wails binding ${app}.${method} is missing`, { cause });
}

async function callV3(app: WailsApp, method: string, args: unknown[]): Promise<unknown> {
  const { Call } = await loadWailsRuntime();
  try {
    return await Call.ByName(`main.${app}.${method}`, ...args);
  } catch (err) {
    throw err instanceof ReferenceError ? missingBinding(app, method, err) : err;
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
  if (!hasV2Runtime()) {
    return callV3(app, method, args) as ReturnType<B[M]>;
  }
  const binding = lookup(app, method);
  if (typeof binding !== 'function') {
    throw missingBinding(app, method);
  }
  return binding(...args) as ReturnType<B[M]>;
}
