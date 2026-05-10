<template>
  <section class="reader-page">
    <div class="reader-shell">
      <header class="reader-toolbar">
        <RouterLink :to="`/books/${id}`" class="reader-back">Back to detail</RouterLink>
        <div class="reader-title">
          <span class="reader-kicker">Reader</span>
          <h2>{{ title || id }}</h2>
        </div>
        <div class="reader-actions">
          <span class="reader-progress">Progress: {{ progress?.percent ?? 0 }}%</span>
          <button class="button" type="button" @click="openSplitModal">Split</button>
          <button class="button reader-bookmark" :disabled="bookmarking" @click="bookmarkCurrent">
            {{ bookmarking ? 'Saving...' : 'Bookmark' }}
          </button>
        </div>
      </header>

      <div v-if="loading" class="loading reader-status">Loading content...</div>
      <div v-else-if="error" class="error reader-status reader-error" role="alert">
        <p>{{ error }}</p>
        <button class="button" type="button" @click="fetchReaderData">Retry</button>
      </div>

      <article v-else class="reader-document">
        <div class="reader-nav" v-if="sections.length > 0">
          <button class="button reader-nav-button" type="button" :disabled="currentSectionIndex <= 0" @click="goPrevSection">
            Prev
          </button>
          <div class="reader-nav-center">
            <strong>{{ currentSectionIndex + 1 }} / {{ sections.length }}</strong>
            <span class="reader-nav-title">{{ currentSection?.title }}</span>
          </div>
          <button
            class="button reader-nav-button"
            type="button"
            :disabled="currentSectionIndex >= sections.length - 1"
            @click="goNextSection"
          >
            Next
          </button>
          <label class="reader-nav-jump" for="reader-section-jump">
            Jump:
            <select
              id="reader-section-jump"
              class="input reader-nav-select"
              :value="String(currentSectionIndex)"
              @change="onSelectSection"
            >
              <option v-for="section in sections" :key="section.index" :value="String(section.index)">
                {{ section.index + 1 }}. {{ section.title }}
              </option>
            </select>
          </label>
        </div>

        <p v-if="splitWarning" class="reader-split-warning" role="status">{{ splitWarning }}</p>

        <div class="reader-content" @scroll="onScroll" ref="readerRef">
          <div class="reader-text">{{ currentSection?.text ?? '' }}</div>
        </div>
      </article>
    </div>

    <div v-if="isSplitModalOpen" class="modal-overlay" role="presentation" @click="closeSplitModal">
      <section class="panel split-modal" role="dialog" aria-modal="true" aria-labelledby="split-modal-title" @click.stop>
        <header class="split-header">
          <h3 id="split-modal-title">Reader Split Settings</h3>
          <button class="icon-close" type="button" aria-label="Close split dialog" :disabled="savingSplit" @click="closeSplitModal">
            ×
          </button>
        </header>

        <p class="split-desc">Apply section splitting without leaving reader. Current reading position will be preserved.</p>

        <div v-if="splitModalError" class="error" role="alert">{{ splitModalError }}</div>

        <form class="split-form" @submit.prevent="onSubmitSplitConfig">
          <label class="field">
            <span class="label">Split Type</span>
            <select v-model="draftType" class="input" :disabled="savingSplit">
              <option value="none">none</option>
              <option value="line_count">line_count</option>
              <option value="regex">regex</option>
              <option value="lines">lines</option>
            </select>
          </label>

          <label v-if="draftType === 'line_count'" class="field">
            <span class="label">line_count</span>
            <input
              v-model="draftLineCount"
              class="input"
              type="number"
              min="1"
              step="1"
              placeholder="e.g. 100"
              :disabled="savingSplit"
            />
          </label>

          <label v-if="draftType === 'regex'" class="field">
            <span class="label">regex</span>
            <textarea
              v-model="draftRegex"
              class="input split-textarea"
              rows="4"
              placeholder="e.g. ^Chapter\\s+\\d+"
              :disabled="savingSplit"
            />
          </label>

          <label v-if="draftType === 'lines'" class="field">
            <span class="label">lines (1-based, comma or space separated)</span>
            <textarea
              v-model="draftLines"
              class="input split-textarea"
              rows="4"
              placeholder="e.g. 1, 101, 201"
              :disabled="savingSplit"
            />
          </label>

          <div class="actions">
            <button class="button" type="button" :disabled="savingSplit" @click="closeSplitModal">Cancel</button>
            <button class="button primary" type="submit" :disabled="savingSplit">
              {{ savingSplit ? 'Saving...' : 'Save Split Config' }}
            </button>
          </div>
        </form>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { useReader } from '../composables/useReader';
import type { SplitConfig, SplitType } from '../types/book';

const route = useRoute();
const id = computed(() => String(route.params.id));
const {
  title,
  sections,
  currentSectionIndex,
  currentSection,
  splitWarning,
  progress,
  loading,
  bookmarking,
  error,
  readerRef,
  fetchReaderData,
  onScroll,
  goPrevSection,
  goNextSection,
  goToSection,
  splitConfig,
  applySplitConfig,
  bookmarkCurrent
} = useReader(() => id.value);

const isSplitModalOpen = ref(false);
const savingSplit = ref(false);
const splitModalError = ref('');
const draftType = ref<SplitType>('none');
const draftLineCount = ref('100');
const draftRegex = ref('');
const draftLines = ref('1');

function hydrateSplitDraft(config: SplitConfig): void {
  draftType.value = config.type;
  draftLineCount.value = String(config.line_count ?? 100);
  draftRegex.value = config.regex ?? '';
  draftLines.value = (config.lines ?? []).join(', ');
}

function openSplitModal(): void {
  hydrateSplitDraft(splitConfig.value);
  splitModalError.value = '';
  isSplitModalOpen.value = true;
}

function closeSplitModal(): void {
  if (savingSplit.value) {
    return;
  }
  isSplitModalOpen.value = false;
}

function onDocumentKeydown(event: KeyboardEvent): void {
  // Handle Escape key to close Split modal
  if (event.key === 'Escape') {
    if (isSplitModalOpen.value) {
      closeSplitModal();
    }
    return;
  }

  // Don't handle other keys if Split modal is open
  if (isSplitModalOpen.value) {
    return;
  }

  // Handle left/right arrow keys to navigate sections
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    // Don't handle if focus is on input/textarea/select/button/contenteditable elements
    const activeElement = document.activeElement;
    if (
      activeElement instanceof HTMLInputElement ||
      activeElement instanceof HTMLTextAreaElement ||
      activeElement instanceof HTMLSelectElement ||
      activeElement instanceof HTMLButtonElement ||
      activeElement?.getAttribute?.('contenteditable') === 'true'
    ) {
      return;
    }

    // Navigate to previous/next section if not at boundary
    if (event.key === 'ArrowLeft') {
      if (currentSectionIndex.value > 0) {
        goPrevSection();
      }
    } else if (event.key === 'ArrowRight') {
      if (currentSectionIndex.value < sections.value.length - 1) {
        goNextSection();
      }
    }
  }
}

function buildDraftSplitConfig(): SplitConfig {
  if (draftType.value === 'line_count') {
    const parsed = Number.parseInt(draftLineCount.value, 10);
    return {
      type: 'line_count',
      line_count: Number.isNaN(parsed) ? 0 : parsed
    };
  }

  if (draftType.value === 'regex') {
    return {
      type: 'regex',
      regex: draftRegex.value
    };
  }

  if (draftType.value === 'lines') {
    const lines = draftLines.value
      .split(/[\s,]+/)
      .map((token) => Number.parseInt(token, 10))
      .filter((num) => !Number.isNaN(num));

    return {
      type: 'lines',
      lines
    };
  }

  return { type: 'none' };
}

async function onSubmitSplitConfig(): Promise<void> {
  savingSplit.value = true;
  splitModalError.value = '';
  try {
    await applySplitConfig(buildDraftSplitConfig());
    isSplitModalOpen.value = false;
  } catch (err) {
    splitModalError.value = err instanceof Error ? err.message : 'Failed to update split config';
  } finally {
    savingSplit.value = false;
  }
}

function onSelectSection(event: Event): void {
  const target = event.target as HTMLSelectElement | null;
  if (!target) {
    return;
  }
  const index = Number.parseInt(target.value, 10);
  if (Number.isNaN(index)) {
    return;
  }
  void goToSection(index);
}

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown);
  void fetchReaderData();
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown);
});
</script>

<style scoped>
.reader-page {
  width: 100%;
  min-height: 100vh;
  padding: 28px 18px 56px;
}

.reader-shell {
  max-width: 860px;
  margin: 0 auto;
}

.reader-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 18px;
  margin-bottom: 18px;
  color: #6b5f4a;
}

.reader-back {
  justify-self: start;
  color: #6b5f4a;
  text-decoration: none;
  font-size: 0.95rem;
  letter-spacing: 0.02em;
}

.reader-back:hover {
  color: #4f4434;
}

.reader-title {
  text-align: center;
  min-width: 0;
}

.reader-kicker {
  display: block;
  margin-bottom: 4px;
  font-size: 0.78rem;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: rgba(107, 95, 74, 0.72);
}

.reader-title h2 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: #3f3529;
  overflow-wrap: anywhere;
}

.reader-actions {
  justify-self: end;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  min-width: 0;
}

.reader-progress {
  color: #6b5f4a;
  font-size: 0.92rem;
  white-space: nowrap;
}

.reader-bookmark {
  border-color: rgba(122, 104, 72, 0.18);
  background: rgba(255, 250, 240, 0.86);
  color: #4b3f2f;
  box-shadow: 0 8px 18px rgba(91, 73, 46, 0.08);
}

.reader-bookmark:hover:not(:disabled) {
  background: #fffdf7;
}

.reader-bookmark:disabled {
  cursor: wait;
  opacity: 0.72;
}

.reader-status {
  margin: 0 auto;
  max-width: 760px;
}

.reader-error {
  display: grid;
  gap: 10px;
}

.reader-error p {
  margin: 0;
}

.reader-error .button {
  justify-self: start;
}

.reader-document {
  background: linear-gradient(180deg, rgba(255, 251, 242, 0.98), rgba(253, 248, 237, 0.94));
  border-radius: 18px;
  box-shadow:
    0 16px 40px rgba(96, 75, 44, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.7);
  padding: 22px;
}

.reader-nav {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px 12px;
  margin-bottom: 12px;
}

.reader-nav-button {
  min-width: 74px;
}

.reader-nav-center {
  display: grid;
  justify-items: center;
  gap: 2px;
  min-width: 0;
}

.reader-nav-center strong {
  color: #3f3529;
  font-size: 0.95rem;
}

.reader-nav-title {
  max-width: 100%;
  color: #6b5f4a;
  font-size: 0.88rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.reader-nav-jump {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 8px;
  color: #6b5f4a;
  font-size: 0.9rem;
}

.reader-nav-select {
  min-width: 240px;
  max-width: 100%;
}

.reader-split-warning {
  margin: 0 0 10px;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(255, 226, 173, 0.33);
  color: #6f4c1f;
  font-size: 0.9rem;
}

.reader-content {
  max-height: 72vh;
  overflow-y: auto;
  padding: 18px 26px;
  scrollbar-gutter: stable both-edges;
}

.reader-text {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  color: #2f2a22;
  font-family: Georgia, 'Times New Roman', 'Noto Serif TC', 'Songti TC', serif;
  font-size: 1.08rem;
  line-height: 1.95;
  letter-spacing: 0.01em;
}

.modal-overlay {
  align-items: center;
  background: rgba(15, 23, 42, 0.38);
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 16px;
  position: fixed;
  z-index: 70;
}

.split-modal {
  display: grid;
  gap: 10px;
  max-height: calc(100vh - 32px);
  overflow: auto;
  padding: 16px;
  width: min(100%, 560px);
}

.split-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.split-header h3 {
  margin: 0;
  color: #3f3529;
}

.icon-close {
  align-items: center;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--muted);
  cursor: pointer;
  display: inline-flex;
  font-size: 20px;
  height: 32px;
  justify-content: center;
  line-height: 1;
  width: 32px;
}

.icon-close:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.split-desc {
  margin: 0;
  color: #6b5f4a;
  font-size: 0.92rem;
}

.split-form {
  display: grid;
  gap: 10px;
}

.split-textarea {
  resize: vertical;
  min-height: 88px;
}

@media (max-width: 720px) {
  .reader-page {
    padding: 18px 12px 36px;
  }

  .reader-toolbar {
    grid-template-columns: 1fr;
    justify-items: start;
    gap: 10px;
    margin-bottom: 14px;
  }

  .reader-title {
    text-align: left;
  }

  .reader-actions {
    justify-self: stretch;
    width: 100%;
    justify-content: space-between;
    flex-wrap: wrap;
  }

  .reader-document {
    border-radius: 14px;
    padding: 12px;
  }

  .reader-nav {
    grid-template-columns: 1fr 1fr;
  }

  .reader-nav-center {
    grid-column: 1 / -1;
    order: -1;
    justify-items: start;
  }

  .reader-nav-jump {
    grid-column: 1 / -1;
  }

  .reader-nav-select {
    min-width: 0;
    width: 100%;
  }

  .reader-content {
    max-height: calc(100vh - 200px);
    padding: 12px 8px 16px;
  }

  .reader-text {
    font-size: 1rem;
    line-height: 1.82;
  }

  .split-modal {
    width: min(100%, 96vw);
    padding: 12px;
  }
}
</style>
