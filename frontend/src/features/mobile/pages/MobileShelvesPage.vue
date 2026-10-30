<template>
  <div class="mobile-connect">
    <div v-if="ready && entries.length === 0" class="mobile-shelves-first-run">
      <p v-if="error" class="mobile-shelves-error" role="alert">{{ error }}</p>
      <section v-if="restorePrompt" class="mobile-shelves-restore panel" :aria-label="t('mobileShelves.restoreTitle')">
        <h1 class="mobile-shelves-title">{{ t('mobileShelves.restoreTitle') }}</h1>
        <p class="mobile-shelves-description">{{ t('mobileShelves.restoreDescription') }}</p>
        <div class="mobile-shelves-actions">
          <button type="button" class="button primary mobile-shelves-restore-zip" :disabled="busy" @click="onRestore">
            {{ t('mobileShelves.restoreZip') }}
          </button>
          <button type="button" class="button mobile-shelves-restore-skip" :disabled="busy" @click="onSkipRestore">
            {{ t('mobileShelves.restoreSkip') }}
          </button>
        </div>
      </section>
      <MobileShelfForm />
    </div>

    <section v-else-if="ready" class="mobile-shelves panel" :aria-label="t('mobileShelves.title')">
      <h1 class="mobile-shelves-title">{{ t('mobileShelves.title') }}</h1>
      <p class="mobile-shelves-description">{{ t('mobileShelves.description') }}</p>
      <p v-if="hasLocal" class="mobile-shelves-local-warning" role="note">
        {{ t('mobileShelves.localUninstallWarning') }}
      </p>

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
          <p v-if="entry.type === 'local'" class="mobile-shelves-detail mobile-shelves-last-export">
            {{ lastExportLabel(entry.localShelfId) }}
          </p>

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
            <button
              v-if="entry.type === 'local'"
              type="button"
              class="button mobile-shelves-export"
              :disabled="busy"
              @click="onExport(entry)"
            >
              {{ t('mobileShelves.export') }}
            </button>
            <button type="button" class="button" :disabled="busy" @click="pendingRemoval = entry">
              {{ t('mobileShelves.remove') }}
            </button>
          </div>
        </li>
      </ul>

      <MobileLanSharePanel v-if="canImport && hasLocal" :shelves="localShelves" />

      <p v-if="error" class="mobile-shelves-error" role="alert">{{ error }}</p>
      <p v-if="notice" class="mobile-shelves-notice" role="status">{{ notice }}</p>

      <button type="button" class="button primary" @click="onAdd">
        {{ t('mobileShelves.add') }}
      </button>
      <button v-if="canImport" type="button" class="button mobile-shelves-import" :disabled="busy" @click="onImport">
        {{ t('mobileShelves.importZip') }}
      </button>

      <div v-if="pendingRemoval" class="mobile-shelves-confirm" role="alertdialog">
        <h2 class="mobile-shelves-confirm-title">
          {{ t('mobileShelves.removeTitle', { name: displayName(pendingRemoval) }) }}
        </h2>
        <p class="mobile-shelves-confirm-body">
          {{
            pendingRemoval.type === 'local'
              ? t('mobileShelves.removeLocalDescription')
              : t('mobileShelves.removeDescription')
          }}
        </p>
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
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Capacitor } from '@capacitor/core';
import { useI18n } from '@/i18n';
import MobileShelfForm from '@/features/mobile/components/MobileShelfForm.vue';
import MobileLanSharePanel from '@/features/mobile/components/MobileLanSharePanel.vue';
import { LocalShelf, localShelfErrorKey, localShelfExportFileName } from '@/providers/localShelf';
import { removeCacheScope } from '@/providers/mobileCacheFs';
import { cacheScopeKeyForEntry } from '@/providers/cacheScope';
import {
  addShelfEntry,
  deleteShelfEntry,
  dismissRestorePrompt,
  isRestorePromptDismissed,
  loadShelfEntries,
  newShelfEntry,
  setActiveShelfEntry,
  shelfEntryDisplayName,
  SOURCE_TYPE_LABEL_KEYS,
  upsertShelfEntry,
  type ShelfEntry
} from '@/providers/mobileConfig';
import { reloadIntoApp } from '@/features/mobile/utils/reloadIntoApp';

const { t, locale } = useI18n();
const route = useRoute();
const router = useRouter();

const entries = ref<ShelfEntry[]>([]);
const activeEntryID = ref('');
const ready = ref(false);
const busy = ref(false);
const error = ref('');
const notice = ref('');
const pendingRemoval = ref<ShelfEntry | null>(null);
const lastExports = ref<Record<string, string>>({});
const canImport = Capacitor.isNativePlatform();
const hasLocal = computed(() => entries.value.some((entry) => entry.type === 'local'));
const localShelves = computed(() =>
  entries.value.flatMap((entry) =>
    entry.type === 'local' ? [{ id: entry.localShelfId, name: shelfEntryDisplayName(entry) }] : []
  )
);
const restorePrompt = ref(false);

onMounted(async () => {
  await refresh();
  if (await firstRun()) {
    return;
  }
  ready.value = true;
});

async function firstRun(): Promise<boolean> {
  if (!canImport || entries.value.length > 0) {
    return false;
  }
  let shelves: { id: string; name: string }[];
  try {
    ({ shelves } = await LocalShelf.listShelves());
  } catch {
    return false;
  }
  if (shelves.length === 0) {
    restorePrompt.value = !(await isRestorePromptDismissed());
    return false;
  }
  try {
    const adopted = shelves.map((shelf) => ({
      ...newShelfEntry('local'),
      type: 'local' as const,
      name: shelf.name,
      localShelfId: shelf.id
    }));
    for (const entry of adopted) {
      await addShelfEntry(entry);
    }
    await setActiveShelfEntry(adopted[0].id);
    reloadIntoApp();
    return true;
  } catch (err) {
    error.value = explain(err);
    return false;
  }
}

async function refresh(): Promise<void> {
  const loaded = await loadShelfEntries();
  entries.value = loaded.entries;
  activeEntryID.value = loaded.activeEntryID;
  if (loaded.entries.some((entry) => entry.type === 'local')) {
    try {
      const { shelves } = await LocalShelf.listShelves();
      lastExports.value = Object.fromEntries(shelves.map((shelf) => [shelf.id, shelf.last_export ?? '']));
    } catch {
    }
  }
}

function lastExportLabel(localShelfId: string): string {
  const at = lastExports.value[localShelfId];
  if (!at) {
    return t('mobileShelves.neverExported');
  }
  return t('mobileShelves.lastExport', { time: new Date(at).toLocaleString(locale.value) });
}

function explain(err: unknown): string {
  const key = localShelfErrorKey(err);
  return key ? t(key) : err instanceof Error ? err.message : String(err);
}

async function onExport(entry: ShelfEntry): Promise<void> {
  if (entry.type !== 'local') {
    return;
  }
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    const { cancelled } = await LocalShelf.exportShelf({
      id: entry.localShelfId,
      fileName: localShelfExportFileName(displayName(entry))
    });
    if (!cancelled) {
      notice.value = t('mobileShelves.exported');
      await refresh();
    }
  } catch (err) {
    error.value = explain(err);
  } finally {
    busy.value = false;
  }
}

async function onImport(): Promise<void> {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    const result = await LocalShelf.importShelf({ fallbackName: t('mobileShelves.importedName') });
    if (result.cancelled || !result.id) {
      return;
    }
    const name = result.name ?? t('mobileShelves.importedName');
    await addShelfEntry({ ...newShelfEntry('local'), type: 'local', name, localShelfId: result.id });
    notice.value = t('mobileShelves.imported', { name });
    await refresh();
  } catch (err) {
    error.value = explain(err);
  } finally {
    busy.value = false;
  }
}

async function onRestore(): Promise<void> {
  busy.value = true;
  error.value = '';
  try {
    const result = await LocalShelf.importShelf({ fallbackName: t('mobileShelves.importedName') });
    if (result.cancelled || !result.id) {
      busy.value = false;
      return;
    }
    const name = result.name ?? t('mobileShelves.importedName');
    await upsertShelfEntry({ ...newShelfEntry('local'), type: 'local', name, localShelfId: result.id });
    reloadIntoApp();
  } catch (err) {
    error.value = explain(err);
    busy.value = false;
  }
}

async function onSkipRestore(): Promise<void> {
  restorePrompt.value = false;
  error.value = '';
  await dismissRestorePrompt();
}

const displayName = shelfEntryDisplayName;

function typeLabel(entry: ShelfEntry): string {
  return t(SOURCE_TYPE_LABEL_KEYS[entry.type]);
}

function detail(entry: ShelfEntry): string {
  if (entry.type === 'local') {
    return t('mobileShelves.localDetail');
  }
  if (entry.type === 'dav') {
    return `${t(`shelfSource.${entry.davSource}`)} · ${entry.davUrl}`;
  }
  return entry.type === 'pcloud'
    ? `${entry.pcloudHost} · ${entry.pcloudShelfRoot}`
    : `${entry.serverUrl} · ${entry.shelfId}`;
}

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

async function offlineScopeForEntry(entry: ShelfEntry): Promise<string | null> {
  if (entry.type === 'pcloud') {
    return cacheScopeKeyForEntry(entry);
  }
  if (entry.type === 'dav') {
    const { scope } = await LocalShelf.sourceShelfOfflineScope({
      source: entry.davSource, url: entry.davUrl, username: entry.davUsername ?? ''
    });
    return scope;
  }
  return null;
}

async function onRemove(): Promise<void> {
  const target = pendingRemoval.value;
  if (!target) {
    return;
  }

  busy.value = true;
  error.value = '';
  const wasActive = target.id === activeEntryID.value;
  try {
    if (target.type === 'local') {
      const { shelves } = await LocalShelf.listShelves();
      if (shelves.some((shelf) => shelf.id === target.localShelfId)) {
        await LocalShelf.removeShelf({ id: target.localShelfId });
      }
      await deleteShelfEntry(target.id);
    } else {
      const remaining = entries.value.filter((entry) => entry.id !== target.id);
      const cacheScope = cacheScopeKeyForEntry(target);
      const sharedCache = remaining.some((entry) => cacheScopeKeyForEntry(entry) === cacheScope);
      if (target.type === 'pcloud' || target.type === 'dav') {
        const peers = remaining.filter((entry) => entry.type === target.type);
        const scopes = await Promise.all(peers.map(offlineScopeForEntry));
        const sharedOffline = peers.length > 0 && scopes.includes(await offlineScopeForEntry(target));
        if (!sharedOffline) {
          await LocalShelf.removeVirtualShelfOffline({
            scope: cacheScope,
            ...(target.type === 'dav' ? { source: target.davSource, url: target.davUrl, username: target.davUsername ?? '' } : {})
          });
        }
      }
      await deleteShelfEntry(target.id);
      if (!sharedCache) {
        await removeCacheScope(cacheScope);
      }
    }
    pendingRemoval.value = null;
    await refresh();

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

<style scoped src="../styles/mobile-connect.css"></style>

<style scoped>
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

.mobile-shelves-local-warning {
  margin: 0;
  padding: 8px 12px;
  border-left: 3px solid var(--danger, #b3261e);
  font-size: 0.9rem;
}

/* .mobile-connect lays out a row; the prompt sits above the form. */
.mobile-shelves-first-run {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.mobile-shelves-restore {
  width: min(480px, 100%);
  box-sizing: border-box;
  padding: 24px;
  display: grid;
  gap: 12px;
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

.mobile-shelves-notice {
  margin: 0;
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
