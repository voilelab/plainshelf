import { computed, ref, toValue, watch, type MaybeRefOrGetter } from 'vue';

const FONT_SIZE_STORAGE_KEY = 'reader-font-size';
const FONT_FAMILY_STORAGE_KEY = 'reader-font-family';
export const DEFAULT_READER_FONT_SIZE = 20;
export const MOBILE_DEFAULT_FONT_SIZE = 22;
const MIN_FONT_SIZE = 14;
const MAX_FONT_SIZE = 36;
const FONT_SIZE_STEP = 2;

export const READER_FONT_OPTIONS = [
  {
    id: 'system',
    cssFamily: "Georgia, 'Times New Roman', 'Noto Serif TC', 'Songti TC', serif"
  },
  {
    id: 'noto-serif-tc',
    cssFamily: "'Noto Serif TC Variable', Georgia, 'Times New Roman', serif"
  },
  {
    id: 'noto-sans-tc',
    cssFamily: "'Noto Sans TC Variable', 'Noto Sans TC', system-ui, sans-serif"
  }
] as const;

export type ReaderFont = (typeof READER_FONT_OPTIONS)[number]['id'];

export const DEFAULT_READER_FONT: ReaderFont = 'system';

const READER_FONT_IDS = READER_FONT_OPTIONS.map((option) => option.id);

function clampFontSize(value: number): number {
  return Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, value));
}

function parseFontSize(rawValue: string | null, fallback: number): number {
  if (!rawValue) {
    return clampFontSize(fallback);
  }

  const parsed = Number.parseInt(rawValue, 10);
  if (Number.isNaN(parsed)) {
    return clampFontSize(fallback);
  }

  return clampFontSize(parsed);
}

export function parseReaderFont(rawValue: string | null): ReaderFont {
  if (rawValue && READER_FONT_IDS.includes(rawValue as ReaderFont)) {
    return rawValue as ReaderFont;
  }
  return DEFAULT_READER_FONT;
}

export function getReaderFontFamily(font: ReaderFont): string {
  return READER_FONT_OPTIONS.find((option) => option.id === font)?.cssFamily ?? READER_FONT_OPTIONS[0].cssFamily;
}

export function useReaderSettings(defaultFontSize: MaybeRefOrGetter<number> = DEFAULT_READER_FONT_SIZE) {
  const fontSize = ref(clampFontSize(toValue(defaultFontSize)));
  const fontFamily = ref<ReaderFont>(DEFAULT_READER_FONT);
  const hasExplicitFontSize = ref(false);

  if (typeof window !== 'undefined') {
    const storedFontSize = window.localStorage.getItem(FONT_SIZE_STORAGE_KEY);
    const parsedStoredFontSize = storedFontSize === null ? Number.NaN : Number.parseInt(storedFontSize, 10);
    hasExplicitFontSize.value = Number.isFinite(parsedStoredFontSize);
    fontSize.value = parseFontSize(storedFontSize, toValue(defaultFontSize));
    fontFamily.value = parseReaderFont(window.localStorage.getItem(FONT_FAMILY_STORAGE_KEY));
  }

  watch(() => toValue(defaultFontSize), (value) => {
    if (!hasExplicitFontSize.value) {
      fontSize.value = clampFontSize(value);
    }
  });

  watch(fontFamily, (value) => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(FONT_FAMILY_STORAGE_KEY, parseReaderFont(value));
  });

  const isAtMinFontSize = computed(() => fontSize.value <= MIN_FONT_SIZE);
  const isAtMaxFontSize = computed(() => fontSize.value >= MAX_FONT_SIZE);

  function increaseFontSize(): void {
    fontSize.value = clampFontSize(fontSize.value + FONT_SIZE_STEP);
    persistFontSize();
  }

  function decreaseFontSize(): void {
    fontSize.value = clampFontSize(fontSize.value - FONT_SIZE_STEP);
    persistFontSize();
  }

  function persistFontSize(): void {
    hasExplicitFontSize.value = true;
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(FONT_SIZE_STORAGE_KEY, String(fontSize.value));
    }
  }

  function setFontFamily(value: ReaderFont): void {
    fontFamily.value = parseReaderFont(value);
  }

  return {
    fontSize,
    fontFamily,
    isAtMinFontSize,
    isAtMaxFontSize,
    increaseFontSize,
    decreaseFontSize,
    setFontFamily
  };
}
