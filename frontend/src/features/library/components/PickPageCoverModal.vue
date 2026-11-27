<template>
  <ConfirmModal
    :open="open"
    :title="t('bookDetail.cover.fromPage.title')"
    :confirm-text="t('bookDetail.cover.confirm')"
    :busy="busy"
    :busy-text="t('bookDetail.cover.uploading')"
    :confirm-disabled="selectedName === ''"
    @cancel="onCancel"
    @confirm="onConfirm"
  >
    <p v-if="!pages.length">{{ t('reader.comic.empty') }}</p>
    <div v-else class="page-cover-grid" role="group" :aria-label="t('bookDetail.cover.fromPage.title')">
      <PageCoverThumb
        v-for="(page, index) in pages"
        :key="`${index}:${page.name}`"
        :book-id="bookId"
        :source-id="sourceId"
        :page="page"
        :index="index"
        :selected="page.name === selectedName"
        :disabled="busy"
        @select="selectedName = page.name"
      />
    </div>
    <p v-if="errorMessage" class="page-cover-error" role="alert">{{ errorMessage }}</p>
  </ConfirmModal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { setCoverFromPage } from '@/composables/setCoverFromPage';
import { useI18n } from '@/i18n';
import type { SourcePage } from '@/types/source';
import PageCoverThumb from './PageCoverThumb.vue';

const props = defineProps<{
  open: boolean;
  bookId: string;
  sourceId: string;
  pages: SourcePage[];
}>();
const emit = defineEmits<{ close: []; saved: [] }>();
const { t } = useI18n();
const selectedName = ref('');
const busy = ref(false);
const errorMessage = ref('');

watch(() => props.open, (open) => {
  if (open) {
    selectedName.value = '';
    errorMessage.value = '';
  }
});

function onCancel(): void {
  if (!busy.value) emit('close');
}

async function onConfirm(): Promise<void> {
  if (!selectedName.value || busy.value) return;
  busy.value = true;
  errorMessage.value = '';
  try {
    await setCoverFromPage(props.bookId, props.sourceId, selectedName.value);
    emit('saved');
  } catch (err) {
    errorMessage.value = err instanceof Error
      ? t('bookDetail.cover.uploadFailedWithReason', { reason: err.message })
      : t('bookDetail.cover.uploadFailed');
  } finally {
    busy.value = false;
  }
}
</script>

<style scoped>
.page-cover-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  gap: 8px;
  max-height: min(520px, calc(60vh / var(--app-zoom, 1)));
  overflow-y: auto;
}

.page-cover-error {
  color: var(--danger);
}
</style>
