import { computed, nextTick, ref } from 'vue';
import { getBook, getBookContent, getBookSplitConfig, getReadingProgress, saveBookmark, updateBookSplitConfig } from '../api/books';
import type { ReaderSection, ReadingProgress, SplitConfig } from '../types/book';

function clampOffset(offset: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(total, Math.round(offset)));
}

function buildLineStartOffsets(content: string): number[] {
  const starts = [0];
  for (let i = 0; i < content.length; i += 1) {
    if (content[i] === '\n') {
      starts.push(i + 1);
    }
  }
  return starts;
}

function buildSectionsFromBoundaries(content: string, starts: number[], isRegexSplit: boolean): ReaderSection[] {
  const total = content.length;
  const uniqueSorted = Array.from(
    new Set(
      starts
        .map((offset) => clampOffset(offset, total))
        .filter((offset) => offset >= 0 && offset <= total)
    )
  ).sort((a, b) => a - b);

  if (uniqueSorted[0] !== 0) {
    uniqueSorted.unshift(0);
  }
  if (uniqueSorted[uniqueSorted.length - 1] !== total) {
    uniqueSorted.push(total);
  }

  const sections: ReaderSection[] = [];
  for (let i = 0; i < uniqueSorted.length - 1; i += 1) {
    const startOffset = uniqueSorted[i];
    const endOffset = uniqueSorted[i + 1];
    if (endOffset <= startOffset) {
      continue;
    }

    const text = content.slice(startOffset, endOffset);
    const firstLine = text.split(/\r?\n/, 1)[0]?.trim() ?? '';

    sections.push({
      index: i,
      startOffset,
      endOffset,
      title: isRegexSplit && firstLine.length > 0 ? firstLine : `Part ${i + 1}`,
      text
    });
  }

  if (sections.length > 0) {
    return sections;
  }

  return [
    {
      index: 0,
      startOffset: 0,
      endOffset: total,
      title: 'Part 1',
      text: content
    }
  ];
}

function buildReaderSectionsWithWarning(content: string, splitConfig: SplitConfig): { sections: ReaderSection[]; warning?: string } {
  const safeConfig = splitConfig ?? { type: 'none' };

  if (safeConfig.type === 'none') {
    return {
      sections: buildSectionsFromBoundaries(content, [0], false)
    };
  }

  if (safeConfig.type === 'line_count') {
    const lineCount = Math.trunc(safeConfig.line_count ?? 0);
    if (lineCount <= 0) {
      return {
        sections: buildSectionsFromBoundaries(content, [0], false)
      };
    }

    const lineStarts = buildLineStartOffsets(content);
    const boundaries: number[] = [];
    for (let i = 0; i < lineStarts.length; i += lineCount) {
      boundaries.push(lineStarts[i]);
    }

    return {
      sections: buildSectionsFromBoundaries(content, boundaries.length > 0 ? boundaries : [0], false)
    };
  }

  if (safeConfig.type === 'lines') {
    const lineStarts = buildLineStartOffsets(content);
    const boundaries = (safeConfig.lines ?? [])
      .map((lineNumber) => Math.trunc(lineNumber))
      .filter((lineNumber) => lineNumber >= 1 && lineNumber <= lineStarts.length)
      .map((lineNumber) => lineStarts[lineNumber - 1]);

    return {
      sections: buildSectionsFromBoundaries(content, boundaries.length > 0 ? boundaries : [0], false)
    };
  }

  if (safeConfig.type === 'regex') {
    const pattern = safeConfig.regex ?? '';
    if (!pattern.trim()) {
      return {
        sections: buildSectionsFromBoundaries(content, [0], false)
      };
    }

    try {
      const regex = new RegExp(pattern, 'gm');
      const boundaries: number[] = [0];
      let matched = false;

      for (const match of content.matchAll(regex)) {
        const start = match.index ?? 0;
        if ((match[0] ?? '').length === 0) {
          continue;
        }
        boundaries.push(start);
        matched = true;
      }

      if (!matched) {
        return {
          sections: buildSectionsFromBoundaries(content, [0], false)
        };
      }

      return {
        sections: buildSectionsFromBoundaries(content, boundaries, true)
      };
    } catch {
      return {
        sections: buildSectionsFromBoundaries(content, [0], false),
        warning: 'Split config regex is invalid. Reader is using a single section.'
      };
    }
  }

  return {
    sections: buildSectionsFromBoundaries(content, [0], false)
  };
}

function buildReaderSections(content: string, splitConfig: SplitConfig): ReaderSection[] {
  return buildReaderSectionsWithWarning(content, splitConfig).sections;
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

function normalizeSplitConfigInput(config: SplitConfig): SplitConfig {
  if (config.type === 'line_count') {
    return {
      type: 'line_count',
      line_count: Math.trunc(config.line_count ?? 0)
    };
  }

  if (config.type === 'regex') {
    return {
      type: 'regex',
      regex: String(config.regex ?? '')
    };
  }

  if (config.type === 'lines') {
    return {
      type: 'lines',
      lines: (config.lines ?? [])
        .filter((line) => Number.isFinite(line))
        .map((line) => Math.trunc(line))
    };
  }

  return { type: 'none' };
}

export function useReader(bookID: () => string) {
  const title = ref('');
  const content = ref('');
  const splitConfig = ref<SplitConfig>({ type: 'none' });
  const splitWarning = ref('');
  const sections = ref<ReaderSection[]>([]);
  const currentSectionIndex = ref(0);
  const currentSection = computed<ReaderSection | null>(
    () => sections.value[currentSectionIndex.value] ?? sections.value[0] ?? null
  );
  const progress = ref<ReadingProgress | null>(null);
  const loading = ref(false);
  const bookmarking = ref(false);
  const error = ref('');
  const currentOffset = ref(0);
  const readerRef = ref<HTMLDivElement | null>(null);

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
    loading.value = true;
    error.value = '';
    splitWarning.value = '';
    let restoredOffset: number | null = null;

    try {
      const [book, bookContent, currentProgress, loadedSplitConfig] = await Promise.all([
        getBook(bookID()),
        getBookContent(bookID()),
        getReadingProgress(bookID()),
        getBookSplitConfig(bookID()).catch((err: unknown) => {
          const reason = err instanceof Error ? err.message : 'Unknown error';
          splitWarning.value = `Failed to load split config, fallback to single section. ${reason}`;
          return { type: 'none' } as SplitConfig;
        })
      ]);

      title.value = book.title ?? (book as { meta?: { title?: string } }).meta?.title ?? bookID();
      content.value = bookContent.content;
      splitConfig.value = loadedSplitConfig;

      const built = buildReaderSectionsWithWarning(content.value, splitConfig.value);
      sections.value = built.sections;
      if (built.warning) {
        splitWarning.value = splitWarning.value ? `${splitWarning.value} ${built.warning}` : built.warning;
      }

      const normalized = normalizeProgress(currentProgress);
      progress.value = normalized;
      restoredOffset = normalized.char_offset;
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load reader data';
    } finally {
      loading.value = false;
    }

    if (restoredOffset !== null) {
      await syncSectionAndScrollByOffset(restoredOffset);
    }
  }

  async function applySplitConfig(config: SplitConfig): Promise<void> {
    const normalizedInput = normalizeSplitConfigInput(config);
    await updateBookSplitConfig(bookID(), normalizedInput);

    splitConfig.value = normalizedInput;
    splitWarning.value = '';

    const built = buildReaderSectionsWithWarning(content.value, splitConfig.value);
    sections.value = built.sections;
    if (built.warning) {
      splitWarning.value = built.warning;
    }

    await syncSectionAndScrollByOffset(currentOffset.value);
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
  }

  async function goPrevSection(): Promise<void> {
    await goToSection(currentSectionIndex.value - 1);
  }

  async function goNextSection(): Promise<void> {
    await goToSection(currentSectionIndex.value + 1);
  }

  async function bookmarkCurrent(): Promise<void> {
    bookmarking.value = true;
    error.value = '';
    try {
      await saveBookmark(bookID(), { char_offset: currentOffset.value });
      const nextProgress = await getReadingProgress(bookID());
      progress.value = normalizeProgress(nextProgress);
      await syncSectionAndScrollByOffset(progress.value.char_offset);
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to create bookmark';
    } finally {
      bookmarking.value = false;
    }
  }

  return {
    title,
    content,
    splitConfig,
    splitWarning,
    sections,
    currentSectionIndex,
    currentSection,
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
    applySplitConfig,
    bookmarkCurrent
  };
}
