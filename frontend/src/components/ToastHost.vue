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
      <ToastAction
        v-if="toast.action"
        class="reka-toast-action"
        :alt-text="toast.action.label"
        @click="toast.action.run()"
      >
        {{ toast.action.label }}
      </ToastAction>
      <ToastClose class="reka-toast-close" :aria-label="t('toast.dismiss')">✕</ToastClose>
    </ToastRoot>
    <ToastViewport class="reka-toast-viewport" />
  </ToastProvider>
</template>

<script setup lang="ts">
import { ToastAction, ToastClose, ToastDescription, ToastProvider, ToastRoot, ToastViewport } from 'reka-ui';
import { useToasts } from '@/composables/useToasts';
import { useI18n } from '@/i18n';

const TOAST_DURATION_MS = 6000;

const { t } = useI18n();
const { toasts, dismissToast } = useToasts();

function onUpdateOpen(id: number, open: boolean): void {
  if (!open) {
    dismissToast(id);
  }
}
</script>
