import { computed, ref, shallowRef, type ComputedRef, type Ref } from 'vue';

import { ROOT_FOLDER_ID } from '@/api/pcloud/client';
import { surfaceError } from '@/composables/useErrorIncident';

export interface PCloudFolderRef {
  name: string;
  folderid: number;
}

interface PCloudFolderListing {
  folders: PCloudFolderRef[];
  isShelf: boolean;
}

export interface PCloudFolderClient {
  listFolder(folderid: number, signal?: AbortSignal): Promise<PCloudFolderListing>;
  resolveFolderTrail(path: string, signal?: AbortSignal): Promise<PCloudFolderRef[]>;
}

const EMPTY_LISTING: PCloudFolderListing = { folders: [], isShelf: false };

interface PCloudFolderBrowser {
  trail: Ref<PCloudFolderRef[]>;
  folders: ComputedRef<PCloudFolderRef[]>;
  currentPath: ComputedRef<string>;
  isShelf: ComputedRef<boolean>;
  loading: Ref<boolean>;
  error: Ref<string>;
  notice: Ref<string>;
  open: (path: string) => Promise<void>;
  enter: (folder: PCloudFolderRef) => Promise<void>;
  goTo: (index: number) => Promise<void>;
  retry: () => Promise<void>;
  close: () => void;
}

export function usePCloudFolderBrowser(createClient: () => PCloudFolderClient): PCloudFolderBrowser {
  const trail = ref<PCloudFolderRef[]>([]);
  const level = shallowRef<PCloudFolderListing>(EMPTY_LISTING);
  const loading = ref(false);
  const error = ref('');
  const notice = ref('');

  const cache = new Map<number, PCloudFolderListing>();

  let activeListingRequest: AbortController | null = null;

  function beginListingRequest(): AbortController {
    activeListingRequest?.abort();
    const controller = new AbortController();
    activeListingRequest = controller;
    return controller;
  }

  function isSupersededListingRequest(controller: AbortController): boolean {
    return activeListingRequest !== controller;
  }

  const currentFolderID = computed(() =>
    trail.value.length > 0 ? trail.value[trail.value.length - 1].folderid : ROOT_FOLDER_ID
  );

  const currentPath = computed(() => `/${trail.value.map((level) => level.name).join('/')}`);

  const folders = computed(() =>
    [...level.value.folders].sort((a, b) => a.name.localeCompare(b.name))
  );

  const isShelf = computed(() => level.value.isShelf);

  async function show(next: PCloudFolderRef[]): Promise<void> {
    const controller = beginListingRequest();
    trail.value = next;
    error.value = '';
    notice.value = '';

    const folderid = next.length > 0 ? next[next.length - 1].folderid : ROOT_FOLDER_ID;
    const cached = cache.get(folderid);
    if (cached) {
      level.value = cached;
      loading.value = false;
      return;
    }

    level.value = EMPTY_LISTING;
    loading.value = true;
    try {
      const listing = await createClient().listFolder(folderid, controller.signal);
      if (isSupersededListingRequest(controller)) {
        return;
      }
      cache.set(folderid, listing);
      level.value = listing;
    } catch (err) {
      if (isSupersededListingRequest(controller)) {
        return;
      }
      error.value = surfaceError(err);
    } finally {
      if (!isSupersededListingRequest(controller)) {
        loading.value = false;
      }
    }
  }

  async function open(path: string): Promise<void> {
    cache.clear();

    const target = path.trim();
    if (target === '' || target === '/') {
      await show([]);
      return;
    }

    const controller = beginListingRequest();
    trail.value = [];
    level.value = EMPTY_LISTING;
    error.value = '';
    notice.value = '';
    loading.value = true;
    try {
      const resolved = await createClient().resolveFolderTrail(target, controller.signal);
      if (isSupersededListingRequest(controller)) {
        return;
      }
      await show(resolved);
    } catch (err) {
      if (isSupersededListingRequest(controller)) {
        return;
      }
      const reason = surfaceError(err);
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
    activeListingRequest?.abort();
    activeListingRequest = null;
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
