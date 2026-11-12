const LISTED_ENTRY_LIMIT = 10;

export function splitListedEntries(entries: string[]): { shown: string[]; hidden: number } {
  const shown = entries.length > LISTED_ENTRY_LIMIT ? entries.slice(0, LISTED_ENTRY_LIMIT - 1) : entries;
  return { shown, hidden: entries.length - shown.length };
}
