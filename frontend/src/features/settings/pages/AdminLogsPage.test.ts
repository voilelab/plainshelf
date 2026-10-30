// @vitest-environment jsdom
import { defineComponent, h, nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { LogFileEntry } from '@/api/logs';

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
import { mount } from '#testing/mount';
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

function mountPage() {
  return mount(AdminLogsPage);
}

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
    const { host } = mountPage();
    await settle();

    expect(api.getLogContent).toHaveBeenCalledWith('app-1', TAIL_BYTES);
    expect(host.querySelector('.truncation')).toBeNull();

  });

  it('offers to reach further back when the file is larger than the window', async () => {
    api.listLogs.mockResolvedValue([logEntry({ size: TAIL_BYTES * 10 })]);
    const { host } = mountPage();
    await settle();

    expect(host.querySelector('.truncation')).not.toBeNull();

    findButton(host, 'Load more')?.click();
    await settle();

    expect(api.getLogContent).toHaveBeenLastCalledWith('app-1', TAIL_BYTES * TAIL_STEP);

  });

  it('explains why the list is empty and how to fix it instead of a bare message', async () => {
    api.listLogs.mockResolvedValue([]);
    const { host } = mountPage();
    await settle();

    const emptyState = host.querySelector('.empty-state');
    expect(emptyState).not.toBeNull();
    const text = emptyState?.textContent ?? '';
    expect(text).toContain('stderr');
    expect(text).toContain('log_file.type');

  });

  it('distinguishes an empty list from a browsable source with no file for the date', async () => {
    api.listLogs.mockResolvedValue([logEntry({ date: '2024-01-02' })]);
    const { host } = mountPage();
    await settle();

    const dateInput = host.querySelector<HTMLInputElement>('input[type="date"]');
    if (!dateInput) {
      throw new Error('date input is missing');
    }
    dateInput.value = '2024-01-05';
    dateInput.dispatchEvent(new Event('input'));
    await settle();

    expect(host.querySelector('.empty-state')).toBeNull();
    expect(host.querySelector('.message.warning')?.textContent).toContain('2024-01-05');

  });

  it('starts from the end again when a different file is selected', async () => {
    api.listLogs.mockResolvedValue([
      logEntry({ size: TAIL_BYTES * 10 }),
      logEntry({ id: 'app-2', filename: 'app-2024-01-03.log', date: '2024-01-03', size: TAIL_BYTES * 10 })
    ]);
    const { host } = mountPage();
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

  });
});
