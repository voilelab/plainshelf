import { afterEach, describe, expect, it } from 'vitest';

import { ServerBookshelfProvider } from './serverBookshelfProvider';
import { createBookshelfProvider } from './index';
import { getShell, registerShell, type RuntimeShell } from './shell';

afterEach(() => {
  registerShell(null);
});

describe('runtime shell registry', () => {
  it('falls back to the server provider when no shell is installed', () => {
    expect(getShell()).toBeNull();

    expect(createBookshelfProvider()).toBeInstanceOf(ServerBookshelfProvider);
  });

  it('lets an installed shell decide instead', () => {
    const fromShell = new ServerBookshelfProvider();
    const shell: RuntimeShell = { createProvider: () => fromShell };

    registerShell(shell);

    expect(getShell()).toBe(shell);
    expect(createBookshelfProvider()).toBe(fromShell);
  });

  it('builds the provider lazily, so a shell may set up state before first use', () => {
    let built = 0;
    registerShell({
      createProvider: () => {
        built += 1;
        return new ServerBookshelfProvider();
      }
    });

    expect(built).toBe(0);

    createBookshelfProvider();
    expect(built).toBe(1);
  });

  it('clears back to the default when a shell is unregistered', () => {
    registerShell({ createProvider: () => new ServerBookshelfProvider() });
    registerShell(null);

    expect(getShell()).toBeNull();
    expect(createBookshelfProvider()).toBeInstanceOf(ServerBookshelfProvider);
  });
});
