import { t } from '@/i18n';
import { computed, ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import { basenameFromPath, deriveTitleFromFilename, hasSupportedExtension } from '@/utils/file';
import { normalizeFolderPath } from '@/utils/folders';
import {
  DEFAULT_CHAPTER_PATTERN,
  countTextChapters,
  textToMarkdownByRegex
} from '@/features/sources/utils/sourceConversions';
import { DEFAULT_EPUB_IMPORT_STRATEGY, type EpubImportStrategy } from '@/types/book';

const bookExtPattern = /\.(txt|md|epub)$/i;
const epubExtPattern = /\.epub$/i;
const txtExtPattern = /\.txt$/i;

// 'cancelled' is deliberately distinct from 'failed': a book left untouched by an
// abort must never read as an import error. It mirrors taskutil's
// partially_completed, so a future server-side taskchain keeps the same mental
// model.
type ImportStatus = 'pending' | 'importing' | 'success' | 'failed' | 'cancelled';

interface ImportFileState {
  filename: string;
  title: string;
  status: ImportStatus;
  error: string;
  createdId?: string;
}

// A single thing to import, independent of where its bytes come from. The upload
// path adds the File payload; a host-path source (desktop) can add its own.
export interface ImportUnit {
  filename: string;
  title: string;
}

interface FileImportUnit extends ImportUnit {
  file: File;
}

// The desktop equivalent of FileImportUnit: the bytes stay on disk and are
// named by a host path the Wails binding imports, rather than an in-memory File.
interface LocalPathImportUnit extends ImportUnit {
  localPath: string;
}

// Turns one unit into a created book id, or throws to fail that unit. Abort is
// cooperative and only checked between units, so an executor may ignore the
// signal; it is passed so an implementation that can bail out early may.
type ImportExecutor<T extends ImportUnit = ImportUnit> = (
  unit: T,
  signal: AbortSignal
) => Promise<string>;

export interface ImportSubmitResult {
  total: number;
  successCount: number;
  failedCount: number;
  cancelledCount: number;
  cancelled: boolean;
  firstImportedId?: string;
}

interface ImportProgress {
  total: number;
  completed: number;
  // 1-based ordinal of the book currently importing; falls back to the completed
  // count when nothing is in flight.
  current: number;
  filename: string;
  percentage: number;
}

// A just-imported TXT whose text reads as chaptered, offered to the user as a
// one-click conversion. `content` is kept so accepting the offer does not fetch
// the source a second time.
interface ChapterConversionSuggestion {
  bookId: string;
  chapters: number;
  content: string;
}

export function useImportBook() {
  const bookFiles = ref<File[]>([]);
  const files = ref<ImportFileState[]>([]);
  const submitting = ref(false);
  const cancelRequested = ref(false);
  const success = ref('');
  const error = ref('');
  const epubStrategy = ref<EpubImportStrategy>({ ...DEFAULT_EPUB_IMPORT_STRATEGY });
  const chapterSuggestion = ref<ChapterConversionSuggestion | null>(null);
  const convertingChapters = ref(false);
  const chapterConversionError = ref('');

  let controller: AbortController | null = null;
  // Bumped whenever the suggestion is cleared (a new import, a reset, or a modal
  // close), so an in-flight chapter probe knows it has been superseded and must
  // not repopulate the prompt for a book the user has moved on from.
  let detectionGeneration = 0;

  const progress = computed<ImportProgress>(() => {
    const total = files.value.length;
    let completed = 0;
    let activeIndex = -1;
    files.value.forEach((item, index) => {
      if (item.status === 'importing') {
        activeIndex = index;
      } else if (item.status !== 'pending') {
        completed += 1;
      }
    });
    const active = activeIndex >= 0 ? files.value[activeIndex] : undefined;
    return {
      total,
      completed,
      current: active ? activeIndex + 1 : completed,
      filename: active?.filename ?? '',
      percentage: total > 0 ? Math.round((completed / total) * 100) : 0
    };
  });

  function setEpubStrategy(next: EpubImportStrategy): void {
    epubStrategy.value = { ...next };
  }

  function hasEpubFile(): boolean {
    return bookFiles.value.some((file) => epubExtPattern.test(file.name));
  }

  function toImportFileStates(units: ImportUnit[]): ImportFileState[] {
    return units.map((unit) => ({
      filename: unit.filename,
      title: unit.title,
      status: 'pending',
      error: ''
    }));
  }

  function reset(): void {
    controller?.abort();
    controller = null;
    clearImportSelection();
    success.value = '';
    error.value = '';
    clearChapterSuggestion();
  }

  // Clears the retained import payload — the chosen files and their per-file
  // states — without touching the result message. Accepting a detected-chapter
  // prompt uses this so the now-empty file chooser cannot silently reimport the
  // previous File while the success line stays visible.
  function clearImportSelection(): void {
    bookFiles.value = [];
    files.value = [];
    submitting.value = false;
    cancelRequested.value = false;
  }

  function setBookFiles(nextFiles: File[]): void {
    bookFiles.value = nextFiles;
    files.value = toImportFileStates(buildFileUnits(nextFiles));
    success.value = '';
    error.value = '';
    clearChapterSuggestion();
  }

  function clearChapterSuggestion(): void {
    // Invalidate any detection probe still awaiting content for a prior import.
    detectionGeneration += 1;
    chapterSuggestion.value = null;
    convertingChapters.value = false;
    chapterConversionError.value = '';
  }

  // Best-effort: after a single TXT imports cleanly, look at its text and, when
  // it reads as chaptered, stage a one-click conversion. Anything else — a batch,
  // a non-TXT file, a failed fetch, or text with no chapter lines — leaves no
  // suggestion, so the import flow the user sees is unchanged.
  async function detectChapterConversion(result: ImportSubmitResult | null): Promise<void> {
    clearChapterSuggestion();
    // Captured after the clear above bumped it, so any later clear (a reset, a
    // modal close, a fresh import) makes this probe's own result stale.
    const generation = detectionGeneration;
    if (!result || result.total !== 1 || result.successCount !== 1 || !result.firstImportedId) {
      return;
    }
    const imported = files.value.find((item) => item.createdId === result.firstImportedId);
    if (!imported || !txtExtPattern.test(imported.filename)) {
      return;
    }

    try {
      const { content } = await getBookshelfProvider().getBookContent(result.firstImportedId);
      if (generation !== detectionGeneration) {
        // The modal was reset/closed (or another import started) while the
        // content was in flight; do not resurrect a prompt for this book.
        return;
      }
      const chapters = countTextChapters(content);
      if (chapters > 0) {
        chapterSuggestion.value = { bookId: result.firstImportedId, chapters, content };
      }
    } catch {
      // Detection is best-effort; a content-fetch failure just means no prompt.
    }
  }

  // Accepts the staged suggestion: convert the detected TXT to Markdown, create
  // it as a new source, and set it current — the original TXT source is kept so
  // the user can still switch back to it.
  async function applyChapterConversion(): Promise<boolean> {
    const suggestion = chapterSuggestion.value;
    if (!suggestion || convertingChapters.value) {
      return false;
    }
    convertingChapters.value = true;
    chapterConversionError.value = '';

    try {
      const converted = textToMarkdownByRegex(suggestion.content, DEFAULT_CHAPTER_PATTERN);
      if (converted.chapters === 0) {
        chapterSuggestion.value = null;
        return false;
      }
      await bookshelfWriter().createSource(suggestion.bookId, {
        content: converted.content,
        format: 'md',
        comment: `Chapter conversion detected on import (${converted.chapters} chapters)`,
        setCurrent: true
      });
      chapterSuggestion.value = null;
      success.value = t('libraryForms.importBook.chapterSuggestion.done', { count: converted.chapters });
      return true;
    } catch (err) {
      chapterConversionError.value = err instanceof Error && err.message.trim().length > 0
        ? err.message
        : t('libraryForms.importBook.chapterSuggestion.failed');
      return false;
    } finally {
      convertingChapters.value = false;
    }
  }

  function dismissChapterSuggestion(): void {
    clearChapterSuggestion();
  }

  function getSafeErrorMessage(err: unknown): string {
    if (err instanceof Error && err.message.trim().length > 0) {
      return err.message;
    }
    return 'Import failed';
  }

  function normalizeImportFolderPath(currentFolderPath?: string): string {
    const normalized = normalizeFolderPath(currentFolderPath ?? '');
    return normalized.length > 0 ? normalized : '/';
  }

  function buildFileUnits(nextFiles: File[]): FileImportUnit[] {
    return nextFiles.map((file) => ({
      file,
      filename: file.name,
      title: deriveTitleFromFilename(file.name)
    }));
  }

  function createFileExecutor(currentFolderPath?: string): ImportExecutor<FileImportUnit> {
    const folder = normalizeImportFolderPath(currentFolderPath);
    return async (unit) => {
      if (!hasSupportedExtension(unit.filename, bookExtPattern)) {
        throw new Error(t('libraryForms.importBook.errors.unsupportedExtension'));
      }

      const isEpub = epubExtPattern.test(unit.filename);
      const created = await bookshelfWriter().importBook({
        // An EPUB knows its own title; sending the one derived from the
        // filename would only override it with something worse.
        title: isEpub ? '' : unit.title,
        folder,
        file: unit.file,
        strategy: isEpub ? epubStrategy.value : undefined
      });
      return created.id;
    };
  }

  // Desktop host paths become units the same way files do. The blank-path filter
  // that normalizeSelectedLocalPaths applies on the Go picker side is kept here
  // too, so an empty or whitespace path never reaches the per-file binding.
  function buildLocalPathUnits(localPaths: string[]): LocalPathImportUnit[] {
    return localPaths
      .map((localPath) => localPath.trim())
      .filter((localPath) => localPath.length > 0)
      .map((localPath) => {
        const filename = basenameFromPath(localPath);
        return { localPath, filename, title: deriveTitleFromFilename(filename) };
      });
  }

  // The desktop executor: one Wails call per file. Unlike the upload path it
  // sends no per-batch EPUB strategy — the desktop import keeps the configured
  // default the server applies. The single call cannot be interrupted mid-flight,
  // so it ignores the signal; abort is enforced by the shared loop at the next
  // file boundary. A per-file failure surfaces as the result's Error field, which
  // becomes a thrown rejection here so the shared loop marks that unit failed.
  function createLocalPathExecutor(currentFolderPath?: string): ImportExecutor<LocalPathImportUnit> {
    const folder = normalizeImportFolderPath(currentFolderPath);
    return async (unit) => {
      const result = (await bookshelfWriter().importBookFromLocalPath?.(unit.localPath, folder)) ?? null;
      if (result?.error) {
        throw new Error(result.error);
      }
      if (!result?.id) {
        throw new Error('Import failed');
      }
      return result.id;
    };
  }

  function applyResultMessages(result: ImportSubmitResult): void {
    const { total, successCount, failedCount, cancelled } = result;

    if (cancelled) {
      // Reuse the partial-success wording; the untouched remainder are cancelled,
      // not failed, so no "someFailed" banner unless a real failure occurred.
      success.value = successCount > 0
        ? t('libraryForms.importBook.results.partial', { count: successCount, total })
        : '';
      if (failedCount > 0) {
        error.value = t('libraryForms.importBook.errors.someFailed', { count: failedCount });
      } else if (successCount === 0) {
        error.value = t('libraryForms.importBook.errors.cancelled');
      } else {
        error.value = '';
      }
      return;
    }

    if (successCount === total) {
      success.value = total === 1
        ? t('libraryForms.importBook.results.one')
        : t('libraryForms.importBook.results.many', { count: successCount });
      error.value = '';
    } else if (successCount > 0) {
      success.value = t('libraryForms.importBook.results.partial', { count: successCount, total });
      error.value = t('libraryForms.importBook.errors.someFailed', { count: failedCount });
    } else {
      success.value = '';
      error.value = t('libraryForms.importBook.errors.allFailed');
    }
  }

  // The source-agnostic core: it owns the progress accounting and the abort
  // boundary, and knows nothing about File. Each unit carries its own executor
  // via the passed-in strategy, so the desktop shell can reuse this loop.
  async function submit<T extends ImportUnit>(
    units: T[],
    executor: ImportExecutor<T>
  ): Promise<ImportSubmitResult | null> {
    if (submitting.value) {
      return null;
    }

    error.value = '';
    success.value = '';

    if (units.length === 0) {
      error.value = t('libraryForms.importBook.errors.noFiles');
      return null;
    }

    files.value = toImportFileStates(units);
    controller = new AbortController();
    const signal = controller.signal;
    cancelRequested.value = false;
    submitting.value = true;

    let successCount = 0;
    let failedCount = 0;
    let cancelledCount = 0;
    let firstImportedId: string | undefined;

    try {
      for (let index = 0; index < units.length; index += 1) {
        if (signal.aborted) {
          // Abort stops at a file boundary: send no further requests and mark the
          // untouched remainder cancelled rather than failed.
          for (let rest = index; rest < files.value.length; rest += 1) {
            files.value[rest] = { ...files.value[rest], status: 'cancelled', error: '' };
            cancelledCount += 1;
          }
          break;
        }

        files.value[index] = { ...files.value[index], status: 'importing', error: '' };

        try {
          const id = await executor(units[index], signal);
          files.value[index] = {
            ...files.value[index],
            status: 'success',
            error: '',
            createdId: id
          };
          successCount += 1;
          if (!firstImportedId) {
            firstImportedId = id;
          }
        } catch (err) {
          if (signal.aborted) {
            // The executor honoured the abort signal and bailed out of its
            // in-flight work. That is this unit being cancelled, not a real
            // import failure, so it must not count toward "someFailed".
            files.value[index] = { ...files.value[index], status: 'cancelled', error: '' };
            cancelledCount += 1;
          } else {
            files.value[index] = {
              ...files.value[index],
              status: 'failed',
              error: getSafeErrorMessage(err)
            };
            failedCount += 1;
          }
        }
      }

      const result: ImportSubmitResult = {
        total: files.value.length,
        successCount,
        failedCount,
        cancelledCount,
        cancelled: cancelledCount > 0,
        firstImportedId
      };
      applyResultMessages(result);
      return result;
    } finally {
      submitting.value = false;
      cancelRequested.value = false;
      controller = null;
    }
  }

  // The File-upload entry point the modal uses. It is one executor over the
  // shared core, not a parallel implementation.
  async function submitFiles(currentFolderPath?: string): Promise<ImportSubmitResult | null> {
    return submit(buildFileUnits(bookFiles.value), createFileExecutor(currentFolderPath));
  }

  // The desktop host-path entry point. Like submitFiles it is one executor over
  // the shared core, so the desktop import shows the same N/M progress and abort
  // as the upload path rather than a second implementation.
  async function submitLocalPaths(
    localPaths: string[],
    currentFolderPath?: string
  ): Promise<ImportSubmitResult | null> {
    return submit(buildLocalPathUnits(localPaths), createLocalPathExecutor(currentFolderPath));
  }

  // Cooperative abort: the in-flight file finishes, then the loop stops at the
  // next boundary.
  function abort(): void {
    if (!submitting.value) {
      return;
    }
    cancelRequested.value = true;
    controller?.abort();
  }

  return {
    bookFiles,
    files,
    submitting,
    cancelRequested,
    success,
    error,
    epubStrategy,
    progress,
    chapterSuggestion,
    convertingChapters,
    chapterConversionError,
    setEpubStrategy,
    hasEpubFile,
    setBookFiles,
    submit,
    submitFiles,
    submitLocalPaths,
    detectChapterConversion,
    applyChapterConversion,
    dismissChapterSuggestion,
    clearImportSelection,
    abort,
    reset
  };
}
