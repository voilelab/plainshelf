import type { Book } from '../types/book';
import type { SidebarNavIconName } from '../types/sidebarNavIcon';

export type MaintenanceNavKey =
  | 'duplicate-content'
  | 'missing-author'
  | 'missing-cover'
  | 'missing-language';

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
  }
];

export type MaintenanceBookFilter = Exclude<MaintenanceNavKey, 'duplicate-content'>;

interface MaintenanceBookFilterConfig {
  titleKey: string;
  emptyMessageKey: string;
  predicate: (book: Book) => boolean;
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
  }
};
