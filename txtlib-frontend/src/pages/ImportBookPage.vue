<template>
  <ImportBookModal :open="true" @close="onClose" @imported="onImported" />
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import ImportBookModal from '../components/ImportBookModal.vue';

const router = useRouter();

function onClose(): void {
  void router.replace('/books');
}

function onImported(result: {
  total: number;
  successCount: number;
  firstImportedId?: string;
}): void {
  if (result.total === 1 && result.successCount === 1 && result.firstImportedId) {
    void router.push({
      path: `/books/${result.firstImportedId}`,
      query: {
        imported: '1'
      }
    });
  }
}
</script>

<style scoped>
.import-page {
  min-height: 0;
}
</style>
