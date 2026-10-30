import { getPlatformCapabilities } from '@/providers/platformCapabilities';
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

type ImportStatus = 'pending' | 'importing' | 'success' | 'failed' | 'cancelled';

interface ImportFileState {
  filename: string;
  title: string;
  status: ImportStatus;
  error: string;
  createdId?: string;
}

export interface ImportUnit {
  filename: string;
  title: string;
}

interface FileImportUnit extends ImportUnit {
  file: File;
}

interface LocalPathImportUnit extends ImportUnit {
  localPath: string;
}

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
  current: number;
  filename: string;
  percentage: number;
}

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
  let chapterDetectionGeneration = 0;

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
    chapterDetectionGeneration += 1;
    chapterSuggestion.value = null;
    convertingChapters.value = false;
    chapterConversionError.value = '';
  }

  async function detectChapterConversion(result: ImportSubmitResult | null): Promise<void> {
    clearChapterSuggestion();
    const generation = chapterDetectionGeneration;
    if (!result || result.total !== 1 || result.successCount !== 1 || !result.firstImportedId) {
      return;
    }
    const imported = files.value.find((item) => item.createdId === result.firstImportedId);
    if (!imported || !txtExtPattern.test(imported.filename)) {
      return;
    }

    try {
      const { content } = await getBookshelfProvider().getBookContent(result.firstImportedId);
      if (generation !== chapterDetectionGeneration) {
        return;
      }
      const chapters = countTextChapters(content);
      if (chapters > 0) {
        chapterSuggestion.value = { bookId: result.firstImportedId, chapters, content };
      }
    } catch {
    }
  }

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
        title: isEpub ? '' : unit.title,
        folder,
        file: unit.file,
        strategy: isEpub ? epubStrategy.value : undefined
      });
      return created.id;
    };
  }

  function buildLocalPathUnits(localPaths: string[]): LocalPathImportUnit[] {
    return localPaths
      .map((localPath) => localPath.trim())
      .filter((localPath) => localPath.length > 0)
      .map((localPath) => {
        const filename = basenameFromPath(localPath);
        return { localPath, filename, title: deriveTitleFromFilename(filename) };
      });
  }

  function createLocalPathExecutor(currentFolderPath?: string): ImportExecutor<LocalPathImportUnit> {
    const folder = normalizeImportFolderPath(currentFolderPath);
    return async (unit) => {
      bookshelfWriter();
      const result = (await getPlatformCapabilities().importBookFromLocalPath?.(unit.localPath, folder)) ?? null;
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

  async function submitFiles(currentFolderPath?: string): Promise<ImportSubmitResult | null> {
    return submit(buildFileUnits(bookFiles.value), createFileExecutor(currentFolderPath));
  }

  async function submitLocalPaths(
    localPaths: string[],
    currentFolderPath?: string
  ): Promise<ImportSubmitResult | null> {
    return submit(buildLocalPathUnits(localPaths), createLocalPathExecutor(currentFolderPath));
  }

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
