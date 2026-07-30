import type { Book } from '../types/book';
import type { SidebarNavIconName } from '../types/sidebarNavIcon';

export type MaintenanceNavKey =
  | 'duplicate-content'
  | 'missing-author'
  | 'missing-cover'
  | 'missing-language'
  | 'low-char-count';

export type MaintenanceNavIcon = Extract<MaintenanceNavKey, SidebarNavIconName>;

export interface MaintenanceNavItem {
  key: MaintenanceNavKey;
  labelKey: string;
  to: string;
  icon?: MaintenanceNavIcon;
}

export const MAINTENANCE_NAV_ITEMS: MaintenanceNavItem[] = [
  {
    key: 'duplicate-content',
    labelKey: 'maintenance.duplicateContent',
    to: '/duplicates',
    icon: 'duplicate-content'
  },
  {
    key: 'missing-author',
    labelKey: 'maintenance.missingAuthor.title',
    to: '/books/maintenance/missing-author',
    icon: 'missing-author'
  },
  {
    key: 'missing-cover',
    labelKey: 'maintenance.missingCover.title',
    to: '/books/maintenance/missing-cover',
    icon: 'missing-cover'
  },
  {
    key: 'missing-language',
    labelKey: 'maintenance.missingLanguage.title',
    to: '/books/maintenance/missing-language',
    icon: 'missing-language'
  },
  {
    key: 'low-char-count',
    labelKey: 'maintenance.lowCharCount.title',
    to: '/books/maintenance/low-char-count',
    icon: 'low-char-count'
  }
];

export type MaintenanceBookFilter = Exclude<MaintenanceNavKey, 'duplicate-content'>;

/** Runtime inputs a filter predicate may read; ignored by predicates that don't need them. */
export interface MaintenanceFilterOptions {
  threshold: number;
}

/** Bounds for a filter's user-adjustable numeric threshold. */
export interface MaintenanceThresholdConfig {
  labelKey: string;
  defaultValue: number;
  min: number;
  step: number;
}

export interface MaintenanceBookFilterConfig {
  titleKey: string;
  emptyMessageKey: string;
  predicate: (book: Book, options: MaintenanceFilterOptions) => boolean;
  /** Present when the filter exposes a threshold control in the page toolbar. */
  threshold?: MaintenanceThresholdConfig;
  /** Set when the predicate reads char_count, which the API only returns on request. */
  requiresCharCount?: boolean;
  filterDescriptionKey?: string;
}

function isNonEmptyString(value: unknown): boolean {
  return typeof value === 'string' && value.trim().length > 0;
}

function hasOwn(obj: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(obj, key);
}

function normalizeBoolean(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') {
    return value;
  }

  if (typeof value === 'number') {
    return value > 0;
  }

  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true' || normalized === '1' || normalized === 'yes') {
      return true;
    }
    if (normalized === 'false' || normalized === '0' || normalized === 'no' || normalized === '') {
      return false;
    }
    return normalized.length > 0;
  }

  return undefined;
}

export function isMissingAuthor(book: Book): boolean {
  const raw = book as Book & { author?: unknown; authors?: unknown };
  const authorValue = raw.author ?? raw.authors;

  if (authorValue === undefined || authorValue === null) {
    return true;
  }

  if (typeof authorValue === 'string') {
    return authorValue.trim().length === 0;
  }

  if (Array.isArray(authorValue)) {
    if (authorValue.length === 0) {
      return true;
    }

    return authorValue.every((item) => typeof item !== 'string' || item.trim().length === 0);
  }

  return true;
}

export function hasBookCover(book: Book): boolean {
  const raw = book as Book & Record<string, unknown>;

  if (hasOwn(raw, 'has_cover')) {
    const normalized = normalizeBoolean(raw.has_cover);
    if (normalized !== undefined) {
      return normalized;
    }
  }

  if (hasOwn(raw, 'hasCover')) {
    const normalized = normalizeBoolean(raw.hasCover);
    if (normalized !== undefined) {
      return normalized;
    }
  }

  if (isNonEmptyString(raw.cover_url)) {
    return true;
  }

  if (isNonEmptyString(raw.coverUrl)) {
    return true;
  }

  if (isNonEmptyString(raw.cover)) {
    return true;
  }

  return false;
}

export function isMissingCover(book: Book): boolean {
  return !hasBookCover(book);
}

export function isMissingLanguage(book: Book): boolean {
  const raw = book as Book & { language?: unknown };
  const languageValue = raw.language;

  if (languageValue === undefined || languageValue === null) {
    return true;
  }

  if (typeof languageValue !== 'string') {
    return true;
  }

  return languageValue.trim().length === 0;
}

export const LOW_CHAR_COUNT_THRESHOLD: MaintenanceThresholdConfig = {
  labelKey: 'maintenance.lowCharCount.thresholdLabel',
  defaultValue: 1000,
  min: 0,
  step: 100
};

/**
 * The backend omits char_count when the source cannot be read, and `omitempty`
 * also drops a genuine 0, so an absent count is treated as 0: both cases are
 * exactly what this maintenance page exists to surface.
 */
export function isLowCharCount(book: Book, options: MaintenanceFilterOptions): boolean {
  return (book.char_count ?? 0) <= options.threshold;
}

export function hasUnknownCharCount(book: Book): boolean {
  return typeof book.char_count !== 'number';
}

/** Parses a raw query value into a usable threshold, falling back to the default. */
export function clampThreshold(raw: string | undefined, config: MaintenanceThresholdConfig): number {
  if (raw === undefined) {
    return config.defaultValue;
  }

  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return config.defaultValue;
  }

  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed)) {
    return config.defaultValue;
  }

  return Math.min(Math.max(parsed, config.min), Number.MAX_SAFE_INTEGER);
}

export const MAINTENANCE_BOOK_FILTERS: Record<MaintenanceBookFilter, MaintenanceBookFilterConfig> = {
  'missing-author': {
    titleKey: 'maintenance.missingAuthor.title',
    emptyMessageKey: 'maintenance.missingAuthor.empty',
    predicate: isMissingAuthor
  },
  'missing-cover': {
    titleKey: 'maintenance.missingCover.title',
    emptyMessageKey: 'maintenance.missingCover.empty',
    predicate: isMissingCover
  },
  'missing-language': {
    titleKey: 'maintenance.missingLanguage.title',
    emptyMessageKey: 'maintenance.missingLanguage.empty',
    predicate: isMissingLanguage
  },
  'low-char-count': {
    titleKey: 'maintenance.lowCharCount.title',
    emptyMessageKey: 'maintenance.lowCharCount.empty',
    predicate: isLowCharCount,
    threshold: LOW_CHAR_COUNT_THRESHOLD,
    requiresCharCount: true,
    filterDescriptionKey: 'maintenance.lowCharCount.filterDescription'
  }
};
