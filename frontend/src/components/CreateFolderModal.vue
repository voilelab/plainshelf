<template>
  <ConfirmModal
    :open="open"
    :title="t('layout.createFolder.title')"
    :confirm-text="t('layout.createFolder.create')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('layout.createFolder.creating')"
    :busy="busy"
    :confirm-disabled="!canSubmit"
    :close-label="t('layout.createFolder.closeLabel')"
    @cancel="emit('cancel')"
    @confirm="submitCreate"
  >
    <form class="create-folder-form" @submit.prevent="submitCreate">
      <div class="create-folder-field">
        <label class="create-folder-label" :for="nameInputId">
          {{ t('layout.createFolder.nameLabel') }}
        </label>
        <input
          :id="nameInputId"
          ref="nameInput"
          v-model="draftName"
          class="create-folder-input"
          type="text"
          :placeholder="t('layout.createFolder.namePlaceholder')"
          :disabled="busy"
          autocomplete="off"
        >
      </div>

      <p v-if="displayError" class="create-folder-error" role="alert">{{ displayError }}</p>
    </form>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue';
import ConfirmModal from './ConfirmModal.vue';
import { useI18n } from '@/i18n';

const props = withDefaults(
  defineProps<{
    open: boolean;
    busy?: boolean;
    error?: string;
  }>(),
  {
    busy: false,
    error: ''
  }
);

const emit = defineEmits<{
  cancel: [];
  submit: [payload: { name: string }];
}>();

const { t } = useI18n();
const draftName = ref('');
const nameInput = ref<HTMLInputElement | null>(null);
const nameInputId = `create-folder-name-${useId()}`;

const trimmedDraftName = computed(() => draftName.value.trim());
const nameHasSeparator = computed(() => trimmedDraftName.value.includes('/'));
const canSubmit = computed(
  () => !props.busy && trimmedDraftName.value.length > 0 && !nameHasSeparator.value
);
const displayError = computed(() =>
  nameHasSeparator.value ? t('layout.createFolder.invalidName') : props.error
);

async function focusNameInput(): Promise<void> {
  // ConfirmModal focuses its confirm button one tick after `open` flips, so wait
  // a second tick to land after it. Same approach as RenameFolderModal.
  await nextTick();
  await nextTick();
  nameInput.value?.focus();
}

function submitCreate(): void {
  if (!canSubmit.value) {
    return;
  }

  emit('submit', { name: trimmedDraftName.value });
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }

    draftName.value = '';
    void focusNameInput();
  }
);
</script>

<style scoped>
.create-folder-form {
  display: grid;
  gap: 12px;
}

.create-folder-field {
  display: grid;
  gap: 6px;
}

.create-folder-label {
  color: var(--muted);
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
}

.create-folder-input {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  font: inherit;
  padding: 10px 12px;
}

.create-folder-input:focus {
  border-color: var(--accent);
  outline: 2px solid rgba(37, 99, 235, 0.18);
}

.create-folder-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  margin: 0;
  padding: 10px;
  white-space: pre-line;
}
</style>
