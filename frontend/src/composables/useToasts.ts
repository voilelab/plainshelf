import { readonly, ref } from 'vue';

type ToastVariant = 'info' | 'danger';

interface ToastAction {
  label: string;
  run: () => void;
}

interface Toast {
  id: number;
  message: string;
  variant: ToastVariant;
  action?: ToastAction;
}

interface ShowToastOptions {
  variant?: ToastVariant;
  action?: ToastAction;
}

const toasts = ref<Toast[]>([]);
let nextID = 0;

export function useToasts() {
  function showToast(message: string, options?: ShowToastOptions): number {
    const id = ++nextID;
    toasts.value = [
      ...toasts.value,
      { id, message, variant: options?.variant ?? 'info', action: options?.action }
    ];
    return id;
  }

  function dismissToast(id: number): void {
    toasts.value = toasts.value.filter((toast) => toast.id !== id);
  }

  return { toasts: readonly(toasts), showToast, dismissToast };
}
