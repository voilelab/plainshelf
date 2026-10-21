// @vitest-environment jsdom
import { effectScope, ref } from 'vue';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useBookItemInteractions } from './useBookItemInteractions';
import type { Book } from '@/types/book';
import type { BookActivation } from '@/types/bookSelection';


const LONG_PRESS_MS = 450;

function touch(id: string, x: number, y: number): PointerEvent {
  return { pointerType: 'touch', clientX: x, clientY: y } as PointerEvent;
}

function book(id: string, title = id): Book {
  return { id, title } as Book;
}

function dragEvent() {
  const data = new Map<string, string>();
  const dataTransfer = {
    effectAllowed: '',
    setData: (type: string, value: string) => data.set(type, value),
    setDragImage: vi.fn()
  };
  return { data, event: { dataTransfer, preventDefault: vi.fn() } as unknown as DragEvent };
}

interface Harness {
  activations: BookActivation[];
  longPresses: string[];
  api: ReturnType<typeof useBookItemInteractions>;
  stop: () => void;
}

function setup(options: { mobile: boolean; selected?: string[] }): Harness {
  const activations: BookActivation[] = [];
  const longPresses: string[] = [];
  const scope = effectScope();
  let api!: ReturnType<typeof useBookItemInteractions>;
  scope.run(() => {
    api = useBookItemInteractions({
      mobile: ref(options.mobile),
      selectedIds: ref(new Set(options.selected ?? [])),
      onActivate: (payload) => activations.push(payload),
      onLongPress: (id) => longPresses.push(id)
    });
  });
  return { activations, longPresses, api, stop: () => scope.stop() };
}

let harness: Harness | null = null;

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  harness?.stop();
  harness = null;
  vi.useRealTimers();
  for (const preview of document.querySelectorAll('.book-drag-preview')) {
    preview.remove();
  }
});

describe('long press on a touch device', () => {
  it('starts a selection once the press is held long enough', () => {
    harness = setup({ mobile: true });

    harness.api.onPointerDown(touch('b1', 24, 24), 'b1');
    expect(harness.longPresses).toEqual([]);

    vi.advanceTimersByTime(LONG_PRESS_MS);

    expect(harness.longPresses).toEqual(['b1']);
  });

  it('cancels the pending press when the finger slides past the tolerance', () => {
    harness = setup({ mobile: true });

    harness.api.onPointerDown(touch('b1', 20, 20), 'b1');
    harness.api.onPointerMove(touch('b1', 20, 45));
    vi.advanceTimersByTime(LONG_PRESS_MS);

    expect(harness.longPresses).toEqual([]);
  });

  it('tolerates the small drift a stationary finger reports', () => {
    harness = setup({ mobile: true });

    harness.api.onPointerDown(touch('b1', 20, 20), 'b1');
    harness.api.onPointerMove(touch('b1', 24, 28));
    vi.advanceTimersByTime(LONG_PRESS_MS);

    expect(harness.longPresses).toEqual(['b1']);
  });

  it('swallows the compatibility click a real touch emits after the press', () => {
    harness = setup({ mobile: true });
    harness.api.onPointerDown(touch('b1', 24, 24), 'b1');
    vi.advanceTimersByTime(LONG_PRESS_MS);

    const click = { metaKey: false, ctrlKey: false, shiftKey: false, preventDefault: vi.fn() };
    harness.api.onClick(click as unknown as MouseEvent, 'b1');

    expect(harness.activations).toEqual([]);
    expect(click.preventDefault).toHaveBeenCalled();

    harness.api.onClick(click as unknown as MouseEvent, 'b1');
    expect(harness.activations).toHaveLength(1);
  });

  it('ignores a mouse press, and a touch press on the desktop shell', () => {
    harness = setup({ mobile: true });
    harness.api.onPointerDown({ pointerType: 'mouse', clientX: 1, clientY: 1 } as PointerEvent, 'b1');
    vi.advanceTimersByTime(LONG_PRESS_MS);
    expect(harness.longPresses).toEqual([]);
    harness.stop();

    harness = setup({ mobile: false });
    harness.api.onPointerDown(touch('b1', 1, 1), 'b1');
    vi.advanceTimersByTime(LONG_PRESS_MS);
    expect(harness.longPresses).toEqual([]);
  });

  it('forwards the modifier keys a click carries, so range-select still works', () => {
    harness = setup({ mobile: false });

    harness.api.onClick(
      { metaKey: true, ctrlKey: false, shiftKey: false, preventDefault: vi.fn() } as unknown as MouseEvent,
      'b2'
    );

    expect(harness.activations).toEqual([{ id: 'b2', metaKey: true, ctrlKey: false, shiftKey: false }]);
  });
});

describe('dragging a book', () => {
  it('drags an unselected book on its own, ignoring the current selection', () => {
    harness = setup({ mobile: false, selected: ['b1', 'b2'] });
    const { data, event } = dragEvent();

    harness.api.onDragStart(event, book('b3', 'Solaris'));

    expect(data.has('application/x-plainshelf-book-ids')).toBe(false);
    expect(data.get('application/x-plainshelf-book-id')).toBe('b3');
    expect(harness.api.draggingBookId.value).toBe('b3');
    expect(document.querySelector('.book-drag-preview')?.textContent).toBe('Solaris');
  });

  it('drags the whole selection when the dragged book belongs to it', () => {
    harness = setup({ mobile: false, selected: ['b1', 'b2'] });
    const { data, event } = dragEvent();

    harness.api.onDragStart(event, book('b1', 'Solaris'));

    expect(JSON.parse(data.get('application/x-plainshelf-book-ids')!).sort()).toEqual(['b1', 'b2']);
    expect(document.querySelector('.book-drag-preview')?.textContent).toBe('Solaris · 2');
  });

  it('refuses a drag on the mobile shell, where a press means select', () => {
    harness = setup({ mobile: true, selected: ['b1'] });
    const { data, event } = dragEvent();

    harness.api.onDragStart(event, book('b1'));

    expect(event.preventDefault).toHaveBeenCalled();
    expect(data.size).toBe(0);
    expect(harness.api.draggingBookId.value).toBeNull();
  });

  it('takes the preview back out of the document when the drag ends', () => {
    harness = setup({ mobile: false });
    const { event } = dragEvent();
    harness.api.onDragStart(event, book('b1'));
    expect(document.querySelectorAll('.book-drag-preview')).toHaveLength(1);

    harness.api.onDragEnd();

    expect(document.querySelectorAll('.book-drag-preview')).toHaveLength(0);
    expect(harness.api.draggingBookId.value).toBeNull();
  });
});
