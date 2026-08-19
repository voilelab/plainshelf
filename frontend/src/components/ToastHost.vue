<template>
  <ToastProvider :duration="TOAST_DURATION_MS" :label="t('toast.label')">
    <ToastRoot
      v-for="toast in toasts"
      :key="toast.id"
      :class="['reka-toast', `reka-toast-${toast.variant}`]"
      :type="toast.variant === 'danger' ? 'foreground' : 'background'"
      :open="true"
      @update:open="onUpdateOpen(toast.id, $event)"
    >
      <ToastDescription class="reka-toast-message">{{ toast.message }}</ToastDescription>
      <ToastClose class="reka-toast-close" :aria-label="t('toast.dismiss')">✕</ToastClose>
    </ToastRoot>
    <ToastViewport class="reka-toast-viewport" />
  </ToastProvider>
</template>

<script setup lang="ts">
import { ToastClose, ToastDescription, ToastProvider, ToastRoot, ToastViewport } from 'reka-ui';
import { useToasts } from '@/composables/useToasts';
import { useI18n } from '@/i18n';

// Long enough to read a sentence without the message becoming furniture. Reka
// pauses the timer while the pointer is over the viewport or a toast has focus,
// so a slower reader is not fighting it.
const TOAST_DURATION_MS = 6000;

const { t } = useI18n();
const { toasts, dismissToast } = useToasts();

// The queue owns which toasts exist; Reka only reports that one has finished.
// Removing it here rather than tracking an `open` flag per toast keeps a
// dismissed toast from staying in the list as an invisible entry.
function onUpdateOpen(id: number, open: boolean): void {
  if (!open) {
    dismissToast(id);
  }
}
</script>

<!--
  The styling lives in styles.css as `.reka-toast*`, alongside `.reka-menu` and
  `.reka-tooltip`, and for the same reason: ToastRoot renders a fragment and
  teleports its body into the viewport, so this component's `scoped` attribute
  never lands on either element and a scoped rule silently does nothing.
-->
