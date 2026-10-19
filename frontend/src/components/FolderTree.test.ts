// @vitest-environment jsdom
import { createApp, h, nextTick, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { setLocale } from '@/i18n';

vi.mock('@/composables/useBookStore', async () => {
  const { ref } = await import('vue');
  return { useBookStore: () => ({ books: ref([]) }) };
});

const { default: FolderTree } = await import('./FolderTree.vue');

const nodes = [
  { name: 'Fiction', path: 'Fiction', children: [] },
  { name: 'Broken', path: 'Broken', children: [] }
];

let app: App | null = null;

function mount(props: Record<string, unknown>) {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const changed: string[] = [];
  app = createApp({
    render: () => h(FolderTree, { nodes, selected: undefined, ...props, onChangeIcon: (path: string) => changed.push(path) })
  });
  app.mount(host);
  return { host, changed };
}

function row(host: HTMLElement, name: string): HTMLElement {
  const label = [...host.querySelectorAll<HTMLElement>('.sidebar-nav-item-label')].find((el) => el.textContent?.trim() === name);
  const item = label?.closest<HTMLElement>('.folder-node');
  if (!item) throw new Error(`no folder row ${name}`);
  return item;
}

async function openMenu(target: HTMLElement): Promise<string[]> {
  target.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: 10, clientY: 10 }));
  await nextTick();
  await nextTick();
  return [...document.body.querySelectorAll('[role="menuitem"]')].map((item) => item.textContent?.trim() ?? '');
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  app?.unmount();
  app = null;
  document.body.innerHTML = '';
});

describe('FolderTree folder icons', () => {
  it('draws each folder with its own icon, and the default without one', () => {
    const { host } = mount({ folderMeta: { Fiction: { icon: 'rocket', version: 'v1', readOnly: false } } });
    expect(row(host, 'Fiction').querySelector('.folder-icon')?.outerHTML).not.toBe(
      row(host, 'Broken').querySelector('.folder-icon')?.outerHTML
    );
  });

  it('offers Change icon on a writable folder and reports the path', async () => {
    const { host, changed } = mount({ folderMeta: {} });
    const items = await openMenu(row(host, 'Fiction'));
    expect(items).toContain('Change icon');

    const changeIcon = [...document.body.querySelectorAll<HTMLElement>('[role="menuitem"]')].find(
      (item) => item.textContent?.trim() === 'Change icon'
    );
    changeIcon?.click();
    await nextTick();
    expect(changed).toEqual(['Fiction']);
  });

  it('does not offer Change icon on a read-only shelf', async () => {
    const { host } = mount({ readOnly: true, canTransferFolder: true, folderMeta: {} });
    expect(await openMenu(row(host, 'Fiction'))).not.toContain('Change icon');
  });

  it('does not offer Change icon for a folder.json the server will not rewrite', async () => {
    const { host } = mount({ folderMeta: { Broken: { icon: '', version: 'v1', readOnly: true } } });
    expect(await openMenu(row(host, 'Broken'))).not.toContain('Change icon');
  });
});
