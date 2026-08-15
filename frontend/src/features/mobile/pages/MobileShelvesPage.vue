<template>
  <div class="mobile-connect">
    <!-- A device with no shelves has nothing to list, and the router sends every
         route here until it has one — so show the form itself rather than an
         empty state the user would have to tap through. -->
    <MobileShelfForm v-if="ready && entries.length === 0" />

    <section v-else-if="ready" class="mobile-shelves panel" :aria-label="t('mobileShelves.title')">
      <h1 class="mobile-shelves-title">{{ t('mobileShelves.title') }}</h1>
      <p class="mobile-shelves-description">{{ t('mobileShelves.description') }}</p>

      <ul class="mobile-shelves-list">
        <li v-for="entry in entries" :key="entry.id" class="mobile-shelves-item">
          <div class="mobile-shelves-item-head">
            <span class="mobile-shelves-name">{{ displayName(entry) }}</span>
            <span class="mobile-shelves-type">{{ typeLabel(entry) }}</span>
            <span v-if="entry.id === activeEntryID" class="mobile-shelves-active">
              {{ t('mobileShelves.active') }}
            </span>
          </div>
          <p class="mobile-shelves-detail">{{ detail(entry) }}</p>

          <div class="mobile-shelves-actions">
            <button
              v-if="entry.id !== activeEntryID"
              type="button"
              class="button"
              :disabled="busy"
              @click="onUse(entry.id)"
            >
              {{ t('mobileShelves.use') }}
            </button>
            <button type="button" class="button" @click="onEdit(entry.id)">
              {{ t('mobileShelves.edit') }}
            </button>
            <button type="button" class="button" :disabled="busy" @click="pendingRemoval = entry">
              {{ t('mobileShelves.remove') }}
            </button>
          </div>
        </li>
      </ul>

      <p v-if="error" class="mobile-shelves-error" role="alert">{{ error }}</p>

      <button type="button" class="button primary" @click="onAdd">
        {{ t('mobileShelves.add') }}
      </button>

      <div v-if="pendingRemoval" class="mobile-shelves-confirm" role="alertdialog">
        <h2 class="mobile-shelves-confirm-title">
          {{ t('mobileShelves.removeTitle', { name: displayName(pendingRemoval) }) }}
        </h2>
        <p class="mobile-shelves-confirm-body">{{ t('mobileShelves.removeDescription') }}</p>
        <div class="mobile-shelves-actions">
          <button type="button" class="button" :disabled="busy" @click="pendingRemoval = null">
            {{ t('mobileShelves.removeCancel') }}
          </button>
          <button type="button" class="button primary" :disabled="busy" @click="onRemove">
            {{ t('mobileShelves.removeConfirm') }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from '@/i18n';
import MobileShelfForm from '@/features/mobile/components/MobileShelfForm.vue';
import { removeCacheScope } from '@/providers/mobileCacheFs';
import { cacheScopeKeyForEntry } from '@/providers/cacheScope';
import {
  deleteShelfEntry,
  loadShelfEntries,
  setActiveShelfEntry,
  shelfEntryDisplayName,
  type ShelfEntry
} from '@/providers/mobileConfig';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();

const entries = ref<ShelfEntry[]>([]);
const activeEntryID = ref('');
const ready = ref(false);
const busy = ref(false);
const error = ref('');
const pendingRemoval = ref<ShelfEntry | null>(null);

onMounted(async () => {
  await refresh();
  ready.value = true;
});

async function refresh(): Promise<void> {
  const loaded = await loadShelfEntries();
  entries.value = loaded.entries;
  activeEntryID.value = loaded.activeEntryID;
}

const displayName = shelfEntryDisplayName;

function typeLabel(entry: ShelfEntry): string {
  return entry.type === 'pcloud' ? t('mobileConnect.modePCloud') : t('mobileConnect.modeServer');
}

function detail(entry: ShelfEntry): string {
  return entry.type === 'pcloud'
    ? `${entry.pcloudHost} · ${entry.pcloudShelfRoot}`
    : `${entry.serverUrl} · ${entry.shelfId}`;
}

// The query is carried across every navigation between the setup pages: saving
// restarts the app from the current URL (reloadIntoApp), so a push that dropped
// `?mobile-shell-preview=1` would leave the browser preview — and the e2e suite
// built on it — restarting outside mobile mode. Native Capacitor is unaffected,
// which is exactly what makes the loss silent.
function onAdd(): void {
  void router.push({ name: 'mobile-shelf-add', query: route.query });
}

function onEdit(entryID: string): void {
  void router.push({
    name: 'mobile-shelf-edit',
    params: { entryId: entryID },
    query: route.query
  });
}

async function onUse(entryID: string): Promise<void> {
  busy.value = true;
  error.value = '';
  try {
    await setActiveShelfEntry(entryID);
    reloadIntoApp();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
    busy.value = false;
  }
}

/**
 * Drops the entry first and sweeps its downloads afterwards.
 *
 * That order is deliberate: a failed sweep leaves reclaimable files behind,
 * which is harmless, while a failed entry write after a successful sweep would
 * leave a shelf in the list whose downloads had already been deleted.
 */
async function onRemove(): Promise<void> {
  const target = pendingRemoval.value;
  if (!target) {
    return;
  }

  busy.value = true;
  error.value = '';
  const wasActive = target.id === activeEntryID.value;
  try {
    await deleteShelfEntry(target.id);
    await removeCacheScope(cacheScopeKeyForEntry(target));
    pendingRemoval.value = null;
    await refresh();

    // The app was reading this shelf, so everything held in memory now belongs
    // to a shelf that no longer exists.
    if (wasActive) {
      reloadIntoApp(entries.value.length > 0 ? '/books' : '/connect');
      return;
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    busy.value = false;
  }
}
</script>

<style scoped>
.mobile-connect {
  min-height: calc(100vh / var(--app-zoom, 1));
  width: 100%;
  box-sizing: border-box;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 32px 16px;
  overflow-y: auto;
  background:
    radial-gradient(circle at top, rgba(255, 250, 240, 0.92), rgba(245, 241, 232, 0.96) 42%),
    linear-gradient(180deg, #f7f2e7 0%, #efe7d7 100%);
}

.mobile-shelves {
  width: min(480px, 100%);
  padding: 24px;
  display: grid;
  gap: 12px;
}

.mobile-shelves-title {
  margin: 0;
  font-size: 1.4rem;
}

.mobile-shelves-description {
  margin: 0;
  color: var(--text-muted, #666);
}

.mobile-shelves-list {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  display: grid;
  gap: 12px;
}

.mobile-shelves-item {
  display: grid;
  gap: 6px;
  padding: 12px;
  border: 1px solid var(--border, #d9d2c4);
  border-radius: 8px;
}

.mobile-shelves-item-head {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px;
}

.mobile-shelves-name {
  font-weight: 600;
}

.mobile-shelves-type,
.mobile-shelves-active {
  font-size: 0.75rem;
  color: var(--text-muted, #666);
}

.mobile-shelves-active {
  font-weight: 600;
  color: var(--accent, #7a5c2e);
}

.mobile-shelves-detail {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted, #666);
  overflow-wrap: anywhere;
}

.mobile-shelves-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mobile-shelves-error {
  margin: 0;
  color: var(--danger, #b3261e);
  font-size: 0.9rem;
}

.mobile-shelves-confirm {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--danger, #b3261e);
  border-radius: 8px;
}

.mobile-shelves-confirm-title {
  margin: 0;
  font-size: 1rem;
}

.mobile-shelves-confirm-body {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted, #666);
}
</style>
