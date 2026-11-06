// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick, reactive } from 'vue';
import { mount } from '#testing/mount';
import ComicReaderView from './ComicReaderView.vue';

const { getSourceAsset } = vi.hoisted(() => ({ getSourceAsset: vi.fn() }));
vi.mock('@/providers', () => ({ getBookshelfProvider: () => ({ getSourceAsset }) }));

beforeEach(() => {
  getSourceAsset.mockReset().mockResolvedValue(new Blob(['image']));
  vi.stubGlobal('URL', { createObjectURL: vi.fn(() => 'blob:comic'), revokeObjectURL: vi.fn() });
});

function setup(direction: 'ltr' | 'rtl' = 'ltr', missing = false) {
  const state = reactive({ index: 2 });
  const pages = Array.from({ length: 8 }, (_, index) => ({ name: `${index}.png`, missing: missing && index === 2 }));
  const next = vi.fn(() => { state.index += 1; });
  const previous = vi.fn(() => { state.index -= 1; });
  const view = mount(ComicReaderView, { props: () => ({
    bookId: 'comic', sourceId: 'pages', title: 'Comic', pages, index: state.index,
    direction, percent: 38, loading: false, error: '', saveError: '', showBackNavigation: true,
    onNext: next, onPrevious: previous
  }) });
  const viewport = view.host.querySelector<HTMLElement>('.comic-viewport')!;
  vi.spyOn(viewport, 'getBoundingClientRect').mockReturnValue({ left: 0, width: 400 } as DOMRect);
  return { ...view, viewport, state, next, previous };
}

async function settle() { await Promise.resolve(); await Promise.resolve(); await nextTick(); }

function touch(viewport: HTMLElement, from: number, to: number, multi = false): void {
  const item = (x: number) => ({ identifier: 1, clientX: x, clientY: 200 });
  function dispatch(type: string, touches: unknown[], changed: unknown[] = []) {
    const event = new Event(type);
    const list = (items: unknown[]) => Object.assign(items, { item: (index: number) => items[index] ?? null });
    Object.defineProperties(event, { touches: { value: list(touches) }, changedTouches: { value: list(changed) } });
    viewport.dispatchEvent(event);
  }
  dispatch('touchstart', [item(from)]);
  if (multi) dispatch('touchmove', [item(from), { ...item(from + 20), identifier: 2 }]);
  dispatch('touchend', [], [item(to)]);
}

describe('ComicReaderView', () => {
  it.each(['ltr', 'rtl'] as const)('uses %s edge taps and swipes, without a synthetic click turning a second page', async (direction) => {
    const { viewport, next, previous } = setup(direction);
    viewport.dispatchEvent(new MouseEvent('click', { clientX: direction === 'ltr' ? 390 : 10 }));
    expect(next).toHaveBeenCalledOnce();
    viewport.dispatchEvent(new MouseEvent('click', { clientX: direction === 'ltr' ? 10 : 390 }));
    expect(previous).toHaveBeenCalledOnce();
    touch(viewport, direction === 'ltr' ? 300 : 100, direction === 'ltr' ? 100 : 300);
    viewport.dispatchEvent(new MouseEvent('click', { clientX: direction === 'ltr' ? 390 : 10 }));
    expect(next).toHaveBeenCalledTimes(2);
    touch(viewport, 100, 300, true);
    expect(next).toHaveBeenCalledTimes(2);
    expect(previous).toHaveBeenCalledOnce();
  });

  it('preloads two pages in both directions and keeps the old image until the requested page loads', async () => {
    const { host, state } = setup();
    await settle();
    expect(getSourceAsset.mock.calls.map((call) => call[2])).toEqual(['0.png', '1.png', '2.png', '3.png', '4.png']);
    host.querySelectorAll('img').forEach((img) => {
      if (img.alt === 'Page 2' || img.alt === 'Page 3') img.dispatchEvent(new Event('load'));
    });
    state.index = 3;
    await settle();
    const visible = () => [...host.querySelectorAll<HTMLElement>('.comic-page-image')].filter((el) => el.style.display !== 'none');
    expect(visible()[0].querySelector('img')?.alt).toBe('Page 3');
    host.querySelector<HTMLImageElement>('img[alt="Page 4"]')!.dispatchEvent(new Event('load'));
    await nextTick();
    expect(visible()[0].querySelector('img')?.alt).toBe('Page 4');
    expect(getSourceAsset.mock.calls.map((call) => call[2])).toContain('5.png');
    state.index = 1;
    await settle();
    expect(visible()[0].querySelector('img')?.alt).toBe('Page 2');
    expect(host.querySelectorAll('.comic-page-image').length).toBeLessThanOrEqual(6);
  });

  it('preserves missing and undecodable page positions and permits navigation', async () => {
    const { host, viewport, next } = setup('ltr', true);
    await settle();
    expect(host.textContent).toContain('Page 3 is unavailable');
    expect(getSourceAsset.mock.calls.map((call) => call[2])).not.toContain('2.png');
    viewport.dispatchEvent(new MouseEvent('click', { clientX: 390 }));
    await settle();
    host.querySelector<HTMLImageElement>('img[alt="Page 4"]')!.dispatchEvent(new Event('error'));
    await settle();
    expect(host.textContent).toContain('Page 4 is unavailable');
    expect(next).toHaveBeenCalledOnce();
  });

  it('renders an empty source without constructing an image', () => {
    const { host } = mount(ComicReaderView, { props: {
      bookId: 'empty', sourceId: 'pages', title: 'Empty', pages: [], index: 0, direction: 'ltr',
      percent: 0, loading: false, error: '', saveError: '', showBackNavigation: true
    } });
    expect(host.textContent).toContain('This source has no pages.');
    expect(host.querySelector('img')).toBeNull();
  });
});
