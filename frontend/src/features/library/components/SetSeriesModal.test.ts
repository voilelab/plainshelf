// @vitest-environment jsdom
import { createApp, h, nextTick, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/components/BaseDialog.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return {
    default: defineComponent({
      props: { open: Boolean },
      setup(props, { slots }) {
        return () => (props.open ? h('div', slots.default?.()) : null);
      }
    })
  };
});

import SetSeriesModal, { type SeriesBatchDraft } from './SetSeriesModal.vue';

const apps: App[] = [];

async function mountModal(): Promise<{ host: HTMLElement; submitted: SeriesBatchDraft[] }> {
  const host = document.createElement('div');
  document.body.append(host);
  const submitted: SeriesBatchDraft[] = [];
  const app = createApp({
    render: () => h(SetSeriesModal, {
      open: true,
      count: 3,
      knownNames: ['Saga'],
      onSubmit: (draft: SeriesBatchDraft) => submitted.push(draft)
    })
  });
  app.mount(host);
  apps.push(app);
  await nextTick();
  return { host, submitted };
}

async function fill(host: HTMLElement, name: string, value: string): Promise<void> {
  const input = host.querySelector<HTMLInputElement>(`input[name="${name}"]`)!;
  input.value = value;
  input.dispatchEvent(new Event('input'));
  await nextTick();
}

async function submit(host: HTMLElement): Promise<void> {
  host.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
  await nextTick();
}

afterEach(() => {
  while (apps.length) apps.pop()!.unmount();
  document.body.innerHTML = '';
});

describe('SetSeriesModal', () => {
  it('submits the trimmed name and the starting number, which defaults to 1', async () => {
    const { host, submitted } = await mountModal();
    expect(host.querySelector<HTMLInputElement>('input[name="series-start-index"]')!.value).toBe('1');

    await fill(host, 'series-name', '  Saga ');
    await submit(host);
    await fill(host, 'series-start-index', '4');
    await submit(host);

    expect(submitted).toEqual([{ name: 'Saga', startIndex: 1 }, { name: 'Saga', startIndex: 4 }]);
  });

  it('blocks a blank name or a negative starting number without submitting', async () => {
    const { host, submitted } = await mountModal();

    await fill(host, 'series-name', '   ');
    await submit(host);
    expect(host.querySelector('[role="alert"]')?.textContent).toBeTruthy();

    await fill(host, 'series-name', 'Saga');
    await fill(host, 'series-start-index', '-1');
    await submit(host);
    expect(host.querySelector('[role="alert"]')?.textContent).toBeTruthy();

    expect(submitted).toEqual([]);
  });

  it('submits a clear request whatever the other fields hold', async () => {
    const { host, submitted } = await mountModal();
    const clear = host.querySelector<HTMLInputElement>('input[name="series-clear"]')!;
    clear.checked = true;
    clear.dispatchEvent(new Event('change'));
    await nextTick();

    await submit(host);

    expect(submitted).toEqual([null]);
  });
});
