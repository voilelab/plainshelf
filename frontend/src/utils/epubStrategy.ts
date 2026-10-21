import {
  DEFAULT_EPUB_IMPORT_STRATEGY,
  type EpubImportPreset,
  type EpubImportStrategy
} from '@/types/book';

export function normalizeEpubImportPreset(value: unknown): EpubImportPreset {
  if (value === 'markdown' || value === 'plain') {
    return value;
  }
  return DEFAULT_EPUB_IMPORT_STRATEGY.preset;
}

export function normalizeEpubImportStrategy(raw: unknown): EpubImportStrategy {
  if (!raw || typeof raw !== 'object') {
    return { ...DEFAULT_EPUB_IMPORT_STRATEGY };
  }

  const data = raw as Record<string, unknown>;

  const strategy: EpubImportStrategy = {
    preset: normalizeEpubImportPreset(data.preset),
    include_description:
      typeof data.include_description === 'boolean'
        ? data.include_description
        : DEFAULT_EPUB_IMPORT_STRATEGY.include_description
  };

  if (typeof data.keep_images === 'boolean') {
    strategy.keep_images = data.keep_images;
  }

  return strategy;
}
