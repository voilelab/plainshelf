// @vitest-environment jsdom
import { createApp, nextTick, type App } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { setLocale, useI18n } from '@/i18n';

// Reka's Select portals its listbox and needs pointer gestures jsdom does not
// model, so a thin stand-in stands in for it: SelectItem becomes a button that
// forwards its value the way a real selection would, and a captured emitter
// lets a test also probe the guard with a value the list never offers.
const captured = vi.hoisted(() => ({ emit: null as null | ((value: unknown) => void) }));

vi.mock('reka-ui', async () => {
  const { defineComponent, h } = await import('vue');
  const passthrough = (tag: string) =>
    defineComponent({ setup: (_p, { slots }) => () => h(tag, {}, slots.default?.()) });

  return {
    SelectRoot: defineComponent({
      props: { modelValue: { type: String, default: '' } },
      emits: ['update:modelValue'],
      setup(_props, { slots, emit }) {
        captured.emit = (value: unknown) => emit('update:modelValue', value);
        return () => h('div', {}, slots.default?.());
      }
    }),
    SelectItem: defineComponent({
      props: { value: { type: String, required: true } },
      setup(props, { slots }) {
        return () =>
          h(
            'button',
            { 'data-value': props.value, onClick: () => captured.emit?.(props.value) },
            slots.default?.()
          );
      }
    }),
    SelectContent: passthrough('div'),
    SelectItemText: passthrough('span'),
    SelectPortal: passthrough('div'),
    SelectTrigger: passthrough('button'),
    SelectValue: passthrough('span'),
    SelectViewport: passthrough('div')
  };
});

import LanguagePanel from './LanguagePanel.vue';

function mount(): { host: HTMLElement; app: App } {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const app = createApp(LanguagePanel);
  app.mount(host);
  return { host, app };
}

let mounted: App | null = null;

beforeEach(() => {
  setLocale('en');
  captured.emit = null;
});

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  document.body.innerHTML = '';
  setLocale('en');
});

describe('LanguagePanel', () => {
  it('offers every supported locale as an option', () => {
    const { host, app } = mount();
    mounted = app;

    const values = Array.from(host.querySelectorAll('button[data-value]')).map((el) =>
      el.getAttribute('data-value')
    );
    expect(values).toEqual([...useI18n().supportedLocales]);
  });

  it('switches the UI locale when an option is chosen', () => {
    const { host, app } = mount();
    mounted = app;
    const { locale } = useI18n();
    expect(locale.value).toBe('en');

    host.querySelector<HTMLButtonElement>('button[data-value="zh-Hant"]')?.click();

    expect(locale.value).toBe('zh-Hant');
  });

  it('reflects a locale change made elsewhere in the closed trigger label', async () => {
    const { host, app } = mount();
    mounted = app;

    const trigger = host.querySelector('.select-trigger');
    expect(trigger?.textContent?.trim()).toBe('English');

    setLocale('zh-Hant');
    await nextTick();

    // reka-ui snapshots each item's text at mount; currentLabel is what keeps
    // the trigger honest after a runtime switch.
    expect(trigger?.textContent?.trim()).toBe('繁體中文');
  });

  it('ignores a value outside the supported locales', () => {
    const { app } = mount();
    mounted = app;
    const { locale } = useI18n();

    captured.emit?.('fr');

    expect(locale.value).toBe('en');
  });
});
