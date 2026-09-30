<template>
  <AlertDialogRoot :open="open" @update:open="onUpdateOpen">
    <AlertDialogPortal>
      <AlertDialogOverlay class="base-alert-dialog-overlay" />
      <AlertDialogContent
        class="base-alert-dialog-content"
        :aria-describedby="describedBy"
        @escape-key-down="onEscapeKeyDown"
      >
        <VisuallyHidden>
          <AlertDialogTitle>{{ title }}</AlertDialogTitle>
        </VisuallyHidden>
        <slot />
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>

<script setup lang="ts">
import {
  AlertDialogContent,
  AlertDialogOverlay,
  AlertDialogPortal,
  AlertDialogRoot,
  AlertDialogTitle,
  VisuallyHidden
} from 'reka-ui';

// The destructive counterpart of BaseDialog. Two differences carry the whole
// point of the split, and both come from AlertDialogContent rather than from
// anything here: it renders role="alertdialog", and it prevents every
// outside interaction, so a stray click on the backdrop can never confirm
// away a delete. There is deliberately no `dismissible` prop — an alert
// dialog is dismissed by its own buttons or by ESC, never by the backdrop.
//
// Focus lands on the element rendered by AlertDialogCancel, which the caller
// supplies in the slot; see ConfirmModal.vue.
const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    busy?: boolean;
    describedBy?: string;
  }>(),
  {
    busy: false,
    describedBy: undefined
  }
);

const emit = defineEmits<{
  close: [];
}>();

function onUpdateOpen(value: boolean): void {
  if (!value) {
    emit('close');
  }
}

function onEscapeKeyDown(event: Event): void {
  // Same reasoning as BaseDialog: always consume the key event so the native
  // desktop shell (e.g. macOS fullscreen) does not also react to ESC, then
  // close manually because Reka UI checks defaultPrevented before dismissing.
  event.preventDefault();
  if (!props.busy) {
    emit('close');
  }
}
</script>

<style scoped>
/* Kept in step with BaseDialog.vue: the two shells must be visually
   indistinguishable, only their semantics differ. */
.base-alert-dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.38);
  z-index: var(--z-modal);
}

.base-alert-dialog-content {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: var(--z-modal);
  max-width: calc(100vw / var(--app-zoom, 1) - 32px);
  max-height: calc(100vh / var(--app-zoom, 1) - 32px);
}
</style>
