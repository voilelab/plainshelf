import type { SplitConfig, SplitType } from '@/types/book';

/**
 * The read-history limit the user typed, or null when it is not a whole,
 * non-negative count. Zero is valid and means "keep no limit".
 */
export function parseReadHistoryLimit(value: string): number | null {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}

export interface SplitConfigDraft {
  type: SplitType;
  lineCount: string;
  regex: string;
}

export type SplitConfigDraftResult =
  | { ok: true; config: SplitConfig }
  | { ok: false; error: 'invalidLineCount' | 'invalidRegex' };

/**
 * The default split configuration described by the form draft. Only the field
 * belonging to the selected type is validated, so a leftover value in an unused
 * field never blocks saving. `error` names the message key to show.
 */
export function buildDefaultSplitConfig(draft: SplitConfigDraft): SplitConfigDraftResult {
  if (draft.type === 'line_count') {
    const parsed = Number.parseInt(draft.lineCount, 10);
    if (!Number.isInteger(parsed) || parsed <= 0) {
      return { ok: false, error: 'invalidLineCount' };
    }
    return { ok: true, config: { type: 'line_count', line_count: parsed } };
  }

  if (draft.type === 'regex') {
    try {
      new RegExp(draft.regex, 'gm');
    } catch {
      return { ok: false, error: 'invalidRegex' };
    }
    return { ok: true, config: { type: 'regex', regex: draft.regex } };
  }

  return { ok: true, config: { type: 'none' } };
}
