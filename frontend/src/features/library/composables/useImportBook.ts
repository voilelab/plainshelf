import { t } from '@/i18n';
import { ref } from 'vue';
import { bookshelfWriter } from '@/providers';
import { deriveTitleFromFilename, hasSupportedExtension } from '@/utils/file';
import { normalizeFolderPath } from '@/utils/folders';
import { DEFAULT_EPUB_IMPORT_STRATEGY, type EpubImportStrategy } from '@/types/book';

const bookExtPattern = /\.(txt|md|epub)$/i;
const epubExtPattern = /\.epub$/i;

export type ImportStatus = 'pending' | 'importing' | 'success' | 'failed';

export interface ImportFileState {
  filename: string;
  title: string;
  status: ImportStatus;
  error: string;
}

interface ImportBookItem extends ImportFileState {
  file: File;
  createdId?: string;
}

export interface ImportSubmitResult {
  total: number;
  successCount: number;
  failedCount: number;
  firstImportedId?: string;
}

export function useImportBook() {
  const bookFiles = ref<File[]>([]);
  const files = ref<ImportBookItem[]>([]);
  const submitting = ref(false);
  const success = ref('');
  const error = ref('');
  const epubStrategy = ref<EpubImportStrategy>({ ...DEFAULT_EPUB_IMPORT_STRATEGY });

  function setEpubStrategy(next: EpubImportStrategy): void {
    epubStrategy.value = { ...next };
  }

  function hasEpubFile(): boolean {
    return bookFiles.value.some((file) => epubExtPattern.test(file.name));
  }

  function toImportBookItems(nextFiles: File[]): ImportBookItem[] {
    return nextFiles.map((file) => ({
      file,
      filename: file.name,
      title: deriveTitleFromFilename(file.name),
      status: 'pending',
      error: ''
    }));
  }

  function reset(): void {
    bookFiles.value = [];
    files.value = [];
    submitting.value = false;
    success.value = '';
    error.value = '';
  }

  function setBookFiles(nextFiles: File[]): void {
    bookFiles.value = nextFiles;
    files.value = toImportBookItems(nextFiles);
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

  async function submit(currentFolderPath?: string): Promise<ImportSubmitResult | null> {
    if (submitting.value) {
      return null;
    }

    error.value = '';
    success.value = '';

    if (bookFiles.value.length === 0) {
      error.value = t('libraryForms.importBook.errors.noFiles');
      return null;
    }

    files.value = toImportBookItems(bookFiles.value);
    submitting.value = true;
    let successCount = 0;
    let failedCount = 0;
    let firstImportedId: string | undefined;

    try {
      for (let index = 0; index < files.value.length; index += 1) {
        const current = files.value[index];

        if (!hasSupportedExtension(current.filename, bookExtPattern)) {
          files.value[index] = {
            ...current,
            status: 'failed',
            error: t('libraryForms.importBook.errors.unsupportedExtension')
          };
          failedCount += 1;
          continue;
        }

        files.value[index] = {
          ...current,
          status: 'importing',
          error: ''
        };

        const isEpub = epubExtPattern.test(current.filename);

        try {
          const created = await bookshelfWriter().importBook({
            // An EPUB knows its own title; sending the one derived from the
            // filename would only override it with something worse.
            title: isEpub ? '' : current.title,
            folder: normalizeImportFolderPath(currentFolderPath),
            file: current.file,
            strategy: isEpub ? epubStrategy.value : undefined
          });

          files.value[index] = {
            ...current,
            status: 'success',
            error: '',
            createdId: created.id
          };
          successCount += 1;
          if (!firstImportedId) {
            firstImportedId = created.id;
          }
        } catch (err) {
          files.value[index] = {
            ...current,
            status: 'failed',
            error: getSafeErrorMessage(err)
          };
          failedCount += 1;
        }
      }

      const total = files.value.length;
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

      return {
        total,
        successCount,
        failedCount,
        firstImportedId
      };
    } finally {
      submitting.value = false;
    }
  }

  return {
    bookFiles,
    files,
    submitting,
    success,
    error,
    epubStrategy,
    setEpubStrategy,
    hasEpubFile,
    setBookFiles,
    submit,
    reset
  };
}
