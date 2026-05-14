export type ReaderTextBlock = {
  type: 'paragraph' | 'quote';
  text: string;
};

export function parseReaderBlocks(text: string): ReaderTextBlock[] {
  if (!text.trim()) {
    return [];
  }

  return text
    .split(/\n{2,}/)
    .map((chunk) => chunk.trim())
    .filter(Boolean)
    .map((chunk) => {
      const quoteLines = chunk
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean);
      const isQuote = quoteLines.length > 0 && quoteLines.every((line) => line.startsWith('>'));

      if (!isQuote) {
        return {
          type: 'paragraph' as const,
          text: chunk
        };
      }

      return {
        type: 'quote' as const,
        text: quoteLines.map((line) => line.replace(/^>\s?/, '')).join('\n')
      };
    });
}
