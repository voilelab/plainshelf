<template>
  <section class="not-found-page">
    <div class="panel not-found-card" role="status">
      <h2>{{ t('notFound.title') }}</h2>
      <p>{{ t('notFound.description') }}</p>
      <p class="not-found-path">{{ attemptedPath }}</p>
      <RouterLink to="/books" class="button primary not-found-action">
        {{ t('notFound.backToLibrary') }}
      </RouterLink>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from '@/i18n';
import { useDocumentTitle } from '@/composables/useDocumentTitle';

const { t } = useI18n();
const route = useRoute();

const attemptedPath = computed(() => route.fullPath);

useDocumentTitle(() => [t('notFound.title')]);
</script>

<style scoped>
.not-found-page {
  display: flex;
  justify-content: center;
  padding: 24px 0;
}

.not-found-card {
  display: grid;
  gap: 10px;
  max-width: 560px;
  padding: 24px;
  width: 100%;
}

.not-found-card h2,
.not-found-card p {
  margin: 0;
}

.not-found-card p {
  color: var(--muted);
}

.not-found-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.not-found-action {
  justify-self: start;
}
</style>
