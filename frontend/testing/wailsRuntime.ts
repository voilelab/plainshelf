type Bindings = Partial<Record<'DesktopApp' | 'ReaderApp', Record<string, unknown>>>;

const registry = Symbol.for('plainshelf.test.wailsBindings');

const listeners = Symbol.for('plainshelf.test.wailsEvents');

type Listener = (event: { name: string; data: unknown }) => void;

type Holder = { [registry]?: Bindings; [listeners]?: Map<string, Set<Listener>> };

function eventListeners(): Map<string, Set<Listener>> {
  return ((globalThis as Holder)[listeners] ??= new Map());
}

export function bindWails(bindings: Bindings): void {
  (globalThis as Holder)[registry] = bindings;
}

export function unbindWails(): void {
  delete (globalThis as Holder)[registry];
  eventListeners().clear();
}

export function emitWails(name: string, data: unknown): void {
  for (const listener of eventListeners().get(name) ?? []) listener({ name, data });
}

export const Events = {
  On(name: string, callback: Listener): () => void {
    const set = eventListeners().get(name) ?? new Set<Listener>();
    eventListeners().set(name, set);
    set.add(callback);
    return () => set.delete(callback);
  }
};

export const Call = {
  async ByName(name: string, ...args: unknown[]): Promise<unknown> {
    const [, app, method] = name.split('.');
    const bound = (globalThis as Holder)[registry]?.[app as keyof Bindings]?.[method];
    if (typeof bound !== 'function') {
      throw new ReferenceError(`unknown bound method name: ${name}`);
    }
    return bound(...args);
  }
};
