<template>
  <div class="empty-shelf panel">
    <h3 class="empty-shelf-title">{{ t('dashboard.empty.title') }}</h3>
    <p class="empty-shelf-description">
      {{ writesEnabled ? t('dashboard.empty.description') : t('dashboard.empty.readOnlyDescription') }}
    </p>
    <div class="empty-shelf-actions">
      <!-- The import flow only exists where the client can write the shelf. A
           read-only mobile/pCloud client strips the import query and a
           read-only server suppresses the modal, so on those surfaces the
           button would navigate to an inert library; hide it and describe the
           shelf as read-only instead. -->
      <RouterLink
        v-if="writesEnabled"
        class="button primary empty-shelf-import"
        :to="{ path: '/books', query: { import: '1' } }"
      >
        {{ t('dashboard.empty.import') }}
      </RouterLink>
      <a class="empty-shelf-docs" :href="GETTING_STARTED_URL" target="_blank" rel="noreferrer noopener">
        {{ t('dashboard.empty.docs') }}
      </a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from '@/i18n';
import { useWriteAccess } from '@/composables/useWriteAccess';

const { t } = useI18n();
const { writesEnabled } = useWriteAccess();

// PlainShelf has no hosted documentation site; the guides live in the repo.
// `HEAD` resolves to the default branch so the link never pins to a stale one.
const GETTING_STARTED_URL = 'https://github.com/voilelab/plainshelf/blob/HEAD/docs/getting-started.md';
</script>

<style scoped>
.empty-shelf {
  align-items: center;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 48px 24px;
  text-align: center;
}

.empty-shelf-title {
  color: var(--text);
  font-size: 18px;
  font-weight: 700;
  margin: 0;
}

.empty-shelf-description {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
  margin: 0;
  max-width: 42ch;
}

.empty-shelf-actions {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  justify-content: center;
  margin-top: 4px;
}

.empty-shelf-import {
  text-decoration: none;
}

.empty-shelf-docs {
  color: var(--accent);
  font-size: 14px;
}
</style>
