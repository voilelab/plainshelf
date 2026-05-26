<template>
  <ConfirmModal
    :open="open"
    :title="title"
    :confirm-text="confirmText"
    :cancel-text="cancelText"
    :busy-text="busyText"
    :busy="busy"
    variant="danger"
    :close-label="t('deleteModal.closeLabel')"
    @cancel="emit('cancel')"
    @confirm="emit('confirm')"
  >
    <p>
      {{ t('deleteModal.question', { itemName }) }}
    </p>
    <p v-if="description">{{ description }}</p>
    <p v-if="error" class="delete-modal-error" role="alert">{{ error }}</p>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, toRefs } from 'vue';
import ConfirmModal from './ConfirmModal.vue';
import { useI18n } from '../i18n';

const { t } = useI18n();

const props = withDefaults(
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
    busy: false,
    error: ''
  }
);

const title = computed(() => props.title ?? t('deleteModal.title'));
const description = computed(() => props.description ?? t('deleteModal.description'));
const confirmText = computed(() => props.confirmText ?? t('deleteModal.confirm'));
const cancelText = computed(() => props.cancelText ?? t('deleteModal.cancel'));
const busyText = computed(() => props.busyText ?? t('deleteModal.busy'));
const {
  open,
  itemName,
  busy,
  error
} = toRefs(props);

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
