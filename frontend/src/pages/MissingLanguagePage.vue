<template>
  <MaintenanceBooksPage filter="missing-language" />
</template>

<script setup lang="ts">
import MaintenanceBooksPage from './MaintenanceBooksPage.vue';
import { useDocumentTitle } from '../composables/useDocumentTitle';
import { useI18n } from '../i18n';

const { t } = useI18n();

useDocumentTitle(() => [t('maintenance.missingLanguage.title'), t('app.name')]);
</script>
