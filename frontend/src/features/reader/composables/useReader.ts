import { computed, nextTick, ref } from 'vue';
import { stageDesktopReadingProgress } from '@/api/desktop';
import { getBookshelfProvider } from '@/providers';
import { useReadingProgressAutosave } from '@/features/reader/composables/useReadingProgressAutosave';
import { resolveReadingFormat } from '@/utils/bookFormat';
import { buildMarkdownH2Sections } from '@/utils/markdownChapters';
import type { SourcePage } from '@/types/source';
import { comicReadingPercent } from '@/utils/comicProgress';
import type { ReaderSection, ReadingProgress } from '@/types/book';
import { t } from '@/i18n';

function clampOffset(offset: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(total, Math.round(offset)));
}

function buildSingleSection(content: string): ReaderSection[] {
  return [
    {
      index: 0,
      startOffset: 0,
      endOffset: content.length,
      title: t('reader.sections.singleSectionTitle'),
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
  const currentSourceId = ref('');
  const content = ref('');
  const pages = ref<SourcePage[]>([]);
  const pageDirection = ref<'ltr' | 'rtl'>('ltr');
  const totalOffset = computed(() => bookFormat.value === 'images' ? Math.max(0, pages.value.length - 1) : content.value.length);
  const percentAt = (offset: number) => bookFormat.value === 'images'
    ? comicReadingPercent(offset, pages.value.length)
    : (totalOffset.value > 0 ? Math.max(0, Math.min(100, Math.round(offset / totalOffset.value * 100))) : 0);
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
  } = useReadingProgressAutosave(
    async (savedBookID, offset, at) => {
      await getBookshelfProvider().saveReadProgress(savedBookID, { char_offset: offset, at });
    },
    (stagedBookID, offset, at) => stageDesktopReadingProgress(stagedBookID, offset, at)
  );

  function normalizeProgress(next: ReadingProgress): ReadingProgress {
    const total = totalOffset.value;
    const clampedOffset = clampOffset(next.char_offset, total);
    const percent = percentAt(clampedOffset);

    return {
      ...next,
      char_offset: clampedOffset,
      percent
    };
  }

  function updateProgressByOffset(nextOffset: number): void {
    const total = totalOffset.value;
    const clamped = clampOffset(nextOffset, total);
    const percent = percentAt(clamped);

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
    if (bookFormat.value === 'images') return;
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
    const safeOffset = clampOffset(offset, totalOffset.value);
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
      await flushReadingProgress();
      if (generation !== fetchGeneration) {
        return;
      }

      const provider = getBookshelfProvider();
      const book = await provider.getBook(requestedBookID);
      if (generation !== fetchGeneration) {
        return;
      }

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

      bookFormat.value = resolveReadingFormat(sourceMeta?.format, book.format);

      pageDirection.value = sourceMeta?.page_direction === 'rtl' ? 'rtl' : 'ltr';
      const pageList = bookFormat.value === 'images'
        ? await provider.getSourcePages(requestedBookID, sourceID)
        : null;
      if (generation !== fetchGeneration) return;
      pages.value = pageList?.pages ?? [];

      sections.value = bookFormat.value === 'images'
        ? pages.value.map((_, index) => ({
            index, startOffset: index, endOffset: index + 1,
            title: t('reader.comic.page', { page: index + 1 }), text: ''
          }))
        : bookFormat.value === 'md'
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
    if (bookFormat.value === 'images') return;
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
    void flushReadingProgress();
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
    pages,
    pageDirection,
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
