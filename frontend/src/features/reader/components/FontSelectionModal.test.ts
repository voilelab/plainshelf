// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, reactive, type App } from 'vue';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReaderFont } from '@/features/reader/composables/useReaderSettings';

vi.mock('@/components/BaseDialog.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return {
    default: defineComponent({
      name: 'BaseDialogStub',
      props: { open: Boolean, title: String },
      setup(props, { slots }) {
        return () => (props.open ? h('div', { class: 'base-dialog-stub' }, slots.default?.()) : null);
      }
    })
  };
});

import FontSelectionModal from './FontSelectionModal.vue';

interface Mounted {
  app: App;
  host: HTMLElement;
  props: { open: boolean; fontFamily: ReaderFont };
  selected: ReaderFont[];
}

const mounted: Mounted[] = [];

function mountModal(fontFamily: ReaderFont = 'system'): Mounted {
  const host = document.createElement('div');
  document.body.append(host);
  const props = reactive({ open: true, fontFamily });
  const selected: ReaderFont[] = [];

  const app = createApp(defineComponent({
    setup: () => () => h(FontSelectionModal, {
      open: props.open,
      fontFamily: props.fontFamily,
      onClose: () => {},
      onSelect: (font: ReaderFont) => selected.push(font)
    })
  }));
  app.mount(host);

  const entry = { app, host, props, selected };
  mounted.push(entry);
  return entry;
}

function radios(host: HTMLElement): HTMLElement[] {
  return [...host.querySelectorAll<HTMLElement>('[role="radio"]')];
}

/** Lets RovingFocusItem's `nextTick` focus move and RadioGroupItem's `setTimeout(0)` click land. */
async function settleKeyboardMove(): Promise<void> {
  await nextTick();
  await new Promise((resolve) => setTimeout(resolve, 0));
  await nextTick();
}

function arrow(target: HTMLElement, key: 'ArrowDown' | 'ArrowUp'): void {
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
}

afterEach(() => {
  while (mounted.length > 0) {
    mounted.pop()?.app.unmount();
  }
  document.body.innerHTML = '';
});

describe('FontSelectionModal', () => {
  it('renders one Reka radio per font inside a single radiogroup', async () => {
    const { host } = mountModal();
    await nextTick();

    const groups = host.querySelectorAll('[role="radiogroup"]');
    expect(groups).toHaveLength(1);
    expect(groups[0].getAttribute('aria-label')).toBe('Available reading fonts');
    expect(radios(host).map((radio) => radio.getAttribute('value'))).toEqual([
      'system',
      'noto-serif-tc',
      'noto-sans-tc'
    ]);
    // The hand-written markup is gone: no native radio input and no manual role.
    expect(host.querySelectorAll('input[type="radio"]')).toHaveLength(0);
    expect(host.querySelectorAll('div[role="radiogroup"] > label')).toHaveLength(0);
  });

  it('drives the selected styling from data-state instead of a JS id comparison', async () => {
    const { host } = mountModal('noto-serif-tc');
    await nextTick();

    const states = radios(host).map((radio) => radio.dataset.state);
    expect(states).toEqual(['unchecked', 'checked', 'unchecked']);
    expect(radios(host).some((radio) => radio.classList.contains('active'))).toBe(false);

    const checkmarks = host.querySelectorAll('.font-selection-check');
    expect(checkmarks).toHaveLength(1);
    expect(checkmarks[0].textContent?.trim()).toBe('✓');
    expect(radios(host)[1].contains(checkmarks[0])).toBe(true);
  });

  it('keeps a single tab stop and hands entry focus to the checked font', async () => {
    const { host } = mountModal('noto-sans-tc');
    await nextTick();

    const group = host.querySelector<HTMLElement>('[role="radiogroup"]')!;
    expect(group.tabIndex).toBe(0);
    expect(radios(host).map((radio) => radio.tabIndex)).toEqual([-1, -1, -1]);

    group.focus();
    group.dispatchEvent(new FocusEvent('focus', { bubbles: false }));
    await nextTick();

    expect(document.activeElement).toBe(radios(host)[2]);
    expect(radios(host).map((radio) => radio.tabIndex)).toEqual([-1, -1, 0]);
  });

  it('moves the selection with the arrow keys', async () => {
    const entry = mountModal('system');
    const { host, selected } = entry;
    await nextTick();

    radios(host)[0].focus();
    await settleKeyboardMove();
    expect(selected).toEqual([]);

    arrow(radios(host)[0], 'ArrowDown');
    await settleKeyboardMove();

    expect(document.activeElement).toBe(radios(host)[1]);
    expect(selected).toEqual(['noto-serif-tc']);

    entry.props.fontFamily = 'noto-serif-tc';
    await nextTick();

    arrow(radios(host)[1], 'ArrowUp');
    await settleKeyboardMove();

    expect(document.activeElement).toBe(radios(host)[0]);
    expect(selected).toEqual(['noto-serif-tc', 'system']);
  });

  it('emits the picked font when a radio is clicked', async () => {
    const { host, selected } = mountModal('system');
    await nextTick();

    radios(host)[2].click();
    await nextTick();

    expect(selected).toEqual(['noto-sans-tc']);
  });
});
