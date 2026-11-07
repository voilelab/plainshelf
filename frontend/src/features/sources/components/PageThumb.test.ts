// @vitest-environment jsdom
import { nextTick } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({ getSourceAsset: vi.fn() }));

vi.mock('@/providers', () => ({
  getBookshelfProvider: () => mocks
}));

import PageThumb from './PageThumb.vue';
import { mount } from '#testing/mount';

class MockIntersectionObserver {
  static instances: MockIntersectionObserver[] = [];
  readonly elements: Element[] = [];
  constructor(private readonly callback: IntersectionObserverCallback) {
    MockIntersectionObserver.instances.push(this);
  }
  observe(el: Element): void {
    this.elements.push(el);
  }
  unobserve(): void {}
  disconnect(): void {}
  emit(isIntersecting: boolean): void {
    this.callback(
      [{ target: this.elements[0], isIntersecting } as unknown as IntersectionObserverEntry],
      this as unknown as IntersectionObserver
    );
  }
}

async function flush(): Promise<void> {
  for (let i = 0; i < 4; i += 1) {
    await Promise.resolve();
    await nextTick();
  }
}

describe('PageThumb', () => {
  beforeEach(() => {
    MockIntersectionObserver.instances = [];
    mocks.getSourceAsset.mockReset();
    mocks.getSourceAsset.mockResolvedValue(new Blob(['x'], { type: 'image/png' }));
    vi.stubGlobal('IntersectionObserver', MockIntersectionObserver);
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:page'), revokeObjectURL: vi.fn() }));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('fetches a page image only once it scrolls near the viewport', async () => {
    const { host } = mount(PageThumb, { props: { bookId: 'book', sourceId: 'src', name: 'far.png', alt: 'Page 9' } });
    await flush();
    expect(mocks.getSourceAsset).not.toHaveBeenCalled();
    expect(host.querySelector('img')).toBeNull();

    MockIntersectionObserver.instances[0].emit(true);
    await flush();
    expect(mocks.getSourceAsset).toHaveBeenCalledWith('book', 'src', 'far.png');
    expect(host.querySelector('img')?.getAttribute('src')).toBe('blob:page');

    MockIntersectionObserver.instances[0].emit(false);
    await flush();
    expect(host.querySelector('img')).toBeNull();
  });

  it('never fetches a missing page', async () => {
    mount(PageThumb, { props: { bookId: 'book', sourceId: 'src', name: 'gone.png', alt: 'Page 2', missing: true } });
    MockIntersectionObserver.instances[0]?.emit(true);
    await flush();
    expect(mocks.getSourceAsset).not.toHaveBeenCalled();
  });
});
