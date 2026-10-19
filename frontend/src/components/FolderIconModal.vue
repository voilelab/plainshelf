<template>
  <ConfirmModal
    :open="open"
    :title="t('layout.folderIcon.title', { name: folderName })"
    :confirm-text="t('layout.folderIcon.save')"
    :cancel-text="t('common.cancel')"
    :busy-text="t('layout.folderIcon.saving')"
    :busy="busy"
    :confirm-disabled="busy || draftIcon === currentIcon"
    :close-label="t('layout.folderIcon.closeLabel')"
    @cancel="emit('cancel')"
    @confirm="emit('submit', { icon: draftIcon })"
  >
    <FolderIconPicker v-model="draftIcon" :disabled="busy" />
    <p v-if="error" class="folder-icon-error" role="alert">{{ error }}</p>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import ConfirmModal from './ConfirmModal.vue';
import FolderIconPicker from './FolderIconPicker.vue';
import { useI18n } from '@/i18n';

const props = withDefaults(
  defineProps<{
    open: boolean;
    folderName: string;
    /** The folder's icon now, '' for the default. */
    currentIcon: string;
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
  submit: [payload: { icon: string }];
}>();

const { t } = useI18n();
const draftIcon = ref('');

watch(
  () => [props.open, props.currentIcon] as const,
  ([open, icon]) => {
    if (open) {
      draftIcon.value = icon;
    }
  },
  { immediate: true }
);
</script>

<style scoped>
.folder-icon-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #991b1b;
  margin: 12px 0 0;
  padding: 10px;
  white-space: pre-line;
}
</style>
