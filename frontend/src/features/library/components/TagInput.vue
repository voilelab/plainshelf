<template>
  <TagsInputRoot
    :model-value="modelValue"
    class="tag-input-shell"
    add-on-blur
    add-on-paste
    :convert-value="normalizeTag"
    @update:model-value="emit('update:modelValue', $event as string[])"
    @click="focusTagInput"
  >
    <TagsInputItem v-for="tag in modelValue" :key="tag" :value="tag" class="tag-chip">
      <TagsInputItemText />
      <TagsInputItemDelete class="tag-remove" :aria-label="t('libraryForms.editBook.removeTag', { tag })">×</TagsInputItemDelete>
    </TagsInputItem>
    <TagsInputInput ref="tagsInputRef" class="tag-input" :placeholder="t('libraryForms.editBook.tagsPlaceholder')" />
  </TagsInputRoot>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
  TagsInputInput,
  TagsInputItem,
  TagsInputItemDelete,
  TagsInputItemText,
  TagsInputRoot
} from 'reka-ui';
import { useI18n } from '@/i18n';

defineProps<{
  modelValue: string[];
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string[]];
}>();

const { t } = useI18n();

const tagsInputRef = ref<InstanceType<typeof TagsInputInput> | null>(null);

function normalizeTag(rawValue: string): string {
  return rawValue.trim().replace(/\s+/g, ' ');
}

function focusTagInput(event: MouseEvent): void {
  if (event.target !== event.currentTarget) {
    return;
  }
  (tagsInputRef.value as unknown as { $el?: HTMLInputElement } | null)?.$el?.focus();
}
</script>

<style scoped>
.tag-input-shell {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #fff;
}

.tag-input-shell:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(82, 102, 255, 0.12);
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 999px;
  background: #eef2ff;
  color: #2b3a9a;
  font-size: 13px;
}

.tag-chip[data-state='active'] {
  background: #dbeafe;
  outline: 1px solid #93c5fd;
}

.tag-remove {
  border: none;
  background: transparent;
  color: inherit;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  padding: 0;
}

.tag-input {
  flex: 1 1 180px;
  min-width: 140px;
  border: none;
  outline: none;
  background: transparent;
  font: inherit;
  color: inherit;
  padding: 4px 0;
}
</style>
