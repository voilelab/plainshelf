<template>
  <MaintenanceBooksPage filter="missing-cover" />
</template>

<script setup lang="ts">
import MaintenanceBooksPage from './MaintenanceBooksPage.vue';
import { useDocumentTitle } from '../composables/useDocumentTitle';

useDocumentTitle(() => ['Missing Cover', 'PlainShelf']);
</script>
