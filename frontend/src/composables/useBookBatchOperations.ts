import { computed, ref } from 'vue';
import { bookshelfWriter } from '@/providers';
import { useBookStore } from '@/composables/useBookStore';
import { useFolderStore } from '@/composables/useFolderStore';
import { isBookBatchResult, isTerminalTaskStatus, type BookBatchOperation, type BookBatchResult, type TaskChain, type TaskStatus } from '@/types/task';
import { t } from '@/i18n';

const POLL_INTERVAL_MS = 500;
const open = ref(false);
const submitting = ref(false);
const chain = ref<TaskChain | null>(null);
const status = ref<TaskStatus>('pending');
const percentage = ref(0);
const error = ref('');
const titleById = ref<Record<string, string>>({});
const completionVersion = ref(0);
const lastResult = ref<BookBatchResult | null>(null);
let timer: ReturnType<typeof setTimeout> | null = null;

const running = computed(() => submitting.value || (chain.value !== null && !isTerminalTaskStatus(status.value)));
const finished = computed(() => chain.value !== null && isTerminalTaskStatus(status.value));
const result = computed<BookBatchResult | null>(() => {
  for (const task of chain.value?.tasks ?? []) {
    if (isBookBatchResult(task.result)) return task.result;
  }
  return null;
});

function stopTimer(): void {
  if (timer !== null) clearTimeout(timer);
  timer = null;
}

async function settle(): Promise<void> {
  lastResult.value = result.value;
  completionVersion.value += 1;
  const { fetchBooks } = useBookStore();
  const { fetchFolders } = useFolderStore();
  await Promise.all([fetchBooks(), fetchFolders()]);
}

async function poll(id: string): Promise<void> {
  try {
    const next = await bookshelfWriter().getTaskChain(id);
    if (chain.value?.id !== id) return;
    chain.value = next;
    status.value = next.status;
    percentage.value = next.percentage;
    if (isTerminalTaskStatus(next.status)) {
      await settle();
      return;
    }
    timer = setTimeout(() => void poll(id), POLL_INTERVAL_MS);
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('bookCollection.selection.pollFailed');
    status.value = 'failed';
  }
}

async function start(operation: BookBatchOperation, bookIds: string[], titles: Record<string, string>, targetFolder?: string[]): Promise<void> {
  if (running.value || bookIds.length === 0) return;
  stopTimer();
  open.value = true;
  submitting.value = true;
  chain.value = null;
  status.value = 'pending';
  percentage.value = 0;
  error.value = '';
  titleById.value = { ...titles };
  lastResult.value = null;

  try {
    const id = await bookshelfWriter().startBookBatch({
      operation,
      book_ids: [...new Set(bookIds)],
      ...(operation === 'move' ? { target_folder: targetFolder ?? [] } : {})
    });
    chain.value = { id, name: 'book_batch', title: '', status: 'pending', percentage: 0, tasks: [] };
    timer = setTimeout(() => void poll(id), POLL_INTERVAL_MS);
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('bookCollection.selection.startFailed');
  } finally {
    submitting.value = false;
  }
}

function close(): void {
  if (running.value) return;
  open.value = false;
  chain.value = null;
  error.value = '';
  percentage.value = 0;
}

export function useBookBatchOperations() {
  return {
    open,
    chain,
    status,
    percentage,
    error,
    titleById,
    running,
    finished,
    result,
    lastResult,
    completionVersion,
    startMove: (bookIds: string[], targetFolder: string[], titles: Record<string, string>) => start('move', bookIds, titles, targetFolder),
    startTrash: (bookIds: string[], titles: Record<string, string>) => start('trash', bookIds, titles),
    close
  };
}
