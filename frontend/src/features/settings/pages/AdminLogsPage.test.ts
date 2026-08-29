// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { LogFileEntry } from '@/api/logs';

// A log file has no bound on its size, so the page reads only its end and
// reaches further back on request. The window and the step are stubbed at small
// values: what matters is that the page uses whatever the api module exports,
// not what those numbers happen to be.
const TAIL_BYTES = 100;
const TAIL_STEP = 4;

const api = vi.hoisted(() => ({
  listLogs: vi.fn(),
  getLogContent: vi.fn()
}));

vi.mock('@/api/logs', () => ({
  DEFAULT_LOG_TAIL_BYTES: 100,
  LOG_TAIL_STEP: 4,
  listLogs: api.listLogs,
  getLogContent: api.getLogContent
}));

// The select is a Reka primitive that portals its content; the page's tail
// behavior does not depend on it, and the first option is auto-selected.
vi.mock('reka-ui', () => {
  const passthrough = (tag: string) =>
    defineComponent({ setup: (_p, { slots }) => () => h(tag, {}, slots.default?.()) });
  return {
    SelectContent: passthrough('div'),
    SelectItem: passthrough('div'),
    SelectItemText: passthrough('span'),
    SelectPortal: passthrough('div'),
    SelectRoot: passthrough('div'),
    SelectTrigger: passthrough('button'),
    SelectValue: passthrough('span'),
    SelectViewport: passthrough('div')
  };
});

import AdminLogsPage from './AdminLogsPage.vue';
import { setLocale } from '@/i18n';

function logEntry(overrides: Partial<LogFileEntry> = {}): LogFileEntry {
  return {
    id: 'app-1',
    source: 'logger',
    filename: 'app-2024-01-02.log',
    date: '2024-01-02',
    size: 10,
    ...overrides
  };
}

function mount() {
  const host = document.createElement('div');
  const app = createApp(AdminLogsPage);
  app.mount(host);
  return { host, app };
}

// settle lets the two awaited loads (the listing, then the content) and the
// renders that follow them finish.
async function settle(): Promise<void> {
  for (let i = 0; i < 6; i += 1) {
    await nextTick();
  }
}

function findButton(host: HTMLElement, label: string): HTMLButtonElement | undefined {
  return Array.from(host.querySelectorAll('button')).find((b) => b.textContent?.trim() === label);
}

beforeEach(() => {
  setLocale('en');
  api.listLogs.mockResolvedValue([logEntry()]);
  api.getLogContent.mockResolvedValue('line\n');
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('AdminLogsPage', () => {
  it('reads a bounded tail rather than the whole file', async () => {
    const { host, app } = mount();
    await settle();

    expect(api.getLogContent).toHaveBeenCalledWith('app-1', TAIL_BYTES);
    // A file that fits in the window is not announced as truncated.
    expect(host.querySelector('.truncation')).toBeNull();

    app.unmount();
  });

  it('offers to reach further back when the file is larger than the window', async () => {
    api.listLogs.mockResolvedValue([logEntry({ size: TAIL_BYTES * 10 })]);
    const { host, app } = mount();
    await settle();

    expect(host.querySelector('.truncation')).not.toBeNull();

    findButton(host, 'Load more')?.click();
    await settle();

    expect(api.getLogContent).toHaveBeenLastCalledWith('app-1', TAIL_BYTES * TAIL_STEP);

    app.unmount();
  });

  it('starts from the end again when a different file is selected', async () => {
    api.listLogs.mockResolvedValue([
      logEntry({ size: TAIL_BYTES * 10 }),
      logEntry({ id: 'app-2', filename: 'app-2024-01-03.log', date: '2024-01-03', size: TAIL_BYTES * 10 })
    ]);
    const { host, app } = mount();
    await settle();

    findButton(host, 'Load more')?.click();
    await settle();
    expect(api.getLogContent).toHaveBeenLastCalledWith('app-1', TAIL_BYTES * TAIL_STEP);

    const dateInput = host.querySelector<HTMLInputElement>('input[type="date"]');
    if (!dateInput) {
      throw new Error('date input is missing');
    }
    dateInput.value = '2024-01-03';
    dateInput.dispatchEvent(new Event('input'));
    await settle();

    expect(api.getLogContent).toHaveBeenLastCalledWith('app-2', TAIL_BYTES);

    app.unmount();
  });
});
