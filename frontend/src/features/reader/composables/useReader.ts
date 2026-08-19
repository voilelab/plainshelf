import { computed, nextTick, ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import { isLibraryEditingSupported } from '@/composables/useWriteAccess';
import { useReadingProgressAutosave } from '@/features/reader/composables/useReadingProgressAutosave';
import { buildMarkdownH2Sections } from '@/features/reader/utils/markdownChapters';
import type { ReaderSection, ReadingProgress } from '@/types/book';
import { t } from '@/i18n';

function clampOffset(offset: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(total, Math.round(offset)));
}

/** The whole source as one section, which is all a TXT source ever has. */
function buildSingleSection(content: string): ReaderSection[] {
  return [
    {
      index: 0,
      startOffset: 0,
      endOffset: content.length,
      title: 'Part 1',
      text: content
    }
  ];
}

function findSectionIndexByOffset(sections: ReaderSection[], offset: number): number {
  if (sections.length === 0) {
    return 0;
  }

  for (let i = 0; i < sections.length; i += 1) {
    const section = sections[i];
    const isLast = i === sections.length - 1;
    if (offset >= section.startOffset && (offset < section.endOffset || (isLast && offset <= section.endOffset))) {
      return i;
    }
  }

  return sections.length - 1;
}

export function useReader(bookID: () => string) {
  const title = ref('');
  const bookFormat = ref('txt');
  // The source the reader is showing. Illustrations are stored per source, so
  // rendering one needs this alongside the book ID.
  const currentSourceId = ref('');
  const content = ref('');
  const sections = ref<ReaderSection[]>([]);
  const currentSectionIndex = ref(0);
  const currentSection = computed<ReaderSection | null>(
    () => sections.value[currentSectionIndex.value] ?? sections.value[0] ?? null
  );
  const progress = ref<ReadingProgress | null>(null);
  const loading = ref(false);
  const error = ref('');
  const currentOffset = ref(0);
  const readerRef = ref<HTMLDivElement | null>(null);
  let fetchGeneration = 0;

  const {
    saveError,
    setBaseline: setProgressBaseline,
    update: updateAutosaveOffset,
    flush: flushReadingProgress,
    start: startProgressAutosave,
    stop: stopProgressAutosave
  } = useReadingProgressAutosave(async (savedBookID, offset) => {
    await getBookshelfProvider().saveReadProgress(savedBookID, { char_offset: offset });
  });

  function normalizeProgress(next: ReadingProgress): ReadingProgress {
    const total = content.value.length;
    const clampedOffset = clampOffset(next.char_offset, total);
    const percent =
      next.percent ??
      (total > 0 ? Math.max(0, Math.min(100, Math.round((clampedOffset / total) * 100))) : 0);

    return {
      ...next,
      char_offset: clampedOffset,
      percent
    };
  }

  function updateProgressByOffset(nextOffset: number): void {
    const total = content.value.length;
    const clamped = clampOffset(nextOffset, total);
    const percent = total > 0 ? Math.max(0, Math.min(100, Math.round((clamped / total) * 100))) : 0;

    currentOffset.value = clamped;
    updateAutosaveOffset(clamped);
    if (progress.value) {
      progress.value = {
        ...progress.value,
        char_offset: clamped,
        percent
      };
      return;
    }

    progress.value = {
      char_offset: clamped,
      percent
    };
  }

  async function syncScrollToOffset(offset: number): Promise<void> {
    await nextTick();

    const section = currentSection.value;
    const el = readerRef.value;
    if (!el || !section) {
      return;
    }

    const maxScrollTop = el.scrollHeight - el.clientHeight;
    if (maxScrollTop <= 0) {
      return;
    }

    const sectionLength = section.endOffset - section.startOffset;
    if (sectionLength <= 0) {
      el.scrollTop = 0;
      return;
    }

    const localOffset = Math.max(0, Math.min(sectionLength, offset - section.startOffset));
    const ratio = localOffset / sectionLength;
    el.scrollTop = Math.round(maxScrollTop * ratio);
  }

  async function syncSectionAndScrollByOffset(offset: number): Promise<void> {
    const safeOffset = clampOffset(offset, content.value.length);
    const sectionIdx = findSectionIndexByOffset(sections.value, safeOffset);
    currentSectionIndex.value = sectionIdx;
    updateProgressByOffset(safeOffset);
    await syncScrollToOffset(safeOffset);
  }

  async function fetchReaderData(): Promise<void> {
    const requestedBookID = bookID();
    const generation = ++fetchGeneration;
    loading.value = true;
    error.value = '';
    let restoredOffset: number | null = null;

    try {
      // A route-param change reuses this component, so persist the previous
      // book before replacing the autosave baseline with the next one.
      await flushReadingProgress();
      if (generation !== fetchGeneration) {
        return;
      }

      const provider = getBookshelfProvider();
      const book = await provider.getBook(requestedBookID);
      if (generation !== fetchGeneration) {
        return;
      }

      // current_source can name a source that no longer exists — a shelf is
      // edited by hand and by sync tools. The server answers the book-scoped
      // content route from the newest surviving source, so falling back to it
      // keeps such a book readable instead of failing the whole reader.
      const sourceID = book.current_source ?? '';
      const bookContent = () => provider.getBookContent(requestedBookID).then((result) => result.content);
      const [sourceContent, currentProgress, sourceMeta] = await Promise.all([
        sourceID
          ? provider.getSourceContent(requestedBookID, sourceID).catch(bookContent)
          : bookContent(),
        provider.getReadProgress(requestedBookID),
        sourceID
          ? provider.getSource(requestedBookID, sourceID).catch(() => undefined)
          : Promise.resolve(undefined)
      ]);
      if (generation !== fetchGeneration) {
        return;
      }

      title.value = book.title ?? (book as { meta?: { title?: string } }).meta?.title ?? requestedBookID;
      currentSourceId.value = sourceID;
      content.value = sourceContent;

      const sourceFormat = sourceMeta?.format === 'md' || sourceMeta?.format === 'txt' ? sourceMeta.format : undefined;
      bookFormat.value = sourceFormat ?? (book.format === 'md' ? 'md' : 'txt');

      sections.value = bookFormat.value === 'md'
        ? buildMarkdownH2Sections(content.value)
        : buildSingleSection(content.value);

      const normalized = normalizeProgress(currentProgress);
      const effectiveOffset = setProgressBaseline(requestedBookID, normalized.char_offset);
      const effectiveProgress = normalizeProgress({ char_offset: effectiveOffset });
      progress.value = effectiveProgress;
      restoredOffset = effectiveProgress.char_offset;

      try {
        await provider.addReadHistory(requestedBookID);
      } catch (err) {
        console.warn('Failed to update read history', err);
      }
    } catch (err) {
      if (generation === fetchGeneration) {
        error.value = err instanceof Error ? err.message : t('reader.errors.loadFailed');
      }
    } finally {
      if (generation === fetchGeneration) {
        loading.value = false;
      }
    }

    if (generation === fetchGeneration && restoredOffset !== null) {
      await syncSectionAndScrollByOffset(restoredOffset);
    }
  }

  function onScroll(): void {
    const el = readerRef.value;
    const section = currentSection.value;
    if (!el || !section) {
      return;
    }

    const max = el.scrollHeight - el.clientHeight;
    const ratio = max > 0 ? el.scrollTop / max : 0;

    const sectionLength = Math.max(0, section.endOffset - section.startOffset);
    const localOffset = Math.round(sectionLength * ratio);
    const nextGlobalOffset = section.startOffset + localOffset;
    updateProgressByOffset(nextGlobalOffset);
  }

  async function goToSection(index: number): Promise<void> {
    if (sections.value.length === 0) {
      return;
    }

    const clampedIndex = Math.max(0, Math.min(sections.value.length - 1, Math.trunc(index)));
    currentSectionIndex.value = clampedIndex;
    const section = sections.value[clampedIndex];
    updateProgressByOffset(section.startOffset);
    await syncScrollToOffset(section.startOffset);
    readerRef.value?.focus();
  }

  async function goPrevSection(): Promise<void> {
    await goToSection(currentSectionIndex.value - 1);
  }

  async function goNextSection(): Promise<void> {
    await goToSection(currentSectionIndex.value + 1);
  }

  async function syncCurrentScroll(): Promise<void> {
    await syncScrollToOffset(currentOffset.value);
  }

  return {
    title,
    bookFormat,
    currentSourceId,
    content,
    sections,
    currentSectionIndex,
    currentSection,
    progress,
    loading,
    error,
    saveError,
    readerRef,
    fetchReaderData,
    onScroll,
    goPrevSection,
    goNextSection,
    goToSection,
    syncCurrentScroll,
    flushReadingProgress,
    startProgressAutosave,
    stopProgressAutosave
  };
}
