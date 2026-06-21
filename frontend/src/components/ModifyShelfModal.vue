<template>
  <ConfirmModal
    :open="open"
    :title="t('settings.shelves.modifyShelfTitle')"
    :confirm-text="t('settings.shelves.modifyShelfSubmit')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('settings.shelves.modifyShelfSaving')"
    :busy="busy"
    :confirm-disabled="!canSubmit"
    :close-label="t('settings.shelves.modifyShelfCloseLabel')"
    @cancel="emit('cancel')"
    @confirm="onConfirm"
  >
    <form class="modify-shelf-form" @submit.prevent="onConfirm">
      <label class="modify-shelf-field">
        <span class="modify-shelf-label">{{ t('settings.shelves.modifyShelfId') }}</span>
        <input
          class="modify-shelf-input modify-shelf-input-readonly"
          type="text"
          :value="shelf?.id ?? ''"
          readonly
          tabindex="-1"
        />
      </label>
      <label class="modify-shelf-field">
        <span class="modify-shelf-label">{{ t('settings.shelves.modifyShelfPath') }}</span>
        <input
          class="modify-shelf-input modify-shelf-input-readonly"
          type="text"
          :value="shelf?.path ?? ''"
          readonly
          tabindex="-1"
        />
      </label>
      <label class="modify-shelf-field">
        <span class="modify-shelf-label">{{ t('settings.shelves.modifyShelfName') }}</span>
        <input
          v-model="editName"
          class="modify-shelf-input"
          type="text"
          :disabled="busy"
          autofocus
        />
      </label>
      <label class="modify-shelf-field">
        <span class="modify-shelf-label">{{ t('settings.shelves.modifyShelfScanInterval') }}</span>
        <input
          v-model="editScanInterval"
          class="modify-shelf-input"
          type="text"
          :placeholder="t('settings.shelves.addShelfScanIntervalPlaceholder')"
          :disabled="busy"
        />
      </label>
      <p class="modify-shelf-help">{{ t('settings.shelves.modifyShelfScanIntervalHelp') }}</p>
      <p v-if="error" class="modify-shelf-error" role="alert">{{ error }}</p>
    </form>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import ConfirmModal from './ConfirmModal.vue';
import type { ShelfInfo } from '../api/shelves';
import { useI18n } from '../i18n';

const props = defineProps<{
  open: boolean;
  shelf: ShelfInfo | null;
  busy: boolean;
  error: string;
}>();

const emit = defineEmits<{
  cancel: [];
  confirm: [name: string, scanInterval: string];
}>();

const { t } = useI18n();

const editName = ref('');
const editScanInterval = ref('');

const canSubmit = computed(() => editName.value.trim().length > 0);

watch(
  () => props.shelf,
  (shelf) => {
    editName.value = shelf?.name ?? '';
    editScanInterval.value = shelf?.scan_interval ?? '';
  },
  { immediate: true }
);

function onConfirm(): void {
  if (!canSubmit.value) return;
  emit('confirm', editName.value.trim(), editScanInterval.value.trim());
}
</script>

<style scoped>
.modify-shelf-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.modify-shelf-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.modify-shelf-label {
  color: #475569;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.modify-shelf-input {
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  padding: 7px 10px;
}

.modify-shelf-input:disabled {
  cursor: not-allowed;
}

.modify-shelf-input-readonly {
  background: #f8fafc;
  color: #64748b;
  cursor: default;
}

.modify-shelf-help {
  color: #64748b;
  font-size: 12px;
  margin: -4px 0 0;
}

.modify-shelf-error {
  color: #b91c1c;
  font-size: 13px;
  margin: 0;
}
</style>
