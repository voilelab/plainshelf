<template>
  <DialogRoot :open="open" @update:open="onUpdateOpen">
    <DialogPortal>
      <DialogOverlay class="base-dialog-overlay" />
      <DialogContent
        class="base-dialog-content"
        :aria-describedby="describedBy"
        @escape-key-down="onDismissAttempt"
        @pointer-down-outside="onDismissAttempt"
      >
        <VisuallyHidden>
          <DialogTitle>{{ title }}</DialogTitle>
        </VisuallyHidden>
        <slot />
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, VisuallyHidden } from 'reka-ui';

const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    dismissible?: boolean;
    busy?: boolean;
    describedBy?: string;
  }>(),
  {
    dismissible: true,
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

function onDismissAttempt(event: Event): void {
  if (props.busy || !props.dismissible) {
    event.preventDefault();
  }
}
</script>

<style scoped>
.base-dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.38);
  z-index: var(--z-modal);
}

.base-dialog-content {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: var(--z-modal);
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 32px);
}
</style>
