<template>
  <section>
    <div v-if="loading" class="loading">{{ t('libraryForms.editBook.loading') }}</div>
    <div v-else-if="error" class="error edit-error" role="alert">
      <p>{{ error }}</p>
      <button class="button" type="button" @click="fetchBook">{{ t('common.retry') }}</button>
    </div>

    <EditBook v-else-if="book" :book="book" :saving="saving" :error="saveError" @submit="onSubmit" @cancel="goBack" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import EditBook from '@/features/library/components/EditBook.vue';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useSafeBackNavigation } from '@/composables/useSafeBackNavigation';
import type { Book, BookUpdateRequest } from '@/types/book';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const { writesEnabled } = useWriteAccess();
const route = useRoute();
const router = useRouter();
const id = computed(() => String(route.params.id));
const { goBack } = useSafeBackNavigation(() => `/books/${id.value}`);

const loading = ref(false);
const saving = ref(false);
const error = ref('');
const saveError = ref('');
const book = ref<Book | null>(null);

async function fetchBook(): Promise<void> {
  loading.value = true;
  error.value = '';
  try {
    book.value = await getBookshelfProvider().getBook(id.value);
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('libraryForms.editBook.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function onSubmit(payload: BookUpdateRequest): Promise<void> {
  if (!writesEnabled.value) {
    return;
  }

  saving.value = true;
  saveError.value = '';

  try {
    await bookshelfWriter().updateBook(id.value, payload);

    await router.push({
      path: `/books/${id.value}`,
      query: {
        saved: '1'
      }
    });
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : t('libraryForms.editBook.saveFailed');
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void fetchBook();
});
</script>

<style scoped>
.edit-error {
  display: grid;
  gap: 10px;
}

.edit-error p {
  margin: 0;
}

.edit-error .button {
  justify-self: start;
}
</style>
