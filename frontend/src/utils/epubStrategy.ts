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

/**
 * Coerces an unknown value into a usable strategy. The server always sends a
 * complete object, but the setting is also read on paths where a stale or
 * partial value should degrade to the default rather than break the import
 * dialog.
 */
export function normalizeEpubImportStrategy(raw: unknown): EpubImportStrategy {
  if (!raw || typeof raw !== 'object') {
    return { ...DEFAULT_EPUB_IMPORT_STRATEGY };
  }

  const data = raw as Record<string, unknown>;

  return {
    preset: normalizeEpubImportPreset(data.preset),
    include_description:
      typeof data.include_description === 'boolean'
        ? data.include_description
        : DEFAULT_EPUB_IMPORT_STRATEGY.include_description
  };
}
