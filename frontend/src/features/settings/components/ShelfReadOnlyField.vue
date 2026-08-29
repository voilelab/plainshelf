<template>
  <label class="setting-item shelf-read-only" :for="checkboxId">
    <div>
      <span :id="labelId" class="setting-label">{{ t('settings.shelves.readOnlyLabel') }}</span>
      <p :id="descriptionId" class="setting-description">{{ t('settings.shelves.readOnlyHelp') }}</p>
      <!-- None of these three follow from "read-only" on its own, and each one
           is visible to the user the moment it bites: the shelf stops being
           locked against a second instance, its exported book cache stops being
           refreshed, and a path that does not exist yet is an error rather than
           a new shelf. -->
      <ul class="shelf-read-only-effects">
        <li>{{ t('settings.shelves.readOnlyEffectLock') }}</li>
        <li>{{ t('settings.shelves.readOnlyEffectBookCache') }}</li>
        <li>{{ t('settings.shelves.readOnlyEffectPath') }}</li>
      </ul>
    </div>
    <BaseCheckbox
      :id="checkboxId"
      v-model="model"
      :disabled="disabled"
      :aria-labelledby="labelId"
      :aria-describedby="descriptionId"
      data-testid="shelf-read-only"
    />
  </label>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue';
import BaseCheckbox from '@/components/BaseCheckbox.vue';
import { useI18n } from '@/i18n';

const props = withDefaults(defineProps<{ modelValue: boolean; disabled?: boolean }>(), {
  disabled: false
});
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>();

const { t } = useI18n();

const checkboxId = `shelf-read-only-${useId()}`;
const labelId = `shelf-read-only-label-${useId()}`;
const descriptionId = `shelf-read-only-description-${useId()}`;

const model = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
});
</script>

<style scoped src="../styles/settings-form.css"></style>

<style scoped>
.shelf-read-only {
  align-items: flex-start;
}

.shelf-read-only-effects {
  color: #475569;
  font-size: 12px;
  margin: 6px 0 0;
  padding-left: 18px;
}

.shelf-read-only-effects li + li {
  margin-top: 2px;
}
</style>
