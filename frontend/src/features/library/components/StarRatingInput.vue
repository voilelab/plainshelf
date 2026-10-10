<template>
  <div class="star-rating">
    <RatingRoot
      :model-value="modelValue"
      as="div"
      class="star-rating-root"
      :length="5"
      clearable
      :aria-label="t('libraryForms.editBook.starRating')"
      @update:model-value="emit('update:modelValue', $event)"
    >
      <RatingItem
        v-for="value in STAR_VALUES"
        :key="value"
        :item="value"
        as="span"
        class="star-item"
        v-slot="{ steps }"
      >
        <RatingItemIndicator
          v-for="step in steps"
          :key="step"
          :step="step"
          class="star-indicator"
          :aria-label="starLabel(value)"
        >
          ★
        </RatingItemIndicator>
      </RatingItem>
    </RatingRoot>
    <button class="clear-rating" type="button" :disabled="modelValue === 0" @click="emit('update:modelValue', 0)">{{ t('libraryForms.editBook.clearRating') }}</button>
  </div>
</template>

<script setup lang="ts">
import { RatingItem, RatingItemIndicator, RatingRoot } from 'reka-ui';
import { useI18n } from '@/i18n';

defineProps<{
  modelValue: number;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: number];
}>();

const { t } = useI18n();

const STAR_VALUES = [1, 2, 3, 4, 5] as const;

// English says "1 star" but "2 stars"; the catalog has no plural rules, so the
// two forms are separate keys.
function starLabel(value: number): string {
  return value === 1
    ? t('libraryForms.editBook.starValueOne')
    : t('libraryForms.editBook.starValueMany', { count: value });
}
</script>

<style scoped>
.star-rating {
  display: flex;
  align-items: center;
  gap: 4px;
}

.star-rating-root {
  display: flex;
  align-items: center;
  gap: 4px;
}

.star-item {
  display: inline-flex;
}

/* :deep() required: reka-ui's Radio renders a fragment, which breaks scoped
   scope-id inheritance, so the actual star <button> never gets our data-v attr. */
.star-rating :deep(.star-indicator) {
  padding: 0 2px;
  border: 0;
  background: transparent;
  color: #c4cad4;
  cursor: pointer;
  font-size: 28px;
  line-height: 1;
}

.star-rating :deep(.star-indicator[data-state='active']) {
  color: #f5a623;
}

.star-rating :deep(.star-indicator:focus-visible),
.clear-rating:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.clear-rating {
  margin-left: 8px;
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.clear-rating:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
</style>
