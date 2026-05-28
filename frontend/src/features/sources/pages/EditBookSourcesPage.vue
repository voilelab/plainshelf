<template>
  <section class="source-editor-page">
    <ConfirmModal
      :open="showDiscardModal"
      title="Discard unsaved changes?"
      message="You have unsaved changes. Discard them and switch sources?"
      confirm-text="Discard and switch"
      cancel-text="Keep editing"
      @cancel="cancelPendingSource"
      @confirm="confirmPendingSource"
    />
    <ConfirmModal
      :open="showDeleteModal"
      title="Delete source?"
      confirm-text="Delete"
      cancel-text="Cancel"
      variant="danger"
      :busy="deleting"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    >
      <p>Are you sure you want to delete source <strong>{{ pendingDeleteSourceId }}</strong>? This action cannot be undone.</p>
      <p v-if="activeSourceId === pendingDeleteSourceId && isDirty" class="delete-warning" role="alert">You have unsaved changes that will be lost.</p>
      <p v-if="deleteError" class="delete-error" role="alert">{{ deleteError }}</p>
    </ConfirmModal>
    <header class="source-editor-topbar">
      <button class="button" type="button" @click="goBack">Back</button>

      <div class="topbar-title" :title="book?.title || bookId">{{ book?.title || bookId }}</div>
      <div class="topbar-sep">/</div>
      <div class="topbar-source" :title="activeSourceId || '-'">{{ activeSourceId || '-' }}</div>

      <div class="topbar-spacer"></div>

      <p v-if="saveSuccess" class="topbar-message success" role="status">{{ saveSuccess }}</p>
      <p v-else-if="isDirty" class="topbar-message dirty">Unsaved changes</p>
      <p v-else class="topbar-message">No pending changes</p>

      <button class="button primary" type="button" :disabled="disableSave" @click="onSave">
        {{ saving ? 'Saving...' : isDirty ? 'Save*' : 'Save' }}
      </button>
    </header>

    <div class="source-editor-workspace">
      <SourceList
        class="source-editor-sidebar"
        :sources="sources"
        :activeSourceId="activeSourceId"
        :currentSourceId="book?.current_source"
        :loading="listLoading"
        :creating="creating"
        @select="onSelectSource"
        @create="onCreateSource"
        @delete="onDeleteSource"
      />

      <main class="source-editor-main">
        <div v-if="initialLoading" class="loading editor-loading">Loading sources...</div>
        <div v-else-if="loadError" class="error source-error" role="alert">
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
import { getSourceContent, listSource, updateSourceContent, createSource, deleteSource } from '../../../api/sources';
import SourceEditor from '../components/SourceEditor.vue';
import SourceList from '../components/SourceList.vue';
import type { SourceMeta } from '../../../types/source';

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
const creating = ref(false);
const deleting = ref(false);

const loadError = ref('');
const editorError = ref('');
const saveSuccess = ref('');
const showDiscardModal = ref(false);
const pendingSourceId = ref('');
const showDeleteModal = ref(false);
const pendingDeleteSourceId = ref('');
const deleteError = ref('');

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

async function onCreateSource(): Promise<void> {
  creating.value = true;
  editorError.value = '';
  saveSuccess.value = '';

  try {
    const newSource = await createSource(bookId.value);
    await reloadSourceMeta();
    await loadSource(newSource.id);
  } catch (err) {
    editorError.value = err instanceof Error ? err.message : 'Failed to create source';
  } finally {
    creating.value = false;
  }
}

function onDeleteSource(sourceId: string): void {
  pendingDeleteSourceId.value = sourceId;
  deleteError.value = '';
  showDeleteModal.value = true;
}

function cancelDelete(): void {
  showDeleteModal.value = false;
  pendingDeleteSourceId.value = '';
  deleteError.value = '';
}

async function confirmDelete(): Promise<void> {
  const sourceId = pendingDeleteSourceId.value;
  if (!sourceId) {
    return;
  }

  deleting.value = true;
  deleteError.value = '';

  try {
    await deleteSource(bookId.value, sourceId);
    showDeleteModal.value = false;
    pendingDeleteSourceId.value = '';

    await reloadSourceMeta();

    if (activeSourceId.value === sourceId) {
      const preferredSource =
        sources.value.find((source) => source.id === book.value?.current_source)?.id ??
        sources.value[0]?.id ??
        '';

      if (preferredSource) {
        await loadSource(preferredSource);
      } else {
        activeSourceId.value = '';
        content.value = '';
        initialContent.value = '';
      }
    }
  } catch (err) {
    deleteError.value = err instanceof Error ? err.message : 'Failed to delete source';
  } finally {
    deleting.value = false;
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
.source-editor-page {
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

.source-editor-topbar {
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
.topbar-source {
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

.topbar-source {
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

.source-editor-workspace {
  flex: 1;
  min-height: 0;
  min-width: 0;
  box-sizing: border-box;
  display: flex;
  overflow: hidden;
}

.source-editor-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.source-error {
  margin: 12px;
  display: grid;
  gap: 10px;
}

.source-error p {
  margin: 0;
}

.source-error .button {
  justify-self: start;
}

.editor-loading {
  margin: 12px;
}

.delete-error {
  color: #b91c1c;
  margin-top: 8px;
}

.delete-warning {
  color: #92400e;
  margin-top: 8px;
}

@media (max-width: 900px) {
  .source-editor-topbar {
    height: auto;
    min-height: 56px;
    padding: 8px 12px;
    flex-wrap: wrap;
    row-gap: 8px;
  }

  .source-editor-workspace {
    flex-direction: column;
  }
}
</style>
