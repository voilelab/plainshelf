// @vitest-environment jsdom
import { createApp, h, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LanShareStatus } from '@/features/mobile/utils/lanShare';

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  start: vi.fn(),
  stop: vi.fn(),
  listener: null as ((status: LanShareStatus) => void) | null
}));

vi.mock('@/features/mobile/utils/lanShare', () => ({
  LanShare: {
    getStatus: mocks.getStatus,
    start: mocks.start,
    stop: mocks.stop,
    addListener: (_event: string, listener: (status: LanShareStatus) => void) => {
      mocks.listener = listener;
      return Promise.resolve({ remove: () => Promise.resolve() });
    }
  }
}));

import MobileLanSharePanel from './MobileLanSharePanel.vue';

const stopped: LanShareStatus = {
  running: false,
  url: '',
  token: '',
  shelf_ids: [],
  endedReason: '',
  idleLimitMinutes: 30
};
const running: LanShareStatus = {
  ...stopped,
  running: true,
  url: 'http://192.168.1.5:20001/dav/',
  token: 'tok',
  shelf_ids: ['local-1']
};
const shelves = [
  { id: 'local-1', name: 'Phone' },
  { id: 'local-2', name: 'Private' }
];

let mounted: { app: App; host: HTMLElement } | null = null;

async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve));
  await new Promise((resolve) => setTimeout(resolve));
}

async function mountPanel(): Promise<HTMLElement> {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp({ setup: () => () => h(MobileLanSharePanel, { shelves }) });
  app.mount(host);
  mounted = { app, host };
  await flush();
  return host;
}

function boxes(host: HTMLElement): HTMLInputElement[] {
  return [...host.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')];
}

describe('MobileLanSharePanel', () => {
  beforeEach(() => {
    mocks.getStatus.mockReset().mockResolvedValue(stopped);
    mocks.start.mockReset().mockResolvedValue(running);
    mocks.stop.mockReset().mockResolvedValue(stopped);
    mocks.listener = null;
  });

  afterEach(() => {
    mounted?.app.unmount();
    mounted?.host.remove();
    mounted = null;
  });

  it('ticks nothing and cannot start until a shelf is ticked', async () => {
    const host = await mountPanel();
    expect(boxes(host).every((box) => !box.checked)).toBe(true);
    expect(host.querySelector<HTMLButtonElement>('.mobile-lan-share-start')!.disabled).toBe(true);
    expect(host.querySelector('.mobile-lan-share-url')).toBeNull();
  });

  it('starts with only the ticked shelves and shows the address and token', async () => {
    const host = await mountPanel();
    boxes(host)[0].click();
    await flush();
    host.querySelector<HTMLButtonElement>('.mobile-lan-share-start')!.click();
    await flush();

    expect(mocks.start).toHaveBeenCalledWith({ shelfIds: ['local-1'] });
    expect(host.querySelector('.mobile-lan-share-url')!.textContent).toBe(running.url);
    expect(host.querySelector('.mobile-lan-share-token')!.textContent).toBe('tok');
  });

  it('explains a start refused off Wi-Fi', async () => {
    mocks.start.mockRejectedValue(new Error('not connected to Wi-Fi'));
    const host = await mountPanel();
    boxes(host)[0].click();
    await flush();
    host.querySelector<HTMLButtonElement>('.mobile-lan-share-start')!.click();
    await flush();
    expect(host.querySelector('.mobile-lan-share-error')!.textContent).toContain('Connect to Wi-Fi');
  });

  it('stops when the last shelf is unticked while running', async () => {
    mocks.getStatus.mockResolvedValue(running);
    const host = await mountPanel();
    expect(boxes(host)[0].checked).toBe(true);
    boxes(host)[0].click();
    await flush();
    expect(mocks.stop).toHaveBeenCalled();
  });

  it('says why a share stopped by itself', async () => {
    mocks.getStatus.mockResolvedValue(running);
    const host = await mountPanel();
    mocks.listener!({ ...stopped, endedReason: 'No device read the shelves for 30 minutes.' });
    await flush();
    expect(host.querySelector('.mobile-lan-share-url')).toBeNull();
    expect(host.querySelector('.mobile-lan-share-ended')!.textContent).toContain('30 minutes');
  });
});
