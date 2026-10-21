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
  // Reka UI 2.10.5 DismissableLayer suppresses dismissal when the Escape event is prevented.
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
