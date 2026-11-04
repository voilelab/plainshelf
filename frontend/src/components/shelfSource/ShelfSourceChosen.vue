<template>
  <div class="shelf-source-chosen">
    <span class="shelf-source-chosen-icon"><Icon :name="icon" /></span>
    <strong>{{ label }}</strong>
    <button
      v-if="changeLabel"
      type="button"
      class="shelf-source-chosen-change"
      :disabled="disabled"
      @click="emit('change')"
    >
      {{ changeLabel }}
    </button>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/Icon.vue';
import type { IconName } from '@/components/icons/registry';

defineProps<{ icon: IconName; label: string; changeLabel?: string; disabled?: boolean }>();

const emit = defineEmits<{ (event: 'change'): void }>();
</script>

<style scoped>
.shelf-source-chosen {
  align-items: center;
  background: #eaf2ff;
  border-radius: 12px;
  display: flex;
  gap: 12px;
  padding: 10px 14px;
}

.shelf-source-chosen-icon {
  align-items: center;
  background: var(--surface);
  border-radius: 9px;
  color: var(--accent);
  display: flex;
  height: 36px;
  justify-content: center;
  width: 36px;
}

.shelf-source-chosen-icon :deep(svg) {
  height: 20px;
  width: 20px;
}

.shelf-source-chosen strong {
  color: var(--text);
  font-size: 15px;
}

.shelf-source-chosen-change {
  background: none;
  border: 0;
  color: var(--accent);
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  margin-left: auto;
  padding: 4px;
}
</style>
