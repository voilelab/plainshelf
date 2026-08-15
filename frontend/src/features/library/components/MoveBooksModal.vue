<template>
  <BaseDialog :open="open" :title="t('bookCollection.selection.moveTitle')" :busy="busy" @close="emit('cancel')">
    <section class="panel move-books-modal">
      <h2>{{ t('bookCollection.selection.moveTitle') }}</h2>
      <label>
        <span>{{ t('bookCollection.selection.moveTarget') }}</span>
        <select v-model="target" class="input">
          <option value="" disabled>{{ t('bookCollection.selection.chooseLayer') }}</option>
          <option value="/">{{ t('bookCollection.selection.rootLayer') }}</option>
          <option v-for="option in options" :key="option" :value="option">{{ option }}</option>
        </select>
      </label>
      <footer>
        <button type="button" class="button" :disabled="busy" @click="emit('cancel')">{{ t('common.cancel') }}</button>
        <button type="button" class="button primary" :disabled="busy || !target" @click="submit">
          {{ busy ? t('bookCollection.selection.moving') : t('bookCollection.selection.confirmMove', { count }) }}
        </button>
      </footer>
    </section>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import BaseDialog from '@/components/BaseDialog.vue';
import { useI18n } from '@/i18n';

const props = defineProps<{ open: boolean; count: number; options: string[]; busy?: boolean }>();
const emit = defineEmits<{ cancel: []; submit: [targetLayer: string] }>();
const { t } = useI18n();
const target = ref('');
watch(() => props.open, (open) => { if (open) target.value = ''; });
function submit(): void { if (target.value) emit('submit', target.value === '/' ? '' : target.value); }
</script>

<style scoped>
.move-books-modal { display: grid; gap: 16px; max-width: 440px; padding: 18px; width: min(100%, 440px); }
.move-books-modal h2 { margin: 0; }
.move-books-modal label { display: grid; gap: 6px; }
.move-books-modal footer { display: flex; gap: 8px; justify-content: flex-end; }
</style>
