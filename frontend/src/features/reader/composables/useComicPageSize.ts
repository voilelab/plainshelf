import { getCurrentScope, onScopeDispose, ref } from 'vue';
import { getPreference, setPreference, subscribePreference } from '@/storage/preferences';
import {
  clampComicZoom,
  COMIC_ZOOM_DEFAULT,
  COMIC_ZOOM_STEP,
  parseComicFit,
  type ComicFit
} from '@/features/reader/utils/comicPageSize';

const FIT_KEY = 'reader-comic-fit';
const ZOOM_KEY = 'reader-comic-zoom';

function parseZoom(raw: string | null): number {
  return raw === null ? COMIC_ZOOM_DEFAULT : clampComicZoom(Number(raw));
}

export function useComicPageSize() {
  const fit = ref<ComicFit>(parseComicFit(getPreference(FIT_KEY)));
  const zoom = ref(parseZoom(getPreference(ZOOM_KEY)));
  const unsubscribers = [
    subscribePreference(FIT_KEY, (value) => { fit.value = parseComicFit(value); }),
    subscribePreference(ZOOM_KEY, (value) => { zoom.value = parseZoom(value); })
  ];
  if (getCurrentScope()) onScopeDispose(() => unsubscribers.forEach((unsubscribe) => unsubscribe()));

  function setFit(value: string): void {
    fit.value = parseComicFit(value);
    setPreference(FIT_KEY, fit.value);
  }
  function setZoom(value: number): void {
    zoom.value = clampComicZoom(value);
    setPreference(ZOOM_KEY, String(zoom.value));
  }

  return {
    fit,
    zoom,
    setFit,
    zoomIn: () => setZoom(zoom.value + COMIC_ZOOM_STEP),
    zoomOut: () => setZoom(zoom.value - COMIC_ZOOM_STEP),
    resetZoom: () => setZoom(COMIC_ZOOM_DEFAULT)
  };
}
