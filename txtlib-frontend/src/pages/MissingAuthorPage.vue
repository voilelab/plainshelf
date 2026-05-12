<template>
  <MaintenanceBooksPage filter="missing-author" />
</template>

<script setup lang="ts">
import MaintenanceBooksPage from './MaintenanceBooksPage.vue';
import { useDocumentTitle } from '../composables/useDocumentTitle';

useDocumentTitle(() => ['Missing Author', 'PlainShelf']);
</script>
