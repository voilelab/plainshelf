<template>
  <section>
    <div v-if="loading" class="loading">Loading book metadata...</div>
    <div v-else-if="error" class="error edit-error" role="alert">
      <p>{{ error }}</p>
      <button class="button" type="button" @click="fetchBook">Retry</button>
    </div>

    <EditBook v-else-if="book" :book="book" :saving="saving" :error="saveError" @submit="onSubmit" @cancel="goBack" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { getBook, updateBook } from '../api/books';
import EditBook from '../components/EditBook.vue';
import type { Book, BookUpdateRequest } from '../types/book';

const route = useRoute();
const router = useRouter();
const id = computed(() => String(route.params.id));

const loading = ref(false);
const saving = ref(false);
const error = ref('');
const saveError = ref('');
const book = ref<Book | null>(null);

async function fetchBook(): Promise<void> {
  loading.value = true;
  error.value = '';
  try {
    book.value = await getBook(id.value);
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load metadata';
  } finally {
    loading.value = false;
  }
}

async function onSubmit(payload: BookUpdateRequest): Promise<void> {
  saving.value = true;
  saveError.value = '';

  try {
    await updateBook(id.value, payload);

    await router.push({
      path: `/books/${id.value}`,
      query: {
        saved: '1'
      }
    });
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : 'Failed to save metadata';
  } finally {
    saving.value = false;
  }
}

function goBack(): void {
  void router.push(`/books/${id.value}`);
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
