<template>
  <ConfirmModal
    :open="open"
    :title="t('layout.renameLayer.title')"
    :confirm-text="t('layout.renameLayer.confirm')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('layout.renameLayer.renaming')"
    :busy="busy"
    :close-label="t('layout.renameLayer.closeLabel')"
    @cancel="emit('cancel')"
    @confirm="submitRename"
  >
    <form class="rename-layer-form" @submit.prevent="submitRename">
      <label class="rename-layer-field">
        <span class="rename-layer-label">{{ t('layout.renameLayer.nameLabel') }}</span>
        <input
          ref="nameInput"
          v-model="draftName"
          class="rename-layer-input"
          type="text"
          :placeholder="t('layout.renameLayer.placeholder')"
          :disabled="busy"
          autocomplete="off"
        >
      </label>
      <p class="rename-layer-help">{{ t('layout.renameLayer.help', { layerName: currentName }) }}</p>
      <p v-if="error" class="rename-layer-error" role="alert">{{ error }}</p>
    </form>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import ConfirmModal from './ConfirmModal.vue';
import { useI18n } from '../i18n';

const props = withDefaults(
  defineProps<{
    open: boolean;
    currentName: string;
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
  submit: [name: string];
}>();

const { t } = useI18n();
const draftName = ref('');
const nameInput = ref<HTMLInputElement | null>(null);
const trimmedDraftName = computed(() => draftName.value.trim());

async function focusNameInput(): Promise<void> {
  await nextTick();
  await nextTick();
  nameInput.value?.focus();
  nameInput.value?.select();
}

function submitRename(): void {
  if (props.busy) {
    return;
  }

  emit('submit', trimmedDraftName.value);
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }

    draftName.value = props.currentName;
    void focusNameInput();
  }
);

watch(
  () => props.currentName,
  (name) => {
    if (props.open) {
      draftName.value = name;
      void focusNameInput();
    }
  }
);
</script>

<style scoped>
.rename-layer-form {
  display: grid;
  gap: 10px;
}

.rename-layer-field {
  display: grid;
  gap: 6px;
}

.rename-layer-label {
  color: var(--muted);
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
}

.rename-layer-input {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  font: inherit;
  padding: 10px 12px;
}

.rename-layer-input:focus {
  border-color: var(--accent);
  outline: 2px solid rgba(37, 99, 235, 0.18);
}

.rename-layer-help {
  color: var(--muted);
  font-size: 13px;
  margin: 0;
}

.rename-layer-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  margin: 0;
  padding: 10px;
  white-space: pre-line;
}
</style>
