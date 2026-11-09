import { computed, ref } from 'vue';
import { ApiError } from '@/api/client';
import { t } from '@/i18n';
import { bookshelfWriter, getBookshelfProvider } from '@/providers';
import type { PageDirection, SourcePageList } from '@/types/source';
import { insertionIndex, movePage } from '@/features/sources/utils/pageOrder';

const PAGE_LIST_CONFLICT = 'PAGE_LIST_CONFLICT';

export function usePageListEditor(
  bookId: () => string,
  sourceId: () => string,
  writesEnabled: () => boolean,
  onChanged: () => void
) {
  const pageList = ref<SourcePageList | null>(null);
  const direction = ref<PageDirection>('ltr');
  const selectedIndex = ref<number | null>(null);
  const loading = ref(false);
  const busy = ref(false);
  const error = ref('');
  const conflict = ref(false);
  let generation = 0;

  const names = computed(() => pageList.value?.pages.map((page) => page.name) ?? []);
  const canEdit = computed(() =>
    writesEnabled() && !busy.value && !loading.value && !conflict.value && !!pageList.value?.md5_hash
  );

  async function load(initialDirection: PageDirection = 'ltr'): Promise<void> {
    const request = ++generation;
    loading.value = true;
    busy.value = false;
    error.value = '';
    conflict.value = false;
    selectedIndex.value = null;
    direction.value = initialDirection;
    try {
      const list = await getBookshelfProvider().getSourcePages(bookId(), sourceId());
      if (request === generation) pageList.value = list;
    } catch (cause) {
      if (request === generation) {
        pageList.value = null;
        error.value = messageOf(cause, t('sources.pages.errors.load'));
      }
    } finally {
      if (request === generation) loading.value = false;
    }
  }

  async function edit(
    run: (md5Hash: string) => Promise<SourcePageList>,
    fallback: string
  ): Promise<boolean> {
    const md5Hash = pageList.value?.md5_hash;
    if (!canEdit.value || !md5Hash) return false;
    const request = generation;
    busy.value = true;
    error.value = '';
    try {
      const next = await run(md5Hash);
      if (request !== generation) return false;
      pageList.value = next;
      onChanged();
      return true;
    } catch (cause) {
      if (request !== generation) return false;
      if (cause instanceof ApiError && cause.code === PAGE_LIST_CONFLICT) {
        conflict.value = true;
        error.value = t('sources.pages.errors.conflict');
      } else {
        error.value = messageOf(cause, fallback);
      }
      return false;
    } finally {
      if (request === generation) busy.value = false;
    }
  }

  function reorder(from: number, to: number): Promise<boolean> {
    const next = movePage(names.value, from, to);
    if (next.every((name, index) => name === names.value[index])) return Promise.resolve(false);
    if (selectedIndex.value === from) selectedIndex.value = to;
    return edit(
      (md5Hash) => bookshelfWriter().replaceSourcePages(bookId(), sourceId(), md5Hash, next),
      t('sources.pages.errors.save')
    );
  }

  function appendUnlisted(name: string): Promise<boolean> {
    return edit(
      (md5Hash) => bookshelfWriter().replaceSourcePages(bookId(), sourceId(), md5Hash, [...names.value, name]),
      t('sources.pages.errors.save')
    );
  }

  function removeImage(name: string): Promise<boolean> {
    selectedIndex.value = null;
    return edit(
      (md5Hash) => bookshelfWriter().deleteSourcePage(bookId(), sourceId(), md5Hash, name),
      t('sources.pages.errors.delete')
    );
  }

  async function addImages(files: readonly Blob[]): Promise<number> {
    let position = insertionIndex(selectedIndex.value, names.value.length);
    let added = 0;
    for (const file of files) {
      const inserted = await edit(
        (md5Hash) => bookshelfWriter().insertSourcePage(bookId(), sourceId(), md5Hash, position, file),
        t('sources.pages.errors.add')
      );
      if (!inserted) break;
      selectedIndex.value = position;
      position += 1;
      added += 1;
    }
    return added;
  }

  async function setDirection(next: PageDirection): Promise<void> {
    if (!writesEnabled() || busy.value || next === direction.value) return;
    const request = generation;
    busy.value = true;
    error.value = '';
    try {
      const meta = await bookshelfWriter().setSourcePageDirection(bookId(), sourceId(), next);
      if (request !== generation) return;
      direction.value = meta.page_direction ?? 'ltr';
      onChanged();
    } catch (cause) {
      if (request === generation) error.value = messageOf(cause, t('sources.pages.errors.direction'));
    } finally {
      if (request === generation) busy.value = false;
    }
  }

  function select(index: number | null): void {
    selectedIndex.value = selectedIndex.value === index ? null : index;
  }

  return {
    pageList,
    direction,
    selectedIndex,
    loading,
    busy,
    error,
    conflict,
    canEdit,
    load,
    reorder,
    appendUnlisted,
    removeImage,
    addImages,
    setDirection,
    select
  };
}

function messageOf(cause: unknown, fallback: string): string {
  return cause instanceof Error && cause.message ? cause.message : fallback;
}
