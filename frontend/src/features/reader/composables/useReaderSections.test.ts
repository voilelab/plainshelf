import { describe, expect, it } from 'vitest';
import { buildLegacyReaderSections } from './useReader';

describe('legacy reader sections', () => {
  const content = '## One\na\nb\n## Two\nc';

  it('preserves none, line-count, boundary, and regex split behavior', () => {
    expect(buildLegacyReaderSections(content, { type: 'none' })).toHaveLength(1);
    expect(buildLegacyReaderSections(content, { type: 'line_count', line_count: 2 }).map((s) => s.startOffset))
      .toEqual([0, 9, 18]);
    expect(buildLegacyReaderSections(content, { type: 'boundary', boundaries: [3, 5] }).map((s) => s.startOffset))
      .toEqual([0, 9, 18]);
    expect(buildLegacyReaderSections(content, { type: 'regex', regex: '^## ' }).map((s) => s.title))
      .toEqual(['One', 'Two']);
  });
});
