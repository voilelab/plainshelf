import { computed, ref, watch } from 'vue';

const STORAGE_KEY = 'reader-font-size';
const DEFAULT_FONT_SIZE = 20;
const MIN_FONT_SIZE = 14;
const MAX_FONT_SIZE = 36;
const FONT_SIZE_STEP = 2;

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

export function useReaderSettings() {
  const fontSize = ref(DEFAULT_FONT_SIZE);

  if (typeof window !== 'undefined') {
    fontSize.value = parseFontSize(window.localStorage.getItem(STORAGE_KEY));
  }

  watch(fontSize, (value) => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, String(clampFontSize(value)));
  });

  const isAtMinFontSize = computed(() => fontSize.value <= MIN_FONT_SIZE);
  const isAtMaxFontSize = computed(() => fontSize.value >= MAX_FONT_SIZE);

  function increaseFontSize(): void {
    fontSize.value = clampFontSize(fontSize.value + FONT_SIZE_STEP);
  }

  function decreaseFontSize(): void {
    fontSize.value = clampFontSize(fontSize.value - FONT_SIZE_STEP);
  }

  return {
    fontSize,
    isAtMinFontSize,
    isAtMaxFontSize,
    increaseFontSize,
    decreaseFontSize
  };
}
