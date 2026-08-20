<template>
  <BaseDialog :open="open" :title="heading" :busy="busy" @close="emit('cancel')">
    <section class="panel move-books-modal">
      <h2>{{ heading }}</h2>
      <label>
        <span>{{ t('bookCollection.selection.moveTarget') }}</span>
        <select v-model="target" class="input">
          <option value="" disabled>{{ t('bookCollection.selection.chooseLayer') }}</option>
          <option value="/">{{ t('bookCollection.selection.rootLayer') }}</option>
          <option v-for="option in options" :key="option" :value="option">{{ option }}</option>
        </select>
      </label>
      <p v-if="error" class="move-books-error" role="alert">{{ error }}</p>
      <footer>
        <button type="button" class="button" :disabled="busy" @click="emit('cancel')">{{ t('common.cancel') }}</button>
        <button type="button" class="button primary" :disabled="busy || !target" @click="submit">
          {{ busy ? t('bookCollection.selection.moving') : confirmLabel }}
        </button>
      </footer>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import BaseDialog from '@/components/BaseDialog.vue';
import { useI18n } from '@/i18n';

// `title` lets a single-book caller name what it is moving; batch callers omit
// it and keep the selection wording. `error` belongs in here rather than on the
// page behind it: the dialog stays open after a failed move, and the overlay
// hides — and takes out of the accessibility tree — anything outside it.
const props = defineProps<{
  open: boolean;
  count: number;
  options: string[];
  busy?: boolean;
  title?: string;
  error?: string;
}>();
const emit = defineEmits<{ cancel: []; submit: [targetLayer: string] }>();
const { t } = useI18n();
const target = ref('');
const heading = computed(() => props.title ?? t('bookCollection.selection.moveTitle'));
// Neither locale has plural forms, so one book gets its own phrasing instead of
// reading "Move 1 books".
const confirmLabel = computed(() => (
  props.count === 1
    ? t('bookCollection.selection.confirmMoveOne')
    : t('bookCollection.selection.confirmMove', { count: props.count })
));
watch(() => props.open, (open) => { if (open) target.value = ''; });
function submit(): void { if (target.value) emit('submit', target.value === '/' ? '' : target.value); }
</script>

<style scoped>
.move-books-modal { display: grid; gap: 16px; max-width: 440px; padding: 18px; width: min(100%, 440px); }
.move-books-modal h2 { margin: 0; }
.move-books-modal label { display: grid; gap: 6px; }
.move-books-modal footer { display: flex; gap: 8px; justify-content: flex-end; }
.move-books-error { color: #991b1b; font-size: 13px; margin: 0; }
</style>
