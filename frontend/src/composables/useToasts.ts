import { readonly, ref } from 'vue';

export type ToastVariant = 'info' | 'danger';

export interface Toast {
  id: number;
  message: string;
  variant: ToastVariant;
}

export interface ShowToastOptions {
  variant?: ToastVariant;
}

// Module-level, like useBookStore: anything in the app can raise a toast, and
// the single host mounted in App.vue is what renders them all.
const toasts = ref<Toast[]>([]);
let nextID = 0;

/**
 * Transient feedback that does not belong in the layout.
 *
 * For the result of something the user just did — "found 3,183 books" after a
 * rescan — where a permanent line would take space away from the page for a
 * message that stops being interesting a second later. State the page has to
 * keep showing (a load failure, a shelf that is still starting) stays on the
 * page as an error or status region instead; a toast that has come and gone
 * cannot be re-read.
 */
export function useToasts() {
  function showToast(message: string, options?: ShowToastOptions): number {
    const id = ++nextID;
    toasts.value = [...toasts.value, { id, message, variant: options?.variant ?? 'info' }];
    return id;
  }

  function dismissToast(id: number): void {
    toasts.value = toasts.value.filter((toast) => toast.id !== id);
  }

  return { toasts: readonly(toasts), showToast, dismissToast };
}
