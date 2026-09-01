<template>
  <ScanIntervalField
    :model-value="modelValue"
    :disabled="disabled"
    label-key="settings.shelves.bookCheckIntervalLabel"
    help-default-key="settings.shelves.bookCheckIntervalHelpDefault"
    help-every-key="settings.shelves.bookCheckIntervalHelpEvery"
    help-always-key="settings.shelves.bookCheckIntervalHelpAlways"
    testid-prefix="book-check-interval"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>

<script setup lang="ts">
// book_check_interval is the same duration control as the scan interval; only
// the copy differs, so this delegates to ScanIntervalField with the book-check
// label and help keys rather than duplicating the mode/amount/unit logic.
import ScanIntervalField from '@/features/settings/components/ScanIntervalField.vue';

withDefaults(defineProps<{ modelValue: string; disabled?: boolean }>(), {
  disabled: false
});
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();
</script>
