<template>
  <section class="snapshot-editor-page">
    <ConfirmModal
      :open="showDiscardModal"
      title="Discard unsaved changes?"
      message="You have unsaved changes. Discard them and switch sources?"
      confirm-text="Discard and switch"
      cancel-text="Keep editing"
      @cancel="cancelPendingSource"
      @confirm="confirmPendingSource"
    />
    <header class="snapshot-editor-topbar">
      <button class="button" type="button" @click="goBack">Back</button>

      <div class="topbar-title" :title="book?.title || bookId">{{ book?.title || bookId }}</div>
      <div class="topbar-sep">/</div>
      <div class="topbar-snapshot" :title="activeSourceId || '-'">{{ activeSourceId || '-' }}</div>

      <div class="topbar-spacer"></div>

      <p v-if="saveSuccess" class="topbar-message success" role="status">{{ saveSuccess }}</p>
      <p v-else-if="isDirty" class="topbar-message dirty">Unsaved changes</p>
      <p v-else class="topbar-message">No pending changes</p>

      <button class="button primary" type="button" :disabled="disableSave" @click="onSave">
        {{ saving ? 'Saving...' : isDirty ? 'Save*' : 'Save' }}
      </button>
    </header>

    <div class="snapshot-editor-workspace">
      <SourceList
        class="snapshot-editor-sidebar"
        :sources="sources"
        :activeSourceId="activeSourceId"
        :currentSourceId="book?.current_source"
        :loading="listLoading"
        @select="onSelectSource"
      />

      <main class="snapshot-editor-main">
        <div v-if="initialLoading" class="loading editor-loading">Loading sources...</div>
        <div v-else-if="loadError" class="error snapshot-error" role="alert">
          <p>{{ loadError }}</p>
          <button class="button" type="button" @click="fetchInitial">Retry</button>
        </div>
        <SourceEditor
          v-else
          v-model="content"
          :sourceId="activeSourceId"
          :loading="contentLoading"
          :saving="saving"
          :dirty="isDirty"
          :error="editorError"
        />
      </main>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { getBook } from '../../../api/books';
import ConfirmModal from '../../../components/ConfirmModal.vue';
import { useDocumentTitle } from '../../../composables/useDocumentTitle';
import type { Book } from '../../../types/book';
import { getSourceContent, listSource, updateSourceContent } from '../../../api/snapshots';
import SourceEditor from '../components/SnapshotEditor.vue';
import SourceList from '../components/SnapshotList.vue';
import type { SourceMeta } from '../../../types/snapsnot';

const route = useRoute();
const router = useRouter();

const bookId = computed(() => String(route.params.bookId));

const book = ref<Book | null>(null);
const sources = ref<SourceMeta[]>([]);
const activeSourceId = ref('');
const initialContent = ref('');
const content = ref('');

const initialLoading = ref(false);
const listLoading = ref(false);
const contentLoading = ref(false);
const saving = ref(false);

const loadError = ref('');
const editorError = ref('');
const saveSuccess = ref('');
const showDiscardModal = ref(false);
const pendingSourceId = ref('');

const isDirty = computed(() => activeSourceId.value.length > 0 && content.value !== initialContent.value);
const disableSave = computed(
  () =>
    !activeSourceId.value ||
    !isDirty.value ||
    saving.value ||
    contentLoading.value ||
    initialLoading.value
);

useDocumentTitle(() => ['Edit Sources', book.value?.title, 'PlainShelf']);

async function fetchInitial(): Promise<void> {
  initialLoading.value = true;
  loadError.value = '';
  editorError.value = '';
  saveSuccess.value = '';

  try {
    const [bookData, sourceList] = await Promise.all([
      getBook(bookId.value),
      listSource(bookId.value)
    ]);

    book.value = bookData;
    sources.value = sourceList;

    const preferredSource =
      sourceList.find((source) => source.id === bookData.current_source)?.id ??
      sourceList[0]?.id ??
      '';

    if (preferredSource) {
      await loadSource(preferredSource);
    } else {
      activeSourceId.value = '';
      content.value = '';
      initialContent.value = '';
    }
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : 'Failed to load sources';
  } finally {
    initialLoading.value = false;
  }
}

async function reloadSourceMeta(): Promise<void> {
  listLoading.value = true;
  try {
    sources.value = await listSource(bookId.value);
  } finally {
    listLoading.value = false;
  }
}

async function loadSource(sourceId: string): Promise<void> {
  contentLoading.value = true;
  editorError.value = '';
  saveSuccess.value = '';

  try {
    const text = await getSourceContent(bookId.value, sourceId);
    activeSourceId.value = sourceId;
    content.value = text;
    initialContent.value = text;
  } catch (err) {
    editorError.value = err instanceof Error ? err.message : 'Failed to load source content';
  } finally {
    contentLoading.value = false;
  }
}

async function onSelectSource(sourceId: string): Promise<void> {
  if (sourceId === activeSourceId.value) {
    return;
  }

  if (isDirty.value) {
    pendingSourceId.value = sourceId;
    showDiscardModal.value = true;
    return;
  }

  await loadSource(sourceId);
}

function cancelPendingSource(): void {
  showDiscardModal.value = false;
  pendingSourceId.value = '';
}

async function confirmPendingSource(): Promise<void> {
  const sourceId = pendingSourceId.value;
  cancelPendingSource();

  if (!sourceId || sourceId === activeSourceId.value) {
    return;
  }

  await loadSource(sourceId);
}

async function onSave(): Promise<void> {
  if (!activeSourceId.value || !isDirty.value) {
    return;
  }

  saving.value = true;
  editorError.value = '';
  saveSuccess.value = '';

  try {
    await updateSourceContent(bookId.value, activeSourceId.value, content.value);
    initialContent.value = content.value;
    await reloadSourceMeta();
    saveSuccess.value = 'Source saved.';
  } catch (err) {
    editorError.value = err instanceof Error ? err.message : 'Failed to save source';
  } finally {
    saving.value = false;
  }
}

function goBack(): void {
  void router.push(`/books/${bookId.value}`);
}

watch(
  bookId,
  () => {
    void fetchInitial();
  },
  { immediate: true }
);
</script>

<style scoped>
.snapshot-editor-page {
  height: 100vh;
  width: 100vw;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  background: #fff;
}

.snapshot-editor-topbar {
  height: 56px;
  flex-shrink: 0;
  min-width: 0;
  box-sizing: border-box;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  background: #f9fbfd;
  display: flex;
  align-items: center;
  gap: 12px;
}

.topbar-title,
.topbar-snapshot {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-title {
  font-size: 18px;
  font-weight: 700;
}

.topbar-sep {
  color: var(--muted);
}

.topbar-snapshot {
  color: var(--muted);
  max-width: min(36vw, 360px);
}

.topbar-spacer {
  flex: 1;
}

.topbar-message {
  margin: 0;
  color: var(--muted);
}

.topbar-message.dirty {
  color: #9a3412;
}

.topbar-message.success {
  color: #166534;
}

.snapshot-editor-workspace {
  flex: 1;
  min-height: 0;
  min-width: 0;
  box-sizing: border-box;
  display: flex;
  overflow: hidden;
}

.snapshot-editor-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.snapshot-error {
  margin: 12px;
  display: grid;
  gap: 10px;
}

.snapshot-error p {
  margin: 0;
}

.snapshot-error .button {
  justify-self: start;
}

.editor-loading {
  margin: 12px;
}

@media (max-width: 900px) {
  .snapshot-editor-topbar {
    height: auto;
    min-height: 56px;
    padding: 8px 12px;
    flex-wrap: wrap;
    row-gap: 8px;
  }

  .snapshot-editor-workspace {
    flex-direction: column;
  }
}
</style>
