import { computed, ref } from 'vue';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { Book, SplitConfig } from '@/types/book';
import type { CreateSourceOptions, SourceMeta } from '@/types/source';

export interface DerivedSourceInput {
  content: string;
  format: 'txt' | 'md';
  comment: string;
  setCurrent: boolean;
}

export function useSourceEditorSession(
  bookId: () => string,
  writesEnabled: () => boolean
) {
  const book = ref<Book | null>(null);
  const sources = ref<SourceMeta[]>([]);
  const activeSourceId = ref('');
  const initialContent = ref('');
  const content = ref('');

  const initialLoading = ref(false);
  const listLoading = ref(false);
  const contentLoading = ref(false);
  const saving = ref(false);
  const creating = ref(false);
  const deleting = ref(false);
  const settingCurrent = ref(false);

  const loadError = ref('');
  const editorError = ref('');
  const saveSuccess = ref('');
  const deleteError = ref('');
  const conversionError = ref('');

  let sessionGeneration = 0;
  let contentGeneration = 0;
  let metadataGeneration = 0;

  const activeSourceMeta = computed(() =>
    sources.value.find((source) => source.id === activeSourceId.value)
  );
  const isLegacySource = computed(() => !activeSourceMeta.value?.format);
  const activeFormat = computed<'txt' | 'md'>(() =>
    activeSourceMeta.value?.format === 'md' ? 'md' : 'txt'
  );
  const isDirty = computed(() =>
    activeSourceId.value.length > 0 && content.value !== initialContent.value
  );
  const disableSave = computed(() =>
    !activeSourceId.value ||
    !isDirty.value ||
    saving.value ||
    contentLoading.value ||
    initialLoading.value
  );

  function isCurrentSession(generation: number, requestedBookId: string): boolean {
    return generation === sessionGeneration && requestedBookId === bookId();
  }

  function clearContent(): void {
    activeSourceId.value = '';
    content.value = '';
    initialContent.value = '';
  }

  function resetSession(): number {
    const generation = ++sessionGeneration;
    contentGeneration += 1;
    metadataGeneration += 1;
    book.value = null;
    sources.value = [];
    clearContent();
    initialLoading.value = true;
    listLoading.value = false;
    contentLoading.value = false;
    saving.value = false;
    creating.value = false;
    deleting.value = false;
    settingCurrent.value = false;
    loadError.value = '';
    editorError.value = '';
    saveSuccess.value = '';
    deleteError.value = '';
    conversionError.value = '';
    return generation;
  }

  async function fetchInitial(): Promise<void> {
    const requestedBookId = bookId();
    const generation = resetSession();

    try {
      const provider = getBookshelfProvider();
      const [bookData, sourceList] = await Promise.all([
        provider.getBook(requestedBookId),
        provider.listSources(requestedBookId)
      ]);
      if (!isCurrentSession(generation, requestedBookId)) return;

      book.value = bookData;
      sources.value = sourceList;
      const preferredSource =
        sourceList.find((source) => source.id === bookData.current_source)?.id ??
        sourceList[0]?.id ??
        '';

      if (preferredSource) {
        await loadSourceForSession(preferredSource, generation, requestedBookId);
      } else {
        clearContent();
      }
    } catch (error) {
      if (isCurrentSession(generation, requestedBookId)) {
        loadError.value = errorMessage(error, 'Failed to load sources');
      }
    } finally {
      if (isCurrentSession(generation, requestedBookId)) {
        initialLoading.value = false;
      }
    }
  }

  async function reloadSourceMeta(
    expectedSession = sessionGeneration,
    requestedBookId = bookId()
  ): Promise<void> {
    if (!isCurrentSession(expectedSession, requestedBookId)) return;
    const request = ++metadataGeneration;
    listLoading.value = true;
    try {
      const sourceList = await getBookshelfProvider().listSources(requestedBookId);
      if (
        isCurrentSession(expectedSession, requestedBookId) &&
        request === metadataGeneration
      ) {
        sources.value = sourceList;
      }
    } finally {
      if (
        isCurrentSession(expectedSession, requestedBookId) &&
        request === metadataGeneration
      ) {
        listLoading.value = false;
      }
    }
  }

  async function loadSource(sourceId: string): Promise<void> {
    await loadSourceForSession(sourceId, sessionGeneration, bookId());
  }

  async function loadSourceForSession(
    sourceId: string,
    expectedSession: number,
    requestedBookId: string
  ): Promise<void> {
    if (!sourceId || !isCurrentSession(expectedSession, requestedBookId)) return;
    const request = ++contentGeneration;
    contentLoading.value = true;
    editorError.value = '';
    saveSuccess.value = '';

    try {
      const text = await getBookshelfProvider().getSourceContent(requestedBookId, sourceId);
      if (
        !isCurrentSession(expectedSession, requestedBookId) ||
        request !== contentGeneration
      ) return;

      activeSourceId.value = sourceId;
      content.value = text;
      initialContent.value = text;
    } catch (error) {
      if (
        isCurrentSession(expectedSession, requestedBookId) &&
        request === contentGeneration
      ) {
        editorError.value = errorMessage(error, 'Failed to load source content');
      }
    } finally {
      if (
        isCurrentSession(expectedSession, requestedBookId) &&
        request === contentGeneration
      ) {
        contentLoading.value = false;
      }
    }
  }

  async function save(): Promise<void> {
    if (!writesEnabled() || !activeSourceId.value || !isDirty.value || saving.value) return;
    const generation = sessionGeneration;
    const requestedBookId = bookId();
    const sourceId = activeSourceId.value;
    const snapshot = content.value;
    saving.value = true;
    editorError.value = '';
    saveSuccess.value = '';

    try {
      await bookshelfWriter().updateSourceContent(requestedBookId, sourceId, snapshot);
      if (!isActiveSource(generation, requestedBookId, sourceId)) return;
      initialContent.value = snapshot;
      await reloadSourceMeta(generation, requestedBookId);
      if (isActiveSource(generation, requestedBookId, sourceId)) {
        saveSuccess.value = 'Source saved.';
      }
    } catch (error) {
      if (isActiveSource(generation, requestedBookId, sourceId)) {
        editorError.value = errorMessage(error, 'Failed to save source');
      }
    } finally {
      if (isCurrentSession(generation, requestedBookId)) saving.value = false;
    }
  }

  async function setCurrentSource(): Promise<void> {
    const sourceId = activeSourceId.value;
    if (!writesEnabled() || !sourceId || settingCurrent.value) return;
    const generation = sessionGeneration;
    const requestedBookId = bookId();
    settingCurrent.value = true;
    editorError.value = '';
    saveSuccess.value = '';

    try {
      await bookshelfWriter().setCurrentSource(requestedBookId, sourceId);
      if (!isCurrentSession(generation, requestedBookId)) return;
      if (book.value) book.value.current_source = sourceId;
      if (activeSourceId.value === sourceId) {
        saveSuccess.value = 'Current source updated.';
      }
    } catch (error) {
      if (isActiveSource(generation, requestedBookId, sourceId)) {
        editorError.value = errorMessage(error, 'Failed to set current source');
      }
    } finally {
      if (isCurrentSession(generation, requestedBookId)) settingCurrent.value = false;
    }
  }

  async function createSource(): Promise<boolean> {
    if (!writesEnabled() || creating.value) return false;
    return createSourceWithOptions(undefined, 'Failed to create source');
  }

  async function createDerivedSource(input: DerivedSourceInput): Promise<boolean> {
    if (!writesEnabled() || isDirty.value || creating.value) return false;
    conversionError.value = '';
    return createSourceWithOptions(
      {
        content: input.content,
        format: input.format,
        comment: input.comment,
        setCurrent: input.setCurrent
      },
      'Failed to create derived source',
      input
    );
  }

  async function createSourceWithOptions(
    options: CreateSourceOptions | undefined,
    fallbackError: string,
    derived?: DerivedSourceInput
  ): Promise<boolean> {
    const generation = sessionGeneration;
    const requestedBookId = bookId();
    const startingContentGeneration = contentGeneration;
    creating.value = true;
    editorError.value = '';
    saveSuccess.value = '';

    try {
      const writer = bookshelfWriter();
      const next = options
        ? await writer.createSource(requestedBookId, options)
        : await writer.createSource(requestedBookId);
      if (!isCurrentSession(generation, requestedBookId)) return false;
      await reloadSourceMeta(generation, requestedBookId);
      if (!isCurrentSession(generation, requestedBookId)) return false;
      if (derived?.setCurrent && book.value) {
        book.value.current_source = next.id;
        book.value.format = derived.format;
      }
      if (contentGeneration === startingContentGeneration) {
        await loadSourceForSession(next.id, generation, requestedBookId);
      }
      if (!isCurrentSession(generation, requestedBookId)) return false;
      if (derived) saveSuccess.value = 'Derived source created.';
      return true;
    } catch (error) {
      if (isCurrentSession(generation, requestedBookId)) {
        const message = errorMessage(error, fallbackError);
        if (derived) conversionError.value = message;
        else editorError.value = message;
      }
      return false;
    } finally {
      if (isCurrentSession(generation, requestedBookId)) creating.value = false;
    }
  }

  async function deleteSource(sourceId: string): Promise<boolean> {
    if (!writesEnabled() || !sourceId || deleting.value) return false;
    const generation = sessionGeneration;
    const requestedBookId = bookId();
    const startingContentGeneration = contentGeneration;
    deleting.value = true;
    deleteError.value = '';

    try {
      await bookshelfWriter().deleteSource(requestedBookId, sourceId);
      if (!isCurrentSession(generation, requestedBookId)) return false;
      await reloadSourceMeta(generation, requestedBookId);
      if (!isCurrentSession(generation, requestedBookId)) return false;

      if (
        activeSourceId.value === sourceId &&
        contentGeneration === startingContentGeneration
      ) {
        const preferredSource =
          sources.value.find((source) => source.id === book.value?.current_source)?.id ??
          sources.value[0]?.id ??
          '';
        if (preferredSource) {
          await loadSourceForSession(preferredSource, generation, requestedBookId);
        } else {
          clearContent();
        }
      }
      return isCurrentSession(generation, requestedBookId);
    } catch (error) {
      if (isCurrentSession(generation, requestedBookId)) {
        deleteError.value = errorMessage(error, 'Failed to delete source');
      }
      return false;
    } finally {
      if (isCurrentSession(generation, requestedBookId)) deleting.value = false;
    }
  }

  function isActiveSource(
    generation: number,
    requestedBookId: string,
    sourceId: string
  ): boolean {
    return isCurrentSession(generation, requestedBookId) && activeSourceId.value === sourceId;
  }

  return {
    book,
    sources,
    activeSourceId,
    content,
    initialLoading,
    listLoading,
    contentLoading,
    saving,
    creating,
    deleting,
    settingCurrent,
    loadError,
    editorError,
    saveSuccess,
    deleteError,
    conversionError,
    activeSourceMeta,
    isLegacySource,
    activeFormat,
    isDirty,
    disableSave,
    fetchInitial,
    loadSource,
    save,
    setCurrentSource,
    createSource,
    createDerivedSource,
    deleteSource
  };
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}
