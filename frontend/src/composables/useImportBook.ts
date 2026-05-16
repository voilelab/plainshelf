import { ref } from 'vue';
import { importBook } from '../api/books';
import { deriveTitleFromFilename, hasSupportedExtension } from '../utils/file';
import { normalizeLayerPath } from '../utils/layers';

const bookExtPattern = /\.(txt)$/i;

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

  function normalizeImportLayerPath(currentLayerPath?: string): string {
    const normalized = normalizeLayerPath(currentLayerPath ?? '');
    return normalized.length > 0 ? normalized : '/';
  }

  async function submit(currentLayerPath?: string): Promise<ImportSubmitResult | null> {
    if (submitting.value) {
      return null;
    }

    error.value = '';
    success.value = '';

    if (bookFiles.value.length === 0) {
      error.value = 'Please choose at least one TXT file.';
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
            error: 'Book file must be .txt.'
          };
          failedCount += 1;
          continue;
        }

        files.value[index] = {
          ...current,
          status: 'importing',
          error: ''
        };

        try {
          const created = await importBook({
            title: current.title,
            layer: normalizeImportLayerPath(currentLayerPath),
            file: current.file
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
        success.value = total === 1 ? 'Import successful.' : `Imported ${successCount} files.`;
        error.value = '';
      } else if (successCount > 0) {
        success.value = `Imported ${successCount} of ${total} files.`;
        error.value = `${failedCount} file(s) failed.`;
      } else {
        success.value = '';
        error.value = 'Import failed.';
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
    setBookFiles,
    submit,
    reset
  };
}
