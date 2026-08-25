import { t } from '@/i18n';
import { computed, ref } from 'vue';
import { bookshelfWriter } from '@/providers';
import { deriveTitleFromFilename, hasSupportedExtension } from '@/utils/file';
import { normalizeFolderPath } from '@/utils/folders';
import { DEFAULT_EPUB_IMPORT_STRATEGY, type EpubImportStrategy } from '@/types/book';

const bookExtPattern = /\.(txt|md|epub)$/i;
const epubExtPattern = /\.epub$/i;

// 'cancelled' is deliberately distinct from 'failed': a book left untouched by an
// abort must never read as an import error. It mirrors taskutil's
// partially_completed, so a future server-side taskchain keeps the same mental
// model.
export type ImportStatus = 'pending' | 'importing' | 'success' | 'failed' | 'cancelled';

export interface ImportFileState {
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

// Turns one unit into a created book id, or throws to fail that unit. Abort is
// cooperative and only checked between units, so an executor may ignore the
// signal; it is passed so an implementation that can bail out early may.
export type ImportExecutor<T extends ImportUnit = ImportUnit> = (
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

export interface ImportProgress {
  total: number;
  completed: number;
  // 1-based ordinal of the book currently importing; falls back to the completed
  // count when nothing is in flight.
  current: number;
  filename: string;
  percentage: number;
}

export function useImportBook() {
  const bookFiles = ref<File[]>([]);
  const files = ref<ImportFileState[]>([]);
  const submitting = ref(false);
  const cancelRequested = ref(false);
  const success = ref('');
  const error = ref('');
  const epubStrategy = ref<EpubImportStrategy>({ ...DEFAULT_EPUB_IMPORT_STRATEGY });

  let controller: AbortController | null = null;

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
    bookFiles.value = [];
    files.value = [];
    submitting.value = false;
    cancelRequested.value = false;
    success.value = '';
    error.value = '';
  }

  function setBookFiles(nextFiles: File[]): void {
    bookFiles.value = nextFiles;
    files.value = toImportFileStates(buildFileUnits(nextFiles));
    success.value = '';
    error.value = '';
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
    setEpubStrategy,
    hasEpubFile,
    setBookFiles,
    submit,
    submitFiles,
    abort,
    reset
  };
}
