<template>
  <ScanIntervalField
    :model-value="modelValue"
    :disabled="disabled"
    label-key="settings.shelves.bookCheckIntervalLabel"
    help-default-key="settings.shelves.bookCheckIntervalHelpDefault"
    help-every-key="settings.shelves.bookCheckIntervalHelpEvery"
    help-always-key="settings.shelves.bookCheckIntervalHelpAlways"
    amount-label-key="settings.shelves.bookCheckIntervalAmountLabel"
    testid-prefix="book-check-interval"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>

<script setup lang="ts">
import ScanIntervalField from '@/features/settings/components/ScanIntervalField.vue';

withDefaults(defineProps<{ modelValue: string; disabled?: boolean }>(), {
  disabled: false
});
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();
</script>
