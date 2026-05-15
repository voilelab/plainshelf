<template>
  <ConfirmModal
    :open="open"
    :title="title"
    :confirm-text="confirmText"
    :cancel-text="cancelText"
    :busy-text="busyText"
    :busy="busy"
    variant="danger"
    close-label="Close delete confirmation dialog"
    @cancel="emit('cancel')"
    @confirm="emit('confirm')"
  >
    <p>
      Delete <strong>{{ itemName }}</strong>?
    </p>
    <p v-if="description">{{ description }}</p>
    <p v-if="error" class="delete-modal-error" role="alert">{{ error }}</p>
  </ConfirmModal>
</template>

<script setup lang="ts">
import ConfirmModal from './ConfirmModal.vue';

withDefaults(
  defineProps<{
    open: boolean;
    itemName: string;
    title?: string;
    description?: string;
    confirmText?: string;
    cancelText?: string;
    busyText?: string;
    busy?: boolean;
    error?: string;
  }>(),
  {
    title: 'Confirm delete',
    description: 'This cannot be undone.',
    confirmText: 'Delete',
    cancelText: 'Cancel',
    busyText: 'Deleting...',
    busy: false,
    error: ''
  }
);

const emit = defineEmits<{
  cancel: [];
  confirm: [];
}>();
</script>

<style scoped>
.delete-modal-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  padding: 10px;
  white-space: pre-line;
}
</style>
