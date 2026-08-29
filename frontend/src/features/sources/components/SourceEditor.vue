<template>
  <section class="editor-panel">
    <div class="source-editor-status" role="status">
      <p v-if="loading" class="meta">{{ t('sources.editor.loading') }}</p>
      <p v-else-if="!sourceId" class="meta">{{ t('sources.editor.noSelection') }}</p>
      <p v-else-if="dirty" class="meta dirty">{{ t('sources.editor.dirty') }}</p>
      <p v-else class="meta">{{ t('sources.editor.clean') }}</p>
      <span v-if="sourceId && !loading" class="status-spacer"></span>
      <span v-if="sourceId && !loading && isCurrent" class="current-badge">{{ t('sources.list.current') }}</span>
      <button
        v-if="sourceId && !loading && !isCurrent"
        class="button set-current-btn"
        type="button"
        :disabled="settingCurrent"
        @click="$emit('setCurrent')"
      >
        {{ settingCurrent ? t('sources.editor.settingCurrent') : t('sources.editor.setCurrent') }}
      </button>
    </div>

    <div class="editor-find-replace" role="group" :aria-label="t('sources.editor.find.groupLabel')">
      <label class="control-field">
        <span class="field-label">{{ t('sources.editor.find.findLabel') }}</span>
        <input
          v-model="findQuery"
          class="control-input"
          type="text"
          :placeholder="t('sources.editor.find.findPlaceholder')"
          :disabled="isEditorDisabled"
          @keydown.enter.prevent="findNext"
        />
      </label>

      <label class="control-field">
        <span class="field-label">{{ t('sources.editor.find.replaceLabel') }}</span>
        <input
          v-model="replaceQuery"
          class="control-input"
          type="text"
          :placeholder="t('sources.editor.find.replacePlaceholder')"
          :disabled="isEditorDisabled"
          @keydown="onReplaceInputKeydown"
        />
      </label>

      <label v-if="focused" class="control-field scope-field">
        <span class="field-label">{{ t('sources.editor.find.scopeLabel') }}</span>
        <SelectRoot :model-value="findScope" :disabled="isEditorDisabled" @update:model-value="onFindScopeChange">
          <SelectTrigger class="control-input select-trigger">
            <SelectValue />
          </SelectTrigger>
          <SelectPortal>
            <SelectContent class="reka-menu" position="popper" align="start" :side-offset="6">
              <SelectViewport>
                <SelectItem class="reka-menu-item" value="section">
                  <SelectItemText>{{ t('sources.editor.find.scopeSection') }}</SelectItemText>
                </SelectItem>
                <SelectItem class="reka-menu-item" value="source">
                  <SelectItemText>{{ t('sources.editor.find.scopeSource') }}</SelectItemText>
                </SelectItem>
              </SelectViewport>
            </SelectContent>
          </SelectPortal>
        </SelectRoot>
      </label>

      <div class="find-actions">
        <button class="button" type="button" :disabled="disableFind" @click="findPrevious">{{ t('sources.editor.find.previous') }}</button>
        <button class="button" type="button" :disabled="disableFind" @click="findNext">{{ t('sources.editor.find.next') }}</button>
        <button class="button" type="button" :disabled="disableFind" @click="replaceNext">
          {{ t('sources.editor.find.replace') }}
        </button>
        <button class="button" type="button" :disabled="disableFind" @click="replaceAll">
          {{ t('sources.editor.find.replaceAll') }}
        </button>
      </div>

      <div class="find-options">
        <label class="find-option">
          <input v-model="caseSensitive" type="checkbox" :disabled="isEditorDisabled" />
          <span>{{ t('sources.editor.find.caseSensitive') }}</span>
        </label>
        <label class="find-option">
          <input v-model="wholeWord" type="checkbox" :disabled="isEditorDisabled" />
          <span>{{ t('sources.editor.find.wholeWord') }}</span>
        </label>
        <label class="find-option">
          <input v-model="useRegexp" type="checkbox" :disabled="isEditorDisabled" />
          <span>{{ t('sources.editor.find.regexp') }}</span>
        </label>
      </div>

      <p class="find-status" role="status" aria-live="polite">{{ findStatus }}</p>
    </div>

    <div v-if="error" class="error editor-error" role="alert">{{ error }}</div>

    <div ref="hostRef" class="source-content-editor"></div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
  type AcceptableValue
} from 'reka-ui';
import { useSourceCodeMirror } from '@/features/sources/composables/useSourceCodeMirror';
import type {
  SourceDocumentEdit,
  SourceEditorHandle,
  SourceEditorViewRange,
  SourceFindScope
} from '@/features/sources/types/editorAdapter';
import { useI18n } from '@/i18n';

const { t } = useI18n();

const props = defineProps<{
  modelValue: string;
  sourceId: string;
  loading?: boolean;
  saving?: boolean;
  dirty?: boolean;
  error?: string;
  isCurrent?: boolean;
  settingCurrent?: boolean;
  viewRange?: SourceEditorViewRange | null;
  focused?: boolean;
  format?: 'txt' | 'md';
}>();

const emit = defineEmits<{
  documentEdit: [edit: SourceDocumentEdit];
  setCurrent: [];
}>();

const isEditorDisabled = computed(() => !props.sourceId || props.loading || props.saving);
const findScope = ref<SourceFindScope>(props.focused ? 'section' : 'source');

function onFindScopeChange(value: AcceptableValue): void {
  if (value === 'section' || value === 'source') {
    findScope.value = value;
  }
}

watch(
  () => props.focused,
  (focused, wasFocused) => {
    if (!focused) findScope.value = 'source';
    else if (!wasFocused) findScope.value = 'section';
  }
);

const {
  hostRef,
  findQuery,
  replaceQuery,
  caseSensitive,
  useRegexp,
  wholeWord,
  findStatus,
  disableFind,
  findNext,
  findPrevious,
  replaceNext,
  onReplaceInputKeydown,
  replaceAll,
  flushDocument,
  getCurrentParagraphStart,
  replaceRange,
  replaceRangeQuietly,
  jumpToOffset,
  focusAndSelect
} = useSourceCodeMirror({
  content: () => props.modelValue,
  sourceId: () => props.sourceId,
  disabled: () => isEditorDisabled.value,
  viewRange: () => props.viewRange ?? null,
  findScope: () => findScope.value,
  format: () => props.format ?? 'txt',
  contentLabel: () => t('sources.editor.contentLabel'),
  updateDocument: (edit) => emit('documentEdit', edit)
});

defineExpose<SourceEditorHandle>({
  getCurrentParagraphStart,
  replaceRange,
  replaceRangeQuietly,
  jumpToOffset,
  focusAndSelect,
  flushDocument
});
</script>

<style scoped>
.editor-panel {
  flex: 1;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.source-editor-status {
  flex-shrink: 0;
  min-height: 38px;
  display: flex;
  align-items: center;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  background: #f8fafc;
}

.source-editor-status p {
  margin: 0;
}

.source-editor-status .dirty {
  color: #9a3412;
}

.status-spacer {
  flex: 1;
}

.current-badge {
  font-size: 12px;
  font-weight: 600;
  color: #166534;
  background: #dcfce7;
  border: 1px solid #bbf7d0;
  border-radius: 10px;
  padding: 2px 8px;
}

.set-current-btn {
  font-size: 12px;
  padding: 3px 10px;
}

/*
 * The host box only owns the layout; everything inside it is CodeMirror's own
 * DOM and is styled through EditorView.theme, which reaches past this scoped
 * stylesheet's attribute selector.
 */
.source-content-editor {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  width: 100%;
  box-sizing: border-box;
  overflow: hidden;
  background: #fff;
}

.editor-find-replace {
  flex-shrink: 0;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(140px, auto) auto;
  gap: 8px 10px;
  align-items: end;
  background: #f8fafc;
}

.scope-field {
  min-width: 140px;
}

.control-field {
  display: grid;
  gap: 4px;
}

.field-label {
  font-size: 12px;
  color: var(--muted);
}

.control-input {
  height: 34px;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0 10px;
  font: inherit;
}

/* The Reka SelectTrigger is a <button>; give it the white field surface and
   left alignment the native <select> had while keeping .control-input sizing. */
.select-trigger {
  align-items: center;
  background: var(--bg, #fff);
  color: inherit;
  cursor: pointer;
  display: inline-flex;
  text-align: left;
  width: 100%;
}

.find-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.find-options {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 14px;
}

.find-option {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--muted);
}

.find-status {
  color: var(--muted);
  font-size: 12px;
  grid-column: 1 / -1;
  margin: 0;
  min-height: 1.2em;
}

.editor-error {
  flex-shrink: 0;
  margin: 10px 12px;
}

@media (max-width: 900px) {
  .editor-find-replace {
    grid-template-columns: 1fr;
  }

  .find-actions {
    flex-wrap: wrap;
  }
}
</style>
