<!-- The one checkbox deliberately left as a native input.
     Every book card renders one and the library list is not windowed, so this
     mounts once per book. Measured on the Reka primitive (CheckboxRoot plus
     CheckboxIndicator, three component instances against one element): 2000
     items took ~2.5s to mount versus ~75ms for the input below, a 29-34x cost
     repeated across three runs. The shared BaseCheckbox is for form controls
     that appear a handful of times on a page; this is a selection badge over a
     cover, and it stays cheap. -->
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
