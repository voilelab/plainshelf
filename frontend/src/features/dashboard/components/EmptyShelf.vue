<template>
  <div class="empty-shelf panel">
    <h3 class="empty-shelf-title">{{ t('dashboard.empty.title') }}</h3>
    <p class="empty-shelf-description">
      {{ writesEnabled ? t('dashboard.empty.description') : t('dashboard.empty.readOnlyDescription') }}
    </p>
    <div class="empty-shelf-actions">
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
    <p v-if="shelfPath" class="empty-shelf-path">
      <span class="empty-shelf-path-label">{{ t('dashboard.empty.pathLabel') }}</span>
      <button
        type="button"
        class="empty-shelf-path-value"
        :aria-label="t('dashboard.empty.openFolderLabel')"
        @click="onOpenShelfFolder"
      >
        {{ shelfPath }}
      </button>
    </p>
  </div>
</template>

<script setup lang="ts">
import { getPlatformCapabilities } from '@/providers/platformCapabilities';
import { ref, watch } from 'vue';
import { useI18n } from '@/i18n';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useShelvesStore } from '@/composables/useShelvesStore';

const { t } = useI18n();
const { writesEnabled } = useWriteAccess();
const { selectedShelfID } = useShelvesStore();

const shelfPath = ref('');

async function loadShelfPath(shelfID: string): Promise<void> {
  const platform = getPlatformCapabilities();
  if (!platform.shelves?.getShelfDetails || !shelfID) {
    shelfPath.value = '';
    return;
  }
  try {
    const details = await platform.shelves!.getShelfDetails(shelfID);
    if (selectedShelfID.value === shelfID) {
      shelfPath.value = details.path;
    }
  } catch {
    shelfPath.value = '';
  }
}

watch(selectedShelfID, (id) => void loadShelfPath(id), { immediate: true });

async function onOpenShelfFolder(): Promise<void> {
  const platform = getPlatformCapabilities();
  if (!platform.openShelfFolder) {
    return;
  }
  try {
    await platform.openShelfFolder(selectedShelfID.value);
  } catch {
  }
}

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

.empty-shelf-path {
  align-items: center;
  color: var(--muted);
  display: flex;
  flex-wrap: wrap;
  font-size: 13px;
  gap: 6px;
  justify-content: center;
  margin: 4px 0 0;
  max-width: 100%;
}

.empty-shelf-path-value {
  background: transparent;
  border: none;
  color: var(--accent);
  cursor: pointer;
  font: inherit;
  font-family: monospace;
  overflow-wrap: anywhere;
  padding: 0;
  text-align: left;
}

.empty-shelf-path-value:hover {
  text-decoration: underline;
}
</style>
