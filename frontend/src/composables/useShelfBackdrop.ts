import { computed, ref, watch, type CSSProperties } from 'vue';
import { useShelvesStore } from '@/composables/useShelvesStore';
import { createShelfBackdropStorage, type ShelfBackdrop, type ShelfBackdropSwatch } from '@/storage/shelfBackdrops';
import { t } from '@/i18n';

export const SHELF_BACKDROP_MAX_IMAGE_BYTES = 8 * 1024 * 1024;

type ShownBackdrop = { kind: 'color'; swatch: ShelfBackdropSwatch } | { kind: 'image'; url: string };

const storage = createShelfBackdropStorage();
const shown = ref<ShownBackdrop | null>(null);
let loadSeq = 0;
let started = false;

function show(backdrop: ShelfBackdrop | null): void {
  if (shown.value?.kind === 'image') URL.revokeObjectURL(shown.value.url);
  if (!backdrop) {
    shown.value = null;
  } else if (backdrop.kind === 'color') {
    shown.value = backdrop;
  } else {
    shown.value = { kind: 'image', url: URL.createObjectURL(backdrop.image) };
  }
}

async function load(shelfID: string): Promise<void> {
  const seq = ++loadSeq;
  let backdrop: ShelfBackdrop | null = null;
  if (shelfID) {
    try {
      backdrop = await storage.get(shelfID);
    } catch (err) {
      console.error('Failed to load shelf backdrop:', err);
    }
  }
  if (seq === loadSeq) show(backdrop);
}

function start(): void {
  if (started) return;
  started = true;
  const { selectedShelfID } = useShelvesStore();
  watch(selectedShelfID, (id) => void load(id), { immediate: true });
}

function isQuotaError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'QuotaExceededError';
}

async function save(backdrop: ShelfBackdrop | null): Promise<void> {
  const { selectedShelfID } = useShelvesStore();
  const shelfID = selectedShelfID.value;
  if (!shelfID) return;
  if (backdrop?.kind === 'image') {
    if (!backdrop.image.type.startsWith('image/')) {
      throw new Error(t('library.backdrop.notImage'));
    }
    if (backdrop.image.size > SHELF_BACKDROP_MAX_IMAGE_BYTES) {
      throw new Error(t('library.backdrop.tooLarge', { max: SHELF_BACKDROP_MAX_IMAGE_BYTES / 1024 / 1024 }));
    }
  }
  try {
    if (backdrop) await storage.set(shelfID, backdrop);
    else await storage.delete(shelfID);
  } catch (err) {
    console.error('Failed to save shelf backdrop:', err);
    throw new Error(isQuotaError(err) ? t('library.backdrop.quotaExceeded') : t('library.backdrop.saveFailed'));
  }
  loadSeq++;
  if (selectedShelfID.value === shelfID) show(backdrop);
}

const layoutClass = computed(() => (shown.value ? `shelf-backdrop-${shown.value.kind}` : ''));

const layoutStyle = computed<CSSProperties>(() => {
  const value = shown.value;
  if (!value) return {};
  if (value.kind === 'color') return { '--shelf-backdrop-color': `var(--shelf-backdrop-${value.swatch})` };
  return { '--shelf-backdrop-image': `url("${value.url}")` };
});

export function useShelfBackdrop() {
  start();
  return {
    current: computed(() => shown.value),
    layoutClass,
    layoutStyle,
    setColor: (swatch: ShelfBackdropSwatch) => save({ kind: 'color', swatch }),
    setImage: (image: Blob) => save({ kind: 'image', image }),
    clear: () => save(null)
  };
}
