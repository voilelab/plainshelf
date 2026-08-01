<template>
  <label class="book-selection-checkbox" @click.stop>
    <input
      type="checkbox"
      :checked="selected"
      :aria-label="label"
      @change="emit('toggle')"
    />
    <span aria-hidden="true">✓</span>
  </label>
</template>

<script setup lang="ts">
defineProps<{ selected: boolean; label: string }>();
const emit = defineEmits<{ toggle: [] }>();
</script>
