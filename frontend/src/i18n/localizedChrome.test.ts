// @vitest-environment jsdom
import { createApp, defineComponent, h, nextTick, type App } from 'vue';
import { afterEach, describe, expect, it } from 'vitest';
import { setLocale, t } from '@/i18n';
import ConfirmModal from '@/components/ConfirmModal.vue';
import Pagination from '@/components/Pagination.vue';
import { coverFilter, type AnyBookFilterDef } from '@/utils/bookFilters/registry';
import { filterValueLabel } from '@/features/library/utils/filterLabels';

// `locales.test.ts` proves the two catalogs agree with each other, which is a
// different claim from "the screen reads from a catalog at all": a template with
// the English welded in passes it, and so does a component whose default prop
// froze an English literal where it was defined. Six end-to-end cases used to
// close that gap by booting the whole app in zh-Hant. Mounting the components
// makes the same claim without a server — what each one needs is a locale, not a
// shelf.

let active: { app: App; host: HTMLElement } | null = null;

function mount(render: () => unknown): HTMLElement {
  const host = document.createElement('div');
  document.body.append(host);
  const app = createApp(defineComponent({ setup: () => () => render() }));
  app.mount(host);
  active = { app, host };
  return host;
}

function labels(host: HTMLElement, selector: string): (string | null)[] {
  return [...host.querySelectorAll(selector)].map((element) =>
    element.getAttribute('aria-label')
  );
}

afterEach(() => {
  active?.app.unmount();
  active?.host.remove();
  active = null;
  setLocale('en');
});

describe('chrome renders from the active catalog', () => {
  it("names Pagination's icon-only edge buttons in the active locale", () => {
    // These two carry no text, so their accessible name is all a screen reader
    // gets — and both were once welded in as English.
    setLocale('zh-Hant');
    const host = mount(() => h(Pagination, { page: 2, total: 30, pageSize: 10 }));

    const names = labels(host, 'button[aria-label]');
    expect(names).toContain('第一頁');
    expect(names).toContain('最後一頁');
    expect(names).not.toContain('First page');
  });

  it('falls back to translated ConfirmModal defaults, not frozen English ones', async () => {
    // `withDefaults` is evaluated once where the component is defined, so a
    // `t()` call there would have captured whatever locale loaded first. The
    // defaults are computed instead; a caller that omits them still gets words
    // in the reader's language.
    setLocale('zh-Hant');
    mount(() =>
      h(ConfirmModal, { open: true, title: '移到垃圾桶', message: '確定嗎？', variant: 'danger' })
    );
    await nextTick();

    // Reka portals the dialog to the body, so it is read from there rather than
    // from the mount host.
    const dialog = document.body;
    expect(dialog.textContent).toContain('取消');
    expect(dialog.textContent).not.toContain('Cancel');
    expect(labels(dialog, 'button[aria-label]')).toContain('關閉確認對話框');
  });
});

describe('a runtime-composed message follows a locale switch in place', () => {
  it('rebuilds the library empty state rather than keeping the first language', () => {
    // The message is assembled from a filter definition and its value, so it is
    // not a stored sentence that a re-render could simply look up again.
    const condition = () =>
      t('library.empty.noBooksForCondition', {
        condition: filterValueLabel(coverFilter as unknown as AnyBookFilterDef, { kind: 'has' }, t)
      });

    expect(condition()).toBe('No books match Cover: Present.');

    setLocale('zh-Hant');

    expect(condition()).toBe('沒有符合封面：有的書籍。');
  });

  it('keeps the two languages distinct, so a missing translation cannot pass', () => {
    const english = t('library.empty.noBooksForCondition', { condition: 'x' });
    setLocale('zh-Hant');

    expect(t('library.empty.noBooksForCondition', { condition: 'x' })).not.toBe(english);
  });
});
