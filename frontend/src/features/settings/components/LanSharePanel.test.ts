// @vitest-environment jsdom
import { nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import LanSharePanel from './LanSharePanel.vue';
import { mount } from '#testing/mount';
import { setLocale } from '@/i18n';
import { useShelvesStore } from '@/composables/useShelvesStore';
import type { DesktopLANShareStatus, setDesktopLANShare } from '@/api/desktop';

type DesktopLANShareParams = Parameters<typeof setDesktopLANShare>[0];

function offStatus(): DesktopLANShareStatus {
  return {
    enabled: false,
    running: false,
    host: '192.168.1.20',
    addresses: ['192.168.1.20'],
    shelves: [
      { id: 'a', name: 'Shelf A', shared: false },
      { id: 'b', name: 'Shelf B', shared: false }
    ],
    url: '',
    token: '',
    qr_code: '',
    error: ''
  };
}

function installBinding(state: DesktopLANShareStatus) {
  const setLANShare = vi.fn(async (params: DesktopLANShareParams) => {
    state.enabled = params.enabled;
    state.running = params.enabled;
    state.host = params.host;
    state.shelves = state.shelves.map((s) => ({
      ...s,
      shared: params.shelfIDs.includes(s.id)
    }));
    state.url = params.enabled ? 'http://192.168.1.20:20001/dav/' : '';
    state.token = params.enabled ? 'tok' : '';
    state.qr_code = params.enabled ? 'data:image/png;base64,AAAA' : '';
    return { ...state };
  });
  const getLANShare = vi.fn(async () => ({ ...state }));
  (window as unknown as { go: unknown }).go = {
    main: {
      DesktopApp: { GetLANShare: getLANShare, SetLANShare: setLANShare }
    }
  };
  return { setLANShare, getLANShare };
}

async function settle(): Promise<void> {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve();
    await nextTick();
  }
}

function click(host: HTMLElement, testid: string): void {
  const input = host.querySelector<HTMLInputElement>(`[data-testid="${testid}"]`);
  if (!input) throw new Error(`${testid} is missing`);
  input.click();
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  delete (window as unknown as { go?: unknown }).go;
});

describe('LanSharePanel', () => {
  it('starts off with nothing shown but the switch', async () => {
    installBinding(offStatus());
    const { host } = mount(LanSharePanel);
    await settle();

    const toggle = host.querySelector<HTMLInputElement>('[data-testid="lan-share-enabled"]');
    expect(toggle?.checked).toBe(false);
    expect(host.querySelector('[data-testid="lan-share-shelf-a"]')).toBeNull();
    expect(host.querySelector('[data-testid="lan-share-connect"]')).toBeNull();
  });

  it('turning it on shares no shelf until one is ticked, and shows how to connect', async () => {
    const { setLANShare } = installBinding(offStatus());
    const { host } = mount(LanSharePanel);
    await settle();

    click(host, 'lan-share-enabled');
    await settle();
    expect(setLANShare).toHaveBeenLastCalledWith({
      enabled: true,
      host: '192.168.1.20',
      shelfIDs: []
    });
    expect(host.textContent).toContain('No shelf is ticked');
    expect(host.querySelector('[data-testid="lan-share-url"]')?.textContent).toBe(
      'http://192.168.1.20:20001/dav/'
    );
    expect(host.querySelector('[data-testid="lan-share-token"]')?.textContent).toBe('tok');
    expect(host.querySelector<HTMLImageElement>('.lan-share-qr')?.src).toContain('data:image/png');

    click(host, 'lan-share-shelf-b');
    await settle();
    expect(setLANShare).toHaveBeenLastCalledWith({
      enabled: true,
      host: '192.168.1.20',
      shelfIDs: ['b']
    });

    click(host, 'lan-share-enabled');
    await settle();
    expect(setLANShare).toHaveBeenLastCalledWith({
      enabled: false,
      host: '192.168.1.20',
      shelfIDs: ['b']
    });
    expect(host.querySelector('[data-testid="lan-share-connect"]')).toBeNull();
  });

  it('shows why the switch did not turn on', async () => {
    const state = offStatus();
    const { setLANShare, getLANShare } = installBinding(state);
    setLANShare.mockRejectedValueOnce('listen tcp 192.168.1.20:20001: address already in use');
    const { host } = mount(LanSharePanel);
    await settle();

    click(host, 'lan-share-enabled');
    await settle();
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('address already in use');
    expect(getLANShare).toHaveBeenCalledTimes(2);
    expect(host.querySelector<HTMLInputElement>('[data-testid="lan-share-enabled"]')?.checked).toBe(
      false
    );
  });

  it('re-reads the shelf list when the shelves panel changes it', async () => {
    const state = offStatus();
    const { getLANShare } = installBinding(state);
    const { host } = mount(LanSharePanel);
    await settle();
    click(host, 'lan-share-enabled');
    await settle();
    expect(host.querySelector('[data-testid="lan-share-shelf-c"]')).toBeNull();

    state.shelves = [...state.shelves, { id: 'c', name: 'Shelf C', shared: false }];
    const { shelves } = useShelvesStore();
    shelves.value = [...shelves.value];
    await settle();

    expect(getLANShare).toHaveBeenCalledTimes(2);
    expect(host.querySelector('[data-testid="lan-share-shelf-c"]')).not.toBeNull();
  });
});
