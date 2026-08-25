import {
  READER_BLOCK_CHAR_BUDGET,
  READER_BLOCK_LINE_BUDGET
} from '@/utils/renderMarkdownBlocks';

export type ReaderTextBlock = {
  type: 'paragraph' | 'quote';
  text: string;
  /**
   * False when this block is a continuation chunk of a larger block that was
   * split only for virtualization. Such a chunk renders with no trailing gap so
   * the split stays invisible. Absent (spaced) for ordinary paragraph blocks.
   */
  spacedAfter?: boolean;
};

/**
 * A plain-text section with no blank lines collapses to one enormous block, so
 * cut such a block into virtualization chunks at line boundaries — the same
 * char/line budget the Markdown path uses. A line longer than the budget stays
 * whole (never cut mid-line), and every chunk but the last is marked seamless so
 * the wall of text reads exactly as before.
 */
function splitOversizedTextBlock(block: ReaderTextBlock): ReaderTextBlock[] {
  const lines = block.text.split('\n');
  if (lines.length < READER_BLOCK_LINE_BUDGET && block.text.length < READER_BLOCK_CHAR_BUDGET) {
    return [block];
  }

  const chunks: ReaderTextBlock[] = [];
  let start = 0;
  let chars = 0;
  for (let i = 0; i < lines.length; i += 1) {
    chars += lines[i].length + 1;
    const lineCount = i - start + 1;
    const atBudget = chars >= READER_BLOCK_CHAR_BUDGET || lineCount >= READER_BLOCK_LINE_BUDGET;
    if (atBudget && i < lines.length - 1) {
      chunks.push({ type: block.type, text: lines.slice(start, i + 1).join('\n'), spacedAfter: false });
      start = i + 1;
      chars = 0;
    }
  }
  chunks.push({ type: block.type, text: lines.slice(start).join('\n') });
  return chunks;
}

export function parseReaderBlocks(text: string): ReaderTextBlock[] {
  if (!text.trim()) {
    return [];
  }

  return text
    .split(/\n{2,}/)
    .map((chunk) => chunk.trim())
    .filter(Boolean)
    .map((chunk): ReaderTextBlock => {
      const quoteLines = chunk
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean);
      const isQuote = quoteLines.length > 0 && quoteLines.every((line) => line.startsWith('>'));

      if (!isQuote) {
        return {
          type: 'paragraph',
          text: chunk
        };
      }

      return {
        type: 'quote',
        text: quoteLines.map((line) => line.replace(/^>\s?/, '')).join('\n')
      };
    })
    .flatMap(splitOversizedTextBlock);
}
