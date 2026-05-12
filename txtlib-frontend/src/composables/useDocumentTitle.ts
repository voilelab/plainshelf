import { toValue, watchEffect, type MaybeRefOrGetter } from 'vue';

const TITLE_SEPARATOR = ' · ';
const APP_TITLE = 'PlainShelf';

function normalizeTitleSegment(segment: unknown): string | null {
  if (segment == null) {
    return null;
  }

  const normalized = String(segment).trim();
  if (normalized === '') {
    return null;
  }

  const lowered = normalized.toLowerCase();
  if (lowered === 'undefined' || lowered === 'null') {
    return null;
  }

  return normalized;
}

export function buildDocumentTitle(segments: readonly unknown[]): string {
  const normalizedSegments = segments
    .map((segment) => normalizeTitleSegment(segment))
    .filter((segment): segment is string => segment !== null);

  return normalizedSegments.length > 0 ? normalizedSegments.join(TITLE_SEPARATOR) : APP_TITLE;
}

export function setDocumentTitle(segments: readonly unknown[]): void {
  document.title = buildDocumentTitle(segments);
}

export function useDocumentTitle(titleSource: MaybeRefOrGetter<readonly unknown[]>): void {
  watchEffect(() => {
    setDocumentTitle(toValue(titleSource));
  });
}

export { APP_TITLE };