<template>
  <section class="source-editor-page">
    <ConfirmModal
      :open="showDiscardModal"
      :title="t('sources.page.discard.title')"
      :message="t('sources.page.discard.message')"
      :confirm-text="t('sources.page.discard.confirm')"
      :cancel-text="t('sources.page.discard.cancel')"
      @cancel="cancelPendingSource"
      @confirm="confirmPendingSource"
    />
    <ConfirmModal
      :open="showDeleteModal"
      :title="t('sources.page.deleteSource.title')"
      :confirm-text="t('sources.page.deleteSource.confirm')"
      variant="danger"
      :busy="deleting"
      @cancel="cancelDelete"
      @confirm="confirmDelete"
    >
      <p>{{ t('sources.page.deleteSource.question', { id: pendingDeleteSourceId }) }}</p>
      <p v-if="activeSourceId === pendingDeleteSourceId && isDirty" class="delete-warning" role="alert">{{ t('sources.page.deleteSource.dirtyWarning') }}</p>
      <p v-if="deleteError" class="delete-error" role="alert">{{ deleteError }}</p>
    </ConfirmModal>
    <SourceConversionModal
      :open="showConversionModal"
      :kind="conversionKind"
      :source-id="activeSourceId"
      :content="content"
      :legacy-config="conversionLegacyConfig"
      :busy="creating"
      :error="conversionError"
      @cancel="closeConversionModal"
      @create="submitConversion"
    />
    <ConfirmModal
      :open="pendingRenameChapterIndex !== null"
      :title="t('sources.page.renameChapter.title')"
      :confirm-text="t('sources.page.renameChapter.confirm')"
      :confirm-disabled="!pendingChapterTitle.trim()"
      @cancel="cancelRenameChapter"
      @confirm="confirmRenameChapter"
    >
      <form class="chapter-edit-form" @submit.prevent="confirmRenameChapter">
        <label>
          <span>{{ t('sources.page.renameChapter.titleLabel') }}</span>
          <input
            ref="chapterTitleInput"
            v-model="pendingChapterTitle"
            class="input"
            type="text"
            autocomplete="off"
          >
        </label>
      </form>
    </ConfirmModal>
    <ConfirmModal
      :open="pendingMergeChapterIndex !== null"
      :title="t('sources.page.mergeChapter.title')"
      :confirm-text="t('sources.page.mergeChapter.confirm')"
      variant="danger"
      @cancel="cancelMergeChapter"
      @confirm="confirmMergeChapter"
    >
      <p>{{ t('sources.page.mergeChapter.question', { title: pendingMergeChapterTitle }) }}</p>
    </ConfirmModal>
    <header class="source-editor-topbar">
      <button class="button" type="button" @click="goBack">{{ t('sources.page.back') }}</button>

      <div class="topbar-title" :title="book?.title || bookId">{{ book?.title || bookId }}</div>
      <div class="topbar-sep">/</div>
      <div class="topbar-source" :title="activeSourceId || '-'">{{ activeSourceId || '-' }}</div>

      <div class="topbar-spacer"></div>

      <p v-if="saveSuccess" class="topbar-message success" role="status">{{ saveSuccess }}</p>
      <p v-else-if="isDirty" class="topbar-message dirty">{{ t('sources.editor.dirty') }}</p>
      <p v-else class="topbar-message">{{ t('sources.editor.clean') }}</p>

      <button class="button primary" type="button" :disabled="disableSave" @click="onSave">
        {{ saving ? t('sources.page.saving') : isDirty ? t('sources.page.saveDirty') : t('sources.page.save') }}
      </button>
    </header>

    <nav class="mobile-pane-tabs" :aria-label="t('sources.page.panelsLabel')">
      <button type="button" :class="{ active: mobilePane === 'sources' }" @click="mobilePane = 'sources'">{{ t('sources.page.paneSources') }}</button>
      <button type="button" :class="{ active: mobilePane === 'editor' }" @click="mobilePane = 'editor'">{{ t('sources.page.paneEditor') }}</button>
      <button v-if="activeFormat === 'md'" type="button" :class="{ active: mobilePane === 'chapters' }" @click="mobilePane = 'chapters'">{{ t('sources.page.paneChapters') }}</button>
    </nav>

    <SplitterGroup
      direction="horizontal"
      as="div"
      auto-save-id="plainshelf-sources"
      :keyboard-resize-by="16"
      class="source-editor-workspace"
    >
      <SplitterPanel
        as="div"
        class="source-editor-sidebar"
        :class="{ 'mobile-active': mobilePane === 'sources' }"
        size-unit="px"
        :default-size="300"
        :min-size="240"
        :max-size="360"
      >
        <SourceList
          :sources="sources"
          :activeSourceId="activeSourceId"
          :currentSourceId="book?.current_source"
          :loading="listLoading || initialLoading"
          :creating="creating"
          @select="onSelectSource"
          @create="onCreateSource"
          @delete="onDeleteSource"
        />
      </SplitterPanel>

      <SplitterResizeHandle as="div" class="reka-resize-handle" :hit-area-margins="SOURCE_LIST_RESIZE_HIT_AREA_MARGINS" />

      <SplitterPanel as="main" class="source-editor-main" :class="{ 'mobile-active': mobilePane === 'editor' }">
        <div v-if="initialLoading" class="loading editor-loading">{{ t('sources.page.loading') }}</div>
        <div v-else-if="loadError" class="error source-error" role="alert">
          <p>{{ loadError }}</p>
          <button class="button" type="button" @click="fetchInitial">{{ t('common.retry') }}</button>
        </div>
        <template v-else>
          <SourceFormatActions
            v-if="activeSourceId"
            :format="activeFormat"
            :legacy="isLegacySource"
            :disabled="isDirty || creating"
            @convert="onConvertSource"
          />
          <SourceEditor
            ref="editorRef"
            :modelValue="content"
            :sourceId="activeSourceId"
            :loading="contentLoading"
            :saving="saving"
            :dirty="isDirty"
            :error="editorError"
            :isCurrent="activeSourceId === book?.current_source"
            :settingCurrent="settingCurrent"
            :viewRange="editorViewRange"
            :focused="editorFocused"
            @document-edit="onDocumentEdit"
            @request-view-offset="onRequestViewOffset"
            @set-current="onSetCurrentSource"
          />
        </template>
      </SplitterPanel>

      <template v-if="activeFormat === 'md' && !isLegacySource">
        <SplitterResizeHandle as="div" class="reka-resize-handle outline-resize" :hit-area-margins="SOURCE_LIST_RESIZE_HIT_AREA_MARGINS" />
        <SplitterPanel
          as="div"
          class="source-editor-outline"
          :class="{ 'mobile-active': mobilePane === 'chapters' }"
          size-unit="px"
          :default-size="300"
          :min-size="220"
          :max-size="380"
        >
          <ChapterOutline
            :sections="markdownSections"
            :focused="editorFocused"
            :activeSectionIndex="activeEditorSection?.index ?? null"
            :disabled="saving || contentLoading"
            @insert="insertChapter"
            @show-all="showWholeSource"
            @select="selectEditorSection"
            @rename="openRenameChapter"
            @remove="openMergeChapter"
          />
        </SplitterPanel>
      </template>
    </SplitterGroup>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { SplitterGroup, SplitterPanel, SplitterResizeHandle } from 'reka-ui';
import ConfirmModal from '@/components/ConfirmModal.vue';
import { getDefaultSplitConfigSetting } from '@/api/settings';
import { useDocumentTitle } from '@/composables/useDocumentTitle';
import { useWriteAccess } from '@/composables/useWriteAccess';
import {
  buildMarkdownEditorSections,
  findMarkdownEditorSection,
  type MarkdownEditorSection
} from '@/features/reader/utils/markdownChapters';
import ChapterOutline from '@/features/sources/components/ChapterOutline.vue';
import SourceConversionModal, {
  type SourceConversionKind
} from '@/features/sources/components/SourceConversionModal.vue';
import SourceEditor from '@/features/sources/components/SourceEditor.vue';
import SourceFormatActions from '@/features/sources/components/SourceFormatActions.vue';
import SourceList from '@/features/sources/components/SourceList.vue';
import { useSourceEditorSession } from '@/features/sources/composables/useSourceEditorSession';
import type {
  SourceDocumentEdit,
  SourceEditorAdapter,
  SourceEditorViewRange
} from '@/features/sources/types/editorAdapter';
import {
  insertChapterEdit,
  mergeChapterEdit,
  renameChapterEdit,
  type SourceTextEdit
} from '@/features/sources/utils/chapterEdits';
import {
  mapOffsetThroughReplacement,
  replaceTextRange
} from '@/features/sources/utils/textEditing';
import type { SplitConfig } from '@/types/book';
import { useI18n } from '@/i18n';

const { t } = useI18n();

type PendingSourceTransition =
  | { kind: 'select'; sourceId: string }
  | { kind: 'create' };

type EditorViewState =
  | { kind: 'all' }
  | {
      kind: 'section';
      startOffset: number;
      endOffset: number;
      anchorOffset: number;
      affinity: 'forward' | 'backward';
    };

// Stable reference: see MainLayout.vue's SIDEBAR_RESIZE_HIT_AREA_MARGINS for why this
// object must not be an inline template literal (reka-ui re-registers the resize handle
// on every identity change, which can strand `pointer-events: none` mid-drag).
const SOURCE_LIST_RESIZE_HIT_AREA_MARGINS = { coarse: 12, fine: 6 };

const { writesEnabled } = useWriteAccess();
const route = useRoute();
const router = useRouter();
const bookId = computed(() => String(route.params.bookId));
const session = useSourceEditorSession(() => bookId.value, () => writesEnabled.value);
const {
  book,
  sources,
  activeSourceId,
  content,
  initialLoading,
  listLoading,
  contentLoading,
  saving,
  creating,
  deleting,
  settingCurrent,
  loadError,
  editorError,
  saveSuccess,
  deleteError,
  conversionError,
  activeSourceMeta,
  isLegacySource,
  activeFormat,
  isDirty,
  disableSave,
  fetchInitial,
  loadSource,
  save: onSave,
  setCurrentSource: onSetCurrentSource,
  createSource,
  createDerivedSource,
  deleteSource
} = session;

const editorRef = ref<SourceEditorAdapter | null>(null);
const mobilePane = ref<'sources' | 'editor' | 'chapters'>('editor');
const pendingTransition = ref<PendingSourceTransition | null>(null);
const showDiscardModal = computed(() => pendingTransition.value !== null);
const showDeleteModal = ref(false);
const pendingDeleteSourceId = ref('');
const showConversionModal = ref(false);
const conversionKind = ref<SourceConversionKind>('manual-md');
const conversionLegacyConfig = ref<SplitConfig>({ type: 'none' });
const pendingRenameChapterIndex = ref<number | null>(null);
const pendingChapterTitle = ref('');
const chapterTitleInput = ref<HTMLInputElement | null>(null);
const pendingMergeChapterIndex = ref<number | null>(null);
const editorViewState = ref<EditorViewState>({ kind: 'all' });
const editorViewKey = ref(0);
let editorViewReconcileTimer: ReturnType<typeof setTimeout> | null = null;

const markdownSections = computed(() =>
  activeFormat.value === 'md' && !isLegacySource.value
    ? buildMarkdownEditorSections(content.value)
    : []
);
const chapterHeadings = computed(() =>
  markdownSections.value.flatMap((section) => section.heading ? [section.heading] : [])
);
const hasMarkdownChapters = computed(() =>
  markdownSections.value.some((section) => section.kind === 'chapter')
);
const activeEditorSection = computed(() => {
  const view = editorViewState.value;
  if (view.kind !== 'section' || !hasMarkdownChapters.value) return null;
  return findMarkdownEditorSection(markdownSections.value, view.anchorOffset, view.affinity);
});
const editorFocused = computed(() => editorViewState.value.kind === 'section');
const editorViewRange = computed<SourceEditorViewRange | null>(() => {
  const view = editorViewState.value;
  return view.kind === 'section'
    ? { startOffset: view.startOffset, endOffset: view.endOffset, key: editorViewKey.value }
    : null;
});
const pendingMergeChapterTitle = computed(() => {
  const index = pendingMergeChapterIndex.value;
  return index === null ? '' : chapterHeadings.value[index]?.title ?? '';
});

useDocumentTitle(() => [t('sources.page.title'), book.value?.title, t('app.name')]);

async function selectSource(sourceId: string): Promise<void> {
  await loadSource(sourceId);
  if (activeSourceId.value === sourceId) mobilePane.value = 'editor';
}

async function onSelectSource(sourceId: string): Promise<void> {
  if (sourceId === activeSourceId.value) return;
  if (isDirty.value) {
    pendingTransition.value = { kind: 'select', sourceId };
    return;
  }
  await selectSource(sourceId);
}

function cancelPendingSource(): void {
  pendingTransition.value = null;
}

async function confirmPendingSource(): Promise<void> {
  const transition = pendingTransition.value;
  pendingTransition.value = null;
  if (!transition) return;
  if (transition.kind === 'create') {
    await doCreateSource();
  } else if (transition.sourceId !== activeSourceId.value) {
    await selectSource(transition.sourceId);
  }
}

async function onCreateSource(): Promise<void> {
  if (!writesEnabled.value) return;
  if (isDirty.value) {
    pendingTransition.value = { kind: 'create' };
    return;
  }
  await doCreateSource();
}

async function doCreateSource(): Promise<void> {
  if (await createSource()) mobilePane.value = 'editor';
}

async function onConvertSource(kind: SourceConversionKind): Promise<void> {
  if (kind === 'legacy-upgrade') {
    const requestedBookId = bookId.value;
    const requestedSourceId = activeSourceId.value;
    let config = activeSourceMeta.value?.split_config ?? { type: 'none' as const };
    if (config.type === 'none') {
      config = await getDefaultSplitConfigSetting().catch(() => config);
    }
    if (
      bookId.value !== requestedBookId ||
      activeSourceId.value !== requestedSourceId
    ) return;
    conversionLegacyConfig.value = config;
  }
  conversionKind.value = kind;
  conversionError.value = '';
  showConversionModal.value = true;
}

function closeConversionModal(): void {
  if (creating.value) return;
  showConversionModal.value = false;
  conversionError.value = '';
}

async function submitConversion(payload: {
  content: string;
  format: 'txt' | 'md';
  comment: string;
  setCurrent: boolean;
}): Promise<void> {
  if (await createDerivedSource(payload)) {
    showConversionModal.value = false;
    mobilePane.value = 'editor';
  }
}

function showWholeSource(): void {
  clearEditorViewReconcile();
  if (editorViewState.value.kind === 'all') return;
  editorViewState.value = { kind: 'all' };
  editorViewKey.value += 1;
}

function resetEditorView(): void {
  clearEditorViewReconcile();
  editorViewState.value = { kind: 'all' };
  editorViewKey.value += 1;
}

function selectEditorSection(section: MarkdownEditorSection): void {
  if (!hasMarkdownChapters.value) return;
  clearEditorViewReconcile();
  const previous = activeEditorSection.value;
  if (!previous || previous.index !== section.index) editorViewKey.value += 1;
  editorViewState.value = {
    kind: 'section',
    startOffset: section.startOffset,
    endOffset: section.endOffset,
    anchorOffset: section.startOffset,
    affinity: 'forward'
  };
  mobilePane.value = 'editor';
  void nextTick(() => editorRef.value?.focusAndSelect(section.startOffset, section.startOffset));
}

function onRequestViewOffset(
  offset: number,
  affinity: 'forward' | 'backward'
): void {
  if (!editorFocused.value) return;
  const previous = activeEditorSection.value;
  const next = findMarkdownEditorSection(markdownSections.value, offset, affinity);
  if (!next) return;
  if (!previous || previous.index !== next.index) editorViewKey.value += 1;
  clearEditorViewReconcile();
  editorViewState.value = {
    kind: 'section',
    startOffset: next.startOffset,
    endOffset: next.endOffset,
    anchorOffset: offset,
    affinity
  };
  mobilePane.value = 'editor';
}

function onDocumentEdit(edit: SourceDocumentEdit): void {
  const previous = activeEditorSection.value;
  const previousLength = content.value.length;
  const view = editorViewState.value;
  content.value = edit.value;
  if (view.kind !== 'section') return;

  if (edit.deferView) {
    const delta = edit.value.length - previousLength;
    editorViewState.value = {
      kind: 'section',
      startOffset: view.startOffset,
      endOffset: Math.max(view.startOffset, view.endOffset + delta),
      anchorOffset: edit.selectionEnd,
      affinity: edit.affinity
    };
    if (edit.composing) clearEditorViewReconcile();
    else scheduleEditorViewReconcile(previous);
    return;
  }

  clearEditorViewReconcile();
  reconcileEditorView(previous, edit.selectionEnd, edit.affinity);
}

function scheduleEditorViewReconcile(previous: MarkdownEditorSection | null): void {
  clearEditorViewReconcile();
  editorViewReconcileTimer = setTimeout(() => {
    editorViewReconcileTimer = null;
    const view = editorViewState.value;
    if (view.kind === 'section') {
      reconcileEditorView(previous, view.anchorOffset, view.affinity);
    }
  }, 80);
}

function clearEditorViewReconcile(): void {
  if (editorViewReconcileTimer !== null) {
    clearTimeout(editorViewReconcileTimer);
    editorViewReconcileTimer = null;
  }
}

onBeforeUnmount(clearEditorViewReconcile);

function reconcileEditorView(
  previous: MarkdownEditorSection | null,
  selectionEnd: number,
  affinity: 'forward' | 'backward'
): void {
  if (editorViewState.value.kind !== 'section') return;

  if (!hasMarkdownChapters.value) {
    showWholeSource();
    return;
  }
  const next = findMarkdownEditorSection(markdownSections.value, selectionEnd, affinity);
  if (!next) {
    showWholeSource();
    return;
  }
  const currentView = editorViewState.value;
  if (
    !previous ||
    previous.kind !== next.kind ||
    currentView.startOffset !== next.startOffset ||
    currentView.endOffset !== next.endOffset
  ) {
    editorViewKey.value += 1;
  }
  editorViewState.value = {
    kind: 'section',
    startOffset: next.startOffset,
    endOffset: next.endOffset,
    anchorOffset: selectionEnd,
    affinity
  };
}

function applyEdit(
  edit: SourceTextEdit,
  followEdit = true,
  affinity: 'forward' | 'backward' = 'forward',
  focus = true
): void {
  clearEditorViewReconcile();
  const previous = activeEditorSection.value;
  const replacement = replaceTextRange(content.value, edit.start, edit.end, edit.replacement);

  if (followEdit || !previous || editorViewState.value.kind !== 'section') {
    onDocumentEdit({
      value: replacement.value,
      selectionStart: replacement.selectionStart,
      selectionEnd: replacement.selectionEnd,
      affinity
    });
    if (focus) {
      void nextTick(() => editorRef.value?.focusAndSelect(
        replacement.selectionStart,
        replacement.selectionEnd
      ));
    }
    return;
  }

  const view = editorViewState.value;
  const mappedAnchor = mapOffsetThroughReplacement(
    view.anchorOffset,
    edit.start,
    edit.end,
    edit.replacement.length,
    view.affinity
  );
  content.value = replacement.value;
  const next = findMarkdownEditorSection(markdownSections.value, mappedAnchor, view.affinity);
  if (!hasMarkdownChapters.value || !next) {
    showWholeSource();
    return;
  }
  if (previous.kind !== next.kind || previous.startOffset !== next.startOffset) {
    editorViewKey.value += 1;
  }
  editorViewState.value = {
    kind: 'section',
    startOffset: next.startOffset,
    endOffset: next.endOffset,
    anchorOffset: mappedAnchor,
    affinity: view.affinity
  };
}

function insertChapter(): void {
  const editor = editorRef.value;
  if (!editor) return;
  applyEdit(insertChapterEdit(content.value, editor.getCurrentParagraphStart()));
  mobilePane.value = 'editor';
}

async function openRenameChapter(index: number): Promise<void> {
  const heading = chapterHeadings.value[index];
  if (!heading) return;
  pendingRenameChapterIndex.value = index;
  pendingChapterTitle.value = heading.title;
  await nextTick();
  await nextTick();
  chapterTitleInput.value?.focus();
  chapterTitleInput.value?.select();
}

function cancelRenameChapter(): void {
  pendingRenameChapterIndex.value = null;
  pendingChapterTitle.value = '';
}

function confirmRenameChapter(): void {
  const index = pendingRenameChapterIndex.value;
  const title = pendingChapterTitle.value.trim();
  if (index === null || !title) return;
  const heading = chapterHeadings.value[index];
  if (heading) {
    const followsActive = activeEditorSection.value?.headingIndex === index;
    applyEdit(renameChapterEdit(content.value, heading, title), followsActive, 'forward', followsActive);
  }
  cancelRenameChapter();
}

function openMergeChapter(index: number): void {
  if (chapterHeadings.value[index]) pendingMergeChapterIndex.value = index;
}

function cancelMergeChapter(): void {
  pendingMergeChapterIndex.value = null;
}

function confirmMergeChapter(): void {
  const index = pendingMergeChapterIndex.value;
  if (index === null) return;
  const heading = chapterHeadings.value[index];
  if (heading) {
    const followsActive = activeEditorSection.value?.headingIndex === index;
    applyEdit(mergeChapterEdit(content.value, heading), followsActive, 'backward', false);
  }
  cancelMergeChapter();
}

function onDeleteSource(sourceId: string): void {
  if (!writesEnabled.value) return;
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
  if (!sourceId) return;
  if (await deleteSource(sourceId)) cancelDelete();
}

function goBack(): void {
  void router.push(`/books/${bookId.value}`);
}

watch(activeSourceId, (sourceId, previousSourceId) => {
  if (sourceId !== previousSourceId) resetEditorView();
});

watch(
  bookId,
  () => {
    pendingTransition.value = null;
    showDeleteModal.value = false;
    pendingDeleteSourceId.value = '';
    showConversionModal.value = false;
    pendingRenameChapterIndex.value = null;
    pendingMergeChapterIndex.value = null;
    resetEditorView();
    mobilePane.value = 'editor';
    void fetchInitial();
  },
  { immediate: true }
);
</script>

<style scoped>
.source-editor-page {
  height: calc(100vh / var(--app-zoom, 1));
  width: calc(100vw / var(--app-zoom, 1));
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

.mobile-pane-tabs {
  display: none;
}

.source-editor-workspace {
  flex: 1;
  min-height: 0;
  min-width: 0;
  box-sizing: border-box;
  display: flex;
  overflow: hidden;
}

.source-editor-sidebar {
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
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

.source-editor-outline {
  min-width: 0;
  min-height: 0;
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

.chapter-edit-form label {
  display: grid;
  gap: 7px;
}

.chapter-edit-form span {
  color: var(--text);
  font-weight: 700;
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

  .mobile-pane-tabs {
    display: flex;
    border-bottom: 1px solid var(--border);
    background: #f8fafc;
  }

  .mobile-pane-tabs button {
    flex: 1;
    border: 0;
    background: transparent;
    padding: 9px;
  }

  .mobile-pane-tabs button.active {
    color: var(--accent);
    box-shadow: inset 0 -2px var(--accent);
  }

  .source-editor-sidebar,
  .source-editor-main,
  .source-editor-outline {
    display: none;
    width: 100% !important;
    max-width: none !important;
    flex: 1 1 auto !important;
  }

  .source-editor-sidebar.mobile-active,
  .source-editor-main.mobile-active,
  .source-editor-outline.mobile-active {
    display: flex;
  }

  .reka-resize-handle {
    display: none;
  }
}
</style>
