type Bindings = Partial<Record<'DesktopApp' | 'ReaderApp', Record<string, unknown>>>;

const registry = Symbol.for('plainshelf.test.wailsBindings');

type Holder = { [registry]?: Bindings };

export function bindWails(bindings: Bindings): void {
  (globalThis as Holder)[registry] = bindings;
}

export function unbindWails(): void {
  delete (globalThis as Holder)[registry];
}

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

const eventListeners = new Map<string, Set<() => void>>();

export const Events = {
  On(name: string, listener: () => void): () => void {
    const set = eventListeners.get(name) ?? new Set();
    set.add(listener);
    eventListeners.set(name, set);
    return () => set.delete(listener);
  }
};

export function emitWailsEvent(name: string): void {
  for (const listener of eventListeners.get(name) ?? []) {
    listener();
  }
}
