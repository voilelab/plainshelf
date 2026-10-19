// @vitest-environment jsdom
import { createApp, h, nextTick, ref, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import FolderIconModal from './FolderIconModal.vue';
import { setLocale } from '@/i18n';

let app: App | null = null;

function mount(currentIcon: string) {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const open = ref(false);
  const submitted: string[] = [];
  app = createApp({
    render: () =>
      h(FolderIconModal, {
        open: open.value,
        folderName: 'Fiction',
        currentIcon,
        onSubmit: (payload: { icon: string }) => submitted.push(payload.icon)
      })
  });
  app.mount(host);
  return { open, submitted };
}

function radio(label: string): HTMLButtonElement {
  const found = document.body.querySelector<HTMLButtonElement>(`[role="radio"][aria-label="${label}"]`);
  if (!found) throw new Error(`no radio labelled ${label}`);
  return found;
}

function saveButton(): HTMLButtonElement {
  const found = [...document.body.querySelectorAll('button')].find((button) => button.textContent?.trim() === 'Save');
  if (!found) throw new Error('no Save button');
  return found;
}

beforeEach(() => {
  setLocale('en');
});

afterEach(() => {
  app?.unmount();
  app = null;
  document.body.innerHTML = '';
});

describe('FolderIconModal', () => {
  it('starts on the current icon and saves only a change', async () => {
    const { open, submitted } = mount('book');
    open.value = true;
    await nextTick();
    await nextTick();

    expect(radio('book').getAttribute('aria-checked')).toBe('true');
    expect(saveButton().disabled).toBe(true);

    radio('rocket').click();
    await nextTick();
    expect(radio('rocket').getAttribute('aria-checked')).toBe('true');
    saveButton().click();
    expect(submitted).toEqual(['rocket']);
  });

  it('offers the default as the way to clear an icon', async () => {
    const { open, submitted } = mount('book');
    open.value = true;
    await nextTick();
    await nextTick();

    radio('Default').click();
    await nextTick();
    saveButton().click();
    expect(submitted).toEqual(['']);
  });
});
