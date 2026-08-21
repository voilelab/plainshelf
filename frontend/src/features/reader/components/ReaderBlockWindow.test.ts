// @vitest-environment jsdom
import { createApp, h, nextTick, ref, type App } from 'vue';
import { afterEach, describe, expect, it } from 'vitest';
import ReaderBlockWindow from './ReaderBlockWindow.vue';

interface Item {
  label: string;
}

/** A controllable IntersectionObserver stub the tests drive by hand. */
class MockIntersectionObserver {
  static instances: MockIntersectionObserver[] = [];
  readonly elements = new Set<Element>();
  constructor(private readonly callback: IntersectionObserverCallback) {
    MockIntersectionObserver.instances.push(this);
  }
  observe(el: Element): void {
    this.elements.add(el);
  }
  unobserve(el: Element): void {
    this.elements.delete(el);
  }
  disconnect(): void {
    this.elements.clear();
  }
  emit(target: Element, isIntersecting: boolean): void {
    this.callback(
      [{ target, isIntersecting } as unknown as IntersectionObserverEntry],
      this as unknown as IntersectionObserver
    );
  }
}

let app: App | null = null;
let host: HTMLElement | null = null;

function mount(items: Item[]): HTMLElement {
  const list = ref(items);
  host = document.createElement('div');
  document.body.append(host);
  app = createApp({
    setup: () => () =>
      h(
        ReaderBlockWindow,
        { items: list.value, estimate: (item: unknown) => 100 + (item as Item).label.length },
        { default: ({ item }: { item: Item }) => h('div', { class: 'content' }, item.label) }
      )
  });
  app.mount(host);
  return host;
}

function wrappers(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>('[data-window-index]'));
}

afterEach(() => {
  app?.unmount();
  host?.remove();
  app = null;
  host = null;
  MockIntersectionObserver.instances.length = 0;
  delete (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver;
});

describe('ReaderBlockWindow', () => {
  it('mounts every block when IntersectionObserver is unavailable', () => {
    const root = mount([{ label: 'A' }, { label: 'B' }, { label: 'C' }]);
    const contents = root.querySelectorAll('.content');
    expect(Array.from(contents, (el) => el.textContent)).toEqual(['A', 'B', 'C']);
    // Nothing is collapsed, so no placeholder height is imposed.
    expect(wrappers(root).every((el) => el.style.minHeight === '')).toBe(true);
  });

  it('renders only the first block eagerly and holds the rest as placeholders', () => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      MockIntersectionObserver;
    const root = mount([{ label: 'A' }, { label: 'B' }, { label: 'C' }]);

    const contents = root.querySelectorAll('.content');
    expect(Array.from(contents, (el) => el.textContent)).toEqual(['A']);
    // The off-screen blocks reserve their estimated height so scrollHeight holds.
    const [, second, third] = wrappers(root);
    expect(second.style.minHeight).toBe('101px');
    expect(third.style.minHeight).toBe('101px');
  });

  it('mounts a block once the observer reports it near the viewport', async () => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      MockIntersectionObserver;
    const root = mount([{ label: 'A' }, { label: 'B' }, { label: 'C' }]);
    const observer = MockIntersectionObserver.instances[0];
    const [, second] = wrappers(root);

    observer.emit(second, true);
    await nextTick();

    expect(second.querySelector('.content')?.textContent).toBe('B');
    expect(second.style.minHeight).toBe('');

    observer.emit(second, false);
    await nextTick();

    expect(second.querySelector('.content')).toBeNull();
  });
});
