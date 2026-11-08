<template>
  <section class="page-list-editor" :aria-label="t('sources.pages.title')">
    <ConfirmModal
      :open="pendingDelete !== null"
      :title="t('sources.pages.deleteTitle')"
      :confirm-text="t('sources.pages.deleteConfirm')"
      variant="danger"
      :busy="busy"
      @cancel="pendingDelete = null"
      @confirm="confirmDelete"
    >
      <p>{{ pendingDelete?.missing ? t('sources.pages.removeMissingQuestion') : t('sources.pages.deleteQuestion') }}</p>
    </ConfirmModal>

    <div class="page-toolbar">
      <span class="format-badge">IMAGES</span>
      <label class="direction-field">
        <span>{{ t('sources.pages.direction') }}</span>
        <select
          class="input"
          :value="direction"
          :disabled="!writesEnabled || busy || loading"
          data-testid="page-direction"
          @change="onDirectionChange"
        >
          <option value="ltr">{{ t('sources.pages.ltr') }}</option>
          <option value="rtl">{{ t('sources.pages.rtl') }}</option>
        </select>
      </label>
      <button class="button" type="button" :disabled="!canEdit" @click="fileInput?.click()">
        {{ selectedIndex === null ? t('sources.pages.addAtEnd') : t('sources.pages.addAfter', { number: selectedIndex + 1 }) }}
      </button>
      <input
        ref="fileInput"
        class="page-file-input"
        type="file"
        accept="image/jpeg,image/png,image/webp,image/gif"
        multiple
        data-testid="page-upload"
        @change="onFilesChosen"
      >
      <span class="meta">{{ busy ? t('sources.pages.saving') : t('sources.pages.count', { count: pageList?.pages.length ?? 0 }) }}</span>
      <div class="toolbar-spacer"></div>
      <button v-if="!isCurrent" class="button" type="button" :disabled="settingCurrent" @click="emit('set-current')">
        {{ settingCurrent ? t('sources.editor.settingCurrent') : t('sources.editor.setCurrent') }}
      </button>
    </div>

    <div v-if="error" class="error page-error" role="alert">
      <p>{{ error }}</p>
      <button v-if="conflict" class="button" type="button" @click="reload">{{ t('sources.pages.reload') }}</button>
    </div>

    <div v-if="loading" class="loading page-loading">{{ t('sources.editor.loading') }}</div>
    <div v-else-if="pageList" ref="scrollEl" class="page-scroll">
      <p v-if="writesEnabled" class="meta page-hint">{{ t('sources.pages.hint') }}</p>
      <ol class="page-grid" data-testid="page-grid">
        <li
          v-for="(page, index) in pageList.pages"
          :key="`${index}:${page.name}`"
          class="page-cell"
          :class="{ selected: selectedIndex === index, missing: page.missing, 'drop-target': dropIndex === index }"
          :draggable="canEdit"
          :data-page-name="page.name"
          @dragstart="onDragStart(index, $event)"
          @dragover.prevent="dropIndex = index"
          @dragleave="dropIndex = dropIndex === index ? null : dropIndex"
          @drop.prevent="onDrop(index)"
          @dragend="clearDrag"
        >
          <button
            class="page-select"
            type="button"
            :aria-pressed="selectedIndex === index"
            :aria-label="t('sources.pages.pageLabel', { number: index + 1 })"
            @click="select(index)"
          >
            <PageThumb
              :book-id="bookId"
              :source-id="sourceId"
              :name="page.name"
              :missing="page.missing"
              :alt="t('sources.pages.pageLabel', { number: index + 1 })"
              :scroll-root="scrollEl"
            />
          </button>
          <div class="page-actions">
            <span class="page-number">{{ index + 1 }}</span>
            <button class="icon-button" type="button" :disabled="!canEdit || index === 0" :aria-label="t('sources.pages.moveEarlier')" @click="reorder(index, index - 1)">‹</button>
            <button class="icon-button" type="button" :disabled="!canEdit || index === pageList.pages.length - 1" :aria-label="t('sources.pages.moveLater')" @click="reorder(index, index + 1)">›</button>
            <button class="icon-button danger" type="button" :disabled="!canEdit" :aria-label="page.missing ? t('sources.pages.removeMissing') : t('sources.pages.delete')" @click="pendingDelete = { name: page.name, missing: page.missing }">×</button>
          </div>
        </li>
      </ol>
      <p v-if="pageList.pages.length === 0" class="meta">{{ t('sources.pages.empty') }}</p>

      <section v-if="pageList.unlisted.length > 0" class="unlisted" data-testid="unlisted-pages">
        <h3>{{ t('sources.pages.unlistedTitle') }}</h3>
        <p class="meta">{{ t('sources.pages.unlistedHelp') }}</p>
        <ul class="page-grid">
          <li v-for="name in pageList.unlisted" :key="name" class="page-cell" :data-page-name="name">
            <PageThumb :book-id="bookId" :source-id="sourceId" :name="name" :alt="name" :scroll-root="scrollEl" />
            <div class="page-actions">
              <button class="button" type="button" :disabled="!canEdit" @click="appendUnlisted(name)">{{ t('sources.pages.addToList') }}</button>
              <button class="icon-button danger" type="button" :disabled="!canEdit" :aria-label="t('sources.pages.delete')" @click="pendingDelete = { name, missing: false }">×</button>
            </div>
          </li>
        </ul>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import ConfirmModal from '@/components/ConfirmModal.vue';
import PageThumb from '@/features/sources/components/PageThumb.vue';
import { usePageListEditor } from '@/features/sources/composables/usePageListEditor';
import { useI18n } from '@/i18n';
import type { PageDirection } from '@/types/source';

const props = defineProps<{
  bookId: string;
  sourceId: string;
  initialDirection: PageDirection;
  writesEnabled: boolean;
  isCurrent: boolean;
  settingCurrent: boolean;
}>();

const emit = defineEmits<{
  changed: [];
  'set-current': [];
}>();

const { t } = useI18n();
const fileInput = ref<HTMLInputElement | null>(null);
const scrollEl = ref<HTMLElement | null>(null);
const pendingDelete = ref<{ name: string; missing: boolean } | null>(null);
const dragIndex = ref<number | null>(null);
const dropIndex = ref<number | null>(null);

const {
  pageList,
  direction,
  selectedIndex,
  loading,
  busy,
  error,
  conflict,
  canEdit,
  load,
  reorder,
  appendUnlisted,
  removeImage,
  addImages,
  setDirection,
  select
} = usePageListEditor(
  () => props.bookId,
  () => props.sourceId,
  () => props.writesEnabled,
  () => emit('changed')
);

function reload(): void {
  void load(props.initialDirection);
}

function onDirectionChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value;
  void setDirection(value === 'rtl' ? 'rtl' : 'ltr');
}

async function onFilesChosen(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = '';
  if (files.length > 0) await addImages(files);
}

async function confirmDelete(): Promise<void> {
  const target = pendingDelete.value;
  if (!target) return;
  await removeImage(target.name);
  pendingDelete.value = null;
}

function onDragStart(index: number, event: DragEvent): void {
  if (!canEdit.value) {
    event.preventDefault();
    return;
  }
  dragIndex.value = index;
  event.dataTransfer?.setData('text/plain', String(index));
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
}

async function onDrop(index: number): Promise<void> {
  const from = dragIndex.value;
  clearDrag();
  if (from !== null) await reorder(from, index);
}

function clearDrag(): void {
  dragIndex.value = null;
  dropIndex.value = null;
}

watch(() => [props.bookId, props.sourceId] as const, reload, { immediate: true });
</script>

<style scoped>
.page-list-editor {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.page-toolbar {
  min-height: 42px;
  padding: 7px 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid var(--border);
  background: var(--paper);
}

.format-badge {
  font-size: 12px;
  font-weight: 700;
  border-radius: 999px;
  padding: 3px 8px;
  background: var(--info-soft);
  color: var(--info-text);
}

.direction-field {
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 6px;
}

.page-file-input {
  display: none;
}

.toolbar-spacer {
  flex: 1;
}

.page-error {
  margin: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-error p {
  margin: 0;
}

.page-loading {
  margin: 12px;
}

.page-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px;
}

.page-hint {
  margin: 0 0 10px;
}

.page-grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 12px;
}

.page-cell {
  display: grid;
  gap: 6px;
  padding: 6px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}

.page-cell.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 30%, transparent);
}

.page-cell.drop-target {
  border-style: dashed;
  border-color: var(--accent);
}

.page-cell.missing {
  background: var(--warning-tint);
}

.page-select {
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.page-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.page-number {
  flex: 1;
  font-size: 12px;
  color: var(--muted);
}

.icon-button {
  min-width: 26px;
  height: 26px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
}

.icon-button.danger {
  color: var(--danger-text);
}

.unlisted {
  margin-top: 24px;
}

.unlisted h3 {
  margin: 0 0 4px;
  font-size: 15px;
}
</style>
