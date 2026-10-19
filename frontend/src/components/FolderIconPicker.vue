<template>
  <div class="folder-icon-picker" role="radiogroup" :aria-label="t('layout.folderIcon.label')">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      class="folder-icon-option"
      :class="{ selected: modelValue === option.value }"
      :aria-checked="modelValue === option.value"
      :aria-label="option.label"
      :title="option.label"
      :disabled="disabled"
      @click="emit('update:modelValue', option.value)"
    >
      <FolderIcon :icon="option.value" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import FolderIcon from './FolderIcon.vue';
import { DEFAULT_FOLDER_ICON, FOLDER_ICON_NAMES } from '@/components/icons/folderIcons';
import { useI18n } from '@/i18n';

const props = defineProps<{
  /** The chosen icon name, '' for the default. */
  modelValue: string;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [icon: string];
}>();

const { t } = useI18n();

// The default comes first and is a choice of its own: it is what clearing an
// icon picks. It draws as the plain folder, so that name is offered again only
// to a folder that already stores it.
const options = computed(() => [
  { value: '', label: t('layout.folderIcon.default') },
  ...FOLDER_ICON_NAMES.filter((name) => name !== DEFAULT_FOLDER_ICON || props.modelValue === name).map((name) => ({
    value: name,
    label: name
  }))
]);
</script>

<style scoped>
.folder-icon-picker {
  display: grid;
  gap: 4px;
  grid-template-columns: repeat(auto-fill, minmax(36px, 1fr));
  /* The dialog it sits in caps its height without scrolling, so on a short
     window the grid scrolls on its own rather than push the dialog's buttons
     out of reach. vh is divided by --app-zoom like the dialog's own cap. */
  max-height: min(240px, calc(35vh / var(--app-zoom, 1)));
  overflow-y: auto;
  padding: 2px;
}

.folder-icon-option {
  align-items: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  cursor: pointer;
  display: inline-flex;
  height: 36px;
  justify-content: center;
}

.folder-icon-option:hover:not(:disabled) {
  border-color: var(--accent);
}

.folder-icon-option.selected {
  border-color: var(--accent);
  outline: 2px solid rgba(37, 99, 235, 0.18);
}

.folder-icon-option :deep(.folder-icon) {
  height: 20px;
  width: 20px;
}
</style>
