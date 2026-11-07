const COMIC_FITS = ['fit-height', 'fit-width', 'original'] as const;
export type ComicFit = (typeof COMIC_FITS)[number];

export const COMIC_ZOOM_MIN = 50;
export const COMIC_ZOOM_MAX = 300;
export const COMIC_ZOOM_STEP = 25;
export const COMIC_ZOOM_DEFAULT = 100;

interface Size { width: number; height: number }

export function parseComicFit(raw: string | null): ComicFit {
  return COMIC_FITS.find((fit) => fit === raw) ?? 'fit-height';
}

export function clampComicZoom(value: number): number {
  if (!Number.isFinite(value)) return COMIC_ZOOM_DEFAULT;
  const snapped = Math.round(value / COMIC_ZOOM_STEP) * COMIC_ZOOM_STEP;
  return Math.min(COMIC_ZOOM_MAX, Math.max(COMIC_ZOOM_MIN, snapped));
}

export function comicPageSize(natural: Size, viewport: Size, fit: ComicFit, zoom: number): Size | null {
  if (natural.width <= 0 || natural.height <= 0 || viewport.width <= 0 || viewport.height <= 0) return null;
  const widthScale = viewport.width / natural.width;
  const base = fit === 'original' ? 1 : fit === 'fit-width' ? widthScale : Math.min(widthScale, viewport.height / natural.height);
  const scale = base * zoom / 100;
  return { width: Math.floor(natural.width * scale), height: Math.floor(natural.height * scale) };
}
