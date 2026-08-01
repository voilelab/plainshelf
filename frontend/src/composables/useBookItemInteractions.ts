import { onBeforeUnmount, ref, toValue, type MaybeRefOrGetter } from 'vue';
import type { Book } from '@/types/book';
import type { BookActivation } from '@/types/bookSelection';

const LONG_PRESS_MS = 450;
const LONG_PRESS_MOVE_TOLERANCE = 10;
export const BOOK_IDS_DRAG_TYPE = 'application/x-plainshelf-book-ids';

export interface UseBookItemInteractionsOptions {
  mobile: MaybeRefOrGetter<boolean>;
  selectedIds: MaybeRefOrGetter<ReadonlySet<string>>;
  onActivate: (payload: BookActivation) => void;
  onLongPress: (id: string) => void;
}

export function useBookItemInteractions(options: UseBookItemInteractionsOptions) {
  const draggingBookId = ref<string | null>(null);
  let longPressTimer: ReturnType<typeof setTimeout> | null = null;
  let pointerStart: { id: string; x: number; y: number } | null = null;
  let suppressNextClick = false;
  let dragPreviewEl: HTMLElement | null = null;

  function cancelLongPress(): void {
    if (longPressTimer !== null) clearTimeout(longPressTimer);
    longPressTimer = null;
    pointerStart = null;
  }

  function onPointerDown(event: PointerEvent, id: string): void {
    if (!toValue(options.mobile) || event.pointerType === 'mouse') return;
    cancelLongPress();
    pointerStart = { id, x: event.clientX, y: event.clientY };
    longPressTimer = setTimeout(() => {
      suppressNextClick = true;
      options.onLongPress(id);
      cancelLongPress();
    }, LONG_PRESS_MS);
  }

  function onPointerMove(event: PointerEvent): void {
    if (!pointerStart) return;
    if (
      Math.abs(event.clientX - pointerStart.x) > LONG_PRESS_MOVE_TOLERANCE ||
      Math.abs(event.clientY - pointerStart.y) > LONG_PRESS_MOVE_TOLERANCE
    ) cancelLongPress();
  }

  function onClick(event: MouseEvent, id: string): void {
    cancelLongPress();
    if (suppressNextClick) {
      suppressNextClick = false;
      event.preventDefault();
      return;
    }
    options.onActivate({ id, metaKey: event.metaKey, ctrlKey: event.ctrlKey, shiftKey: event.shiftKey });
  }

  function cleanupDragPreview(): void {
    dragPreviewEl?.remove();
    dragPreviewEl = null;
  }

  function onDragStart(event: DragEvent, book: Book): void {
    if (toValue(options.mobile)) {
      event.preventDefault();
      return;
    }
    const selected = toValue(options.selectedIds);
    const draggingSelection = selected.has(book.id);
    const ids = draggingSelection ? [...selected] : [book.id];
    draggingBookId.value = book.id;
    if (draggingSelection) event.dataTransfer?.setData(BOOK_IDS_DRAG_TYPE, JSON.stringify(ids));
    event.dataTransfer?.setData('application/x-plainshelf-book-id', book.id);
    event.dataTransfer?.setData('text/plain', book.id);
    if (!event.dataTransfer) return;

    event.dataTransfer.effectAllowed = 'move';
    const preview = document.createElement('div');
    preview.className = 'book-drag-preview';
    preview.textContent = ids.length > 1 ? `${book.title} · ${ids.length}` : book.title;
    document.body.appendChild(preview);
    dragPreviewEl = preview;
    event.dataTransfer.setDragImage(preview, 24, 24);
  }

  function onDragEnd(): void {
    draggingBookId.value = null;
    cleanupDragPreview();
  }

  onBeforeUnmount(() => {
    cancelLongPress();
    cleanupDragPreview();
  });

  return { draggingBookId, onClick, onPointerDown, onPointerMove, cancelLongPress, onDragStart, onDragEnd };
}
