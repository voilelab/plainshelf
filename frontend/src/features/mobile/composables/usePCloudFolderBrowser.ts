import { computed, ref, shallowRef, type ComputedRef, type Ref } from 'vue';

import { BOOK_EXTENSION, findBooksFolder } from '@/api/pcloud/bookpkg';
import { ROOT_FOLDER_ID, type PCloudClient } from '@/api/pcloud/client';
import type { PCloudItem } from '@/api/pcloud/types';

/** One level of the browsed path. The account root has no entry: it has no name. */
export interface PCloudFolderRef {
  name: string;
  folderid: number;
}

/**
 * What the browser needs from a pCloud client, narrowed so a test can stand one
 * in without a fetch layer.
 */
export type PCloudFolderClient = Pick<PCloudClient, 'listFolder' | 'resolveFolderTrail'>;

interface PCloudFolderBrowser {
  /** Levels below the account root, outermost first. */
  trail: Ref<PCloudFolderRef[]>;
  /** Sub-folders of the current level, sorted, book packages removed. */
  folders: ComputedRef<PCloudFolderRef[]>;
  /** The current level as a shelf-root path, e.g. `/` or `/PlainShelf/shelf`. */
  currentPath: ComputedRef<string>;
  /** Whether the current level holds a `books/` directory, i.e. is a shelf. */
  isShelf: ComputedRef<boolean>;
  loading: Ref<boolean>;
  /** The current level could not be listed; nothing is on screen to browse. */
  error: Ref<string>;
  /**
   * Why the requested starting path was not opened. Kept apart from `error`
   * because the level below it did load: the user can carry on browsing, and
   * a later listing failure must not be overwritten by this.
   */
  notice: Ref<string>;
  open: (path: string) => Promise<void>;
  enter: (folder: PCloudFolderRef) => Promise<void>;
  /** Jumps to a trail level; -1 is the account root. */
  goTo: (index: number) => Promise<void>;
  retry: () => Promise<void>;
  close: () => void;
}

function describe(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/**
 * Drives a lazy, one-level-at-a-time walk of a pCloud account.
 *
 * Each level costs exactly one non-recursive `listfolder`, unlike the whole-tree
 * listing the shelf check runs (`PCloudClient.listFolderRecursive`), which is
 * far too expensive to spend on a folder the user is only passing through.
 * Levels already visited are answered from memory so walking back up is free.
 *
 * `createClient` is called per request rather than held, so re-authorizing in
 * the form mid-browse is picked up instead of silently reusing a stale token.
 */
export function usePCloudFolderBrowser(createClient: () => PCloudFolderClient): PCloudFolderBrowser {
  const trail = ref<PCloudFolderRef[]>([]);
  // Shallow: a listing is replaced wholesale and never edited in place, so deep
  // reactivity would only cost a proxy walk over every item in the folder.
  const contents = shallowRef<PCloudItem[]>([]);
  const loading = ref(false);
  const error = ref('');
  const notice = ref('');

  const cache = new Map<number, PCloudItem[]>();

  /**
   * The request whose result may still be written to the state above. A newer
   * navigation replaces it, which is what makes a stale reply — or the reply to
   * a request the user has already navigated away from — land nowhere instead
   * of overwriting the level now on screen.
   */
  let inFlight: AbortController | null = null;

  function begin(): AbortController {
    inFlight?.abort();
    const controller = new AbortController();
    inFlight = controller;
    return controller;
  }

  function isStale(controller: AbortController): boolean {
    return inFlight !== controller;
  }

  const currentFolderID = computed(() =>
    trail.value.length > 0 ? trail.value[trail.value.length - 1].folderid : ROOT_FOLDER_ID
  );

  const currentPath = computed(() => `/${trail.value.map((level) => level.name).join('/')}`);

  const folders = computed(() =>
    contents.value
      .filter(
        (item) =>
          item.isfolder &&
          item.folderid !== undefined &&
          // A book package is a directory too, but it can never be a shelf root
          // and listing them would bury the folders under them.
          !item.name.endsWith(BOOK_EXTENSION)
      )
      .map((item) => ({ name: item.name, folderid: item.folderid as number }))
      .sort((a, b) => a.name.localeCompare(b.name))
  );

  // Decided from the level already on screen: its own listing names the
  // children, so no folder has to be opened to know whether it is a shelf.
  const isShelf = computed(() =>
    Boolean(findBooksFolder({ name: '', isfolder: true, contents: contents.value }))
  );

  async function show(next: PCloudFolderRef[]): Promise<void> {
    const controller = begin();
    trail.value = next;
    error.value = '';
    // Whatever the notice was about, the user has moved on from it.
    notice.value = '';

    const folderid = next.length > 0 ? next[next.length - 1].folderid : ROOT_FOLDER_ID;
    const cached = cache.get(folderid);
    if (cached) {
      contents.value = cached;
      loading.value = false;
      return;
    }

    contents.value = [];
    loading.value = true;
    try {
      const listing = await createClient().listFolder(folderid, { signal: controller.signal });
      if (isStale(controller)) {
        return;
      }
      const items = listing.contents ?? [];
      cache.set(folderid, items);
      contents.value = items;
    } catch (err) {
      if (isStale(controller)) {
        return;
      }
      error.value = describe(err);
    } finally {
      if (!isStale(controller)) {
        loading.value = false;
      }
    }
  }

  /**
   * Opens the browser, starting at `path` when there is one.
   *
   * A path that no longer resolves — renamed, deleted, or belonging to the
   * account that was authorized before — is reported and then browsed around:
   * dropping the user at the root with an explanation beats refusing to open.
   * That explanation is a notice rather than an error, so the root stays
   * listed and browsable underneath it.
   */
  async function open(path: string): Promise<void> {
    // A fresh open may be reading a different account than the last one.
    cache.clear();

    const target = path.trim();
    if (target === '' || target === '/') {
      await show([]);
      return;
    }

    const controller = begin();
    trail.value = [];
    contents.value = [];
    error.value = '';
    notice.value = '';
    loading.value = true;
    try {
      const resolved = await createClient().resolveFolderTrail(target, controller.signal);
      if (isStale(controller)) {
        return;
      }
      await show(resolved);
    } catch (err) {
      if (isStale(controller)) {
        return;
      }
      const reason = describe(err);
      await show([]);
      notice.value = reason;
    }
  }

  async function enter(folder: PCloudFolderRef): Promise<void> {
    await show([...trail.value, folder]);
  }

  async function goTo(index: number): Promise<void> {
    await show(trail.value.slice(0, index + 1));
  }

  async function retry(): Promise<void> {
    cache.delete(currentFolderID.value);
    await show(trail.value);
  }

  function close(): void {
    inFlight?.abort();
    inFlight = null;
    loading.value = false;
  }

  return {
    trail,
    folders,
    currentPath,
    isShelf,
    loading,
    error,
    notice,
    open,
    enter,
    goTo,
    retry,
    close
  };
}
