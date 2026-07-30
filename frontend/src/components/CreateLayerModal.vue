<template>
  <ConfirmModal
    :open="open"
    :title="t('layout.createLayer.title')"
    :confirm-text="t('layout.createLayer.create')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('layout.createLayer.creating')"
    :busy="busy"
    :confirm-disabled="!canSubmit"
    :close-label="t('layout.createLayer.closeLabel')"
    @cancel="emit('cancel')"
    @confirm="submitCreate"
  >
    <form class="create-layer-form" @submit.prevent="submitCreate">
      <div class="create-layer-field">
        <label class="create-layer-label" :for="nameInputId">
          {{ t('layout.createLayer.nameLabel') }}
        </label>
        <input
          :id="nameInputId"
          ref="nameInput"
          v-model="draftName"
          class="create-layer-input"
          type="text"
          :placeholder="t('layout.createLayer.namePlaceholder')"
          :disabled="busy"
          autocomplete="off"
        >
      </div>

      <div class="create-layer-field">
        <!--
          SelectTrigger renders a button, which is not a labelable element, so a
          wrapping <label> would not associate with it. Point at the visible text
          with aria-labelledby instead.
        -->
        <span :id="parentLabelId" class="create-layer-label">
          {{ t('layout.createLayer.parentLabel') }}
        </span>
        <SelectRoot :model-value="draftParent" :disabled="busy" @update:model-value="onParentSelect">
          <SelectTrigger class="input select-trigger" :aria-labelledby="parentLabelId">
            <SelectValue />
          </SelectTrigger>
          <SelectPortal>
            <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
              <SelectViewport>
                <SelectItem
                  v-for="option in parentOptions"
                  :key="option.value"
                  class="reka-menu-item"
                  :value="option.value"
                  :style="{ paddingLeft: `${10 + option.depth * 14}px` }"
                >
                  <SelectItemText>{{ option.label }}</SelectItemText>
                </SelectItem>
              </SelectViewport>
            </SelectContent>
          </SelectPortal>
        </SelectRoot>
      </div>

      <p v-if="displayError" class="create-layer-error" role="alert">{{ displayError }}</p>
    </form>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue';
import {
  type AcceptableValue,
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport
} from 'reka-ui';
import ConfirmModal from './ConfirmModal.vue';
import { useI18n } from '../i18n';

export type CreateLayerParentOption = {
  /** Slash path, or ROOT_PARENT_VALUE for the top level. Never an empty string. */
  value: string;
  label: string;
  depth: number;
};

/**
 * Reka reserves the empty string for "no selection", so the top level is
 * represented by '/' here and normalized away by the parent on submit.
 */
const ROOT_PARENT_VALUE = '/';

const props = withDefaults(
  defineProps<{
    open: boolean;
    parentOptions: CreateLayerParentOption[];
    defaultParent?: string;
    busy?: boolean;
    error?: string;
  }>(),
  {
    defaultParent: ROOT_PARENT_VALUE,
    busy: false,
    error: ''
  }
);

const emit = defineEmits<{
  cancel: [];
  submit: [payload: { parentPath: string; name: string }];
}>();

const { t } = useI18n();
const draftName = ref('');
const draftParent = ref(ROOT_PARENT_VALUE);
const nameInput = ref<HTMLInputElement | null>(null);
const nameInputId = `create-layer-name-${useId()}`;
const parentLabelId = `create-layer-parent-${useId()}`;

const trimmedDraftName = computed(() => draftName.value.trim());
const nameHasSeparator = computed(() => trimmedDraftName.value.includes('/'));
const canSubmit = computed(
  () => !props.busy && trimmedDraftName.value.length > 0 && !nameHasSeparator.value
);
const displayError = computed(() =>
  nameHasSeparator.value ? t('layout.createLayer.invalidName') : props.error
);

function isKnownParent(value: string): boolean {
  return props.parentOptions.some((option) => option.value === value);
}

async function focusNameInput(): Promise<void> {
  // ConfirmModal focuses its confirm button one tick after `open` flips, so wait
  // a second tick to land after it. Same approach as RenameLayerModal.
  await nextTick();
  await nextTick();
  nameInput.value?.focus();
}

function onParentSelect(value: AcceptableValue): void {
  if (typeof value !== 'string' || !isKnownParent(value)) {
    return;
  }

  draftParent.value = value;
}

function submitCreate(): void {
  if (!canSubmit.value) {
    return;
  }

  emit('submit', { parentPath: draftParent.value, name: trimmedDraftName.value });
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return;
    }

    draftName.value = '';
    draftParent.value = isKnownParent(props.defaultParent) ? props.defaultParent : ROOT_PARENT_VALUE;
    void focusNameInput();
  }
);
</script>

<style scoped>
.create-layer-form {
  display: grid;
  gap: 12px;
}

.create-layer-field {
  display: grid;
  gap: 6px;
}

.create-layer-label {
  color: var(--muted);
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
}

.create-layer-input {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  font: inherit;
  padding: 10px 12px;
}

.create-layer-input:focus {
  border-color: var(--accent);
  outline: 2px solid rgba(37, 99, 235, 0.18);
}

.select-trigger {
  cursor: pointer;
  text-align: left;
}

.create-layer-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  margin: 0;
  padding: 10px;
  white-space: pre-line;
}
</style>
