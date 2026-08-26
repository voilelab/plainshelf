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
    <!-- On the desktop the shelf is a real folder on this machine, so a
         first-time user who has not imported anything can see exactly where to
         drop files. The path is the button: clicking it reveals the folder in
         the host file explorer. Absent off the desktop, where no such local
         path exists. -->
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
import { ref, watch } from 'vue';
import { useI18n } from '@/i18n';
import { useWriteAccess } from '@/composables/useWriteAccess';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { getBookshelfProvider } from '@/providers';

const { t } = useI18n();
const { writesEnabled } = useWriteAccess();
const { selectedShelfID } = useShelvesStore();

// The active shelf's real lib_root, resolved through the desktop provider.
// Empty off the desktop (the provider has no getDesktopShelfDetails there) and
// while nothing is selected, which hides the path row entirely.
const shelfPath = ref('');

async function loadShelfPath(shelfID: string): Promise<void> {
  const provider = getBookshelfProvider();
  if (!provider.getDesktopShelfDetails || !shelfID) {
    shelfPath.value = '';
    return;
  }
  try {
    const details = await provider.getDesktopShelfDetails(shelfID);
    // The lookup is async; ignore a response for a shelf we have since left.
    if (selectedShelfID.value === shelfID) {
      shelfPath.value = details.path;
    }
  } catch {
    shelfPath.value = '';
  }
}

watch(selectedShelfID, (id) => void loadShelfPath(id), { immediate: true });

async function onOpenShelfFolder(): Promise<void> {
  const provider = getBookshelfProvider();
  if (!provider.openDesktopShelfFolder) {
    return;
  }
  try {
    await provider.openDesktopShelfFolder(selectedShelfID.value);
  } catch {
    // Best-effort: revealing the folder is a convenience, and the shelf
    // settings panel is where a genuine shelf error is surfaced.
  }
}

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
