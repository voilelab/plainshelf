import { hasFileTransfer } from '@/utils/file';

// @wailsio/runtime 3.1.0 (window.ts) sets dropEffect to 'none' on documentElement
// for every file drag while EnableFileDrop is off, cancelling drops a page accepted.
// Registered before the runtime loads, this listener runs first and stops it.
export function guardFileDrops(root: HTMLElement): void {
  root.addEventListener('dragenter', keepAcceptedFileDrag);
  root.addEventListener('dragover', keepAcceptedFileDrag);
}

function keepAcceptedFileDrag(event: DragEvent): void {
  if (event.defaultPrevented && hasFileTransfer(event.dataTransfer)) {
    event.stopImmediatePropagation();
  }
}
