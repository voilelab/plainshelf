import { computed, ref, watch } from 'vue';

const FONT_SIZE_STORAGE_KEY = 'reader-font-size';
const FONT_FAMILY_STORAGE_KEY = 'reader-font-family';
const DEFAULT_FONT_SIZE = 20;
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

function parseFontSize(rawValue: string | null): number {
  if (!rawValue) {
    return DEFAULT_FONT_SIZE;
  }

  const parsed = Number.parseInt(rawValue, 10);
  if (Number.isNaN(parsed)) {
    return DEFAULT_FONT_SIZE;
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

export function useReaderSettings() {
  const fontSize = ref(DEFAULT_FONT_SIZE);
  const fontFamily = ref<ReaderFont>(DEFAULT_READER_FONT);

  if (typeof window !== 'undefined') {
    fontSize.value = parseFontSize(window.localStorage.getItem(FONT_SIZE_STORAGE_KEY));
    fontFamily.value = parseReaderFont(window.localStorage.getItem(FONT_FAMILY_STORAGE_KEY));
  }

  watch(fontSize, (value) => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(FONT_SIZE_STORAGE_KEY, String(clampFontSize(value)));
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
  }

  function decreaseFontSize(): void {
    fontSize.value = clampFontSize(fontSize.value - FONT_SIZE_STEP);
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
