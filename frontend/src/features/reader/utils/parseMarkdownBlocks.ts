export type InlineSegment = {
  text: string;
  bold?: boolean;
  italic?: boolean;
  code?: boolean;
};

export type MarkdownHeadingBlock = {
  type: 'heading';
  level: 1 | 2 | 3 | 4 | 5 | 6;
  segments: InlineSegment[];
};

export type MarkdownParagraphBlock = {
  type: 'paragraph';
  segments: InlineSegment[];
};

export type MarkdownQuoteBlock = {
  type: 'quote';
  segments: InlineSegment[];
};

export type MarkdownListBlock = {
  type: 'list';
  ordered: boolean;
  items: InlineSegment[][];
};

export type MarkdownCodeBlock = {
  type: 'code';
  text: string;
};

export type MarkdownHrBlock = {
  type: 'hr';
};

export type MarkdownBlock =
  | MarkdownHeadingBlock
  | MarkdownParagraphBlock
  | MarkdownQuoteBlock
  | MarkdownListBlock
  | MarkdownCodeBlock
  | MarkdownHrBlock;

const HR_RE = /^(-{3,}|\*{3,})$/;
const HEADING_RE = /^(#{1,6})\s+(.+)$/;
const UNORDERED_ITEM_RE = /^[-*]\s+(.*)$/;
const ORDERED_ITEM_RE = /^\d+\.\s+(.*)$/;
const INLINE_RE = /`([^`]+)`|\*\*([^*]+?)\*\*|\*([^*]+?)\*/g;
const FENCE_RE = /^\s{0,3}```/;
const LEADING_INDENT_RE = /^[ \t]+\S/;

function parseInlineSegments(text: string): InlineSegment[] {
  const segments: InlineSegment[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  INLINE_RE.lastIndex = 0;
  while ((match = INLINE_RE.exec(text)) !== null) {
    if (match.index > lastIndex) {
      segments.push({ text: text.slice(lastIndex, match.index) });
    }

    if (match[1] !== undefined) {
      segments.push({ text: match[1], code: true });
    } else if (match[2] !== undefined) {
      segments.push({ text: match[2], bold: true });
    } else if (match[3] !== undefined) {
      segments.push({ text: match[3], italic: true });
    }

    lastIndex = INLINE_RE.lastIndex;
  }

  if (lastIndex < text.length) {
    segments.push({ text: text.slice(lastIndex) });
  }

  if (segments.length === 0) {
    segments.push({ text: '' });
  }

  return segments;
}

function parseTextSegmentToBlocks(text: string): MarkdownBlock[] {
  const lines = text.split('\n');
  const blocks: MarkdownBlock[] = [];

  let paragraphLines: string[] = [];
  let quoteLines: string[] = [];
  let listBlock: { ordered: boolean; items: string[] } | null = null;

  const flushParagraph = (): void => {
    if (paragraphLines.length > 0) {
      blocks.push({ type: 'paragraph', segments: parseInlineSegments(paragraphLines.join('\n')) });
      paragraphLines = [];
    }
  };

  const flushQuote = (): void => {
    if (quoteLines.length > 0) {
      blocks.push({ type: 'quote', segments: parseInlineSegments(quoteLines.join('\n')) });
      quoteLines = [];
    }
  };

  const flushList = (): void => {
    if (listBlock) {
      blocks.push({
        type: 'list',
        ordered: listBlock.ordered,
        items: listBlock.items.map((item) => parseInlineSegments(item))
      });
      listBlock = null;
    }
  };

  const flushAll = (): void => {
    flushParagraph();
    flushQuote();
    flushList();
  };

  for (const rawLine of lines) {
    if (rawLine.trim() === '') {
      flushAll();
      continue;
    }

    if (listBlock && LEADING_INDENT_RE.test(rawLine)) {
      const idx = listBlock.items.length - 1;
      if (idx >= 0) {
        listBlock.items[idx] = `${listBlock.items[idx]} ${rawLine.trim()}`.trim();
        continue;
      }
    }

    const trimmed = rawLine.trim();

    if (HR_RE.test(trimmed)) {
      flushAll();
      blocks.push({ type: 'hr' });
      continue;
    }

    const headingMatch = HEADING_RE.exec(trimmed);
    if (headingMatch) {
      flushAll();
      const level = headingMatch[1].length as 1 | 2 | 3 | 4 | 5 | 6;
      blocks.push({ type: 'heading', level, segments: parseInlineSegments(headingMatch[2].trim()) });
      continue;
    }

    if (trimmed.startsWith('>')) {
      flushParagraph();
      flushList();
      quoteLines.push(trimmed.replace(/^>\s?/, ''));
      continue;
    }

    const unorderedMatch = UNORDERED_ITEM_RE.exec(trimmed);
    const orderedMatch = unorderedMatch ? null : ORDERED_ITEM_RE.exec(trimmed);
    if (unorderedMatch || orderedMatch) {
      flushParagraph();
      flushQuote();
      const ordered = Boolean(orderedMatch);
      const itemText = unorderedMatch ? unorderedMatch[1] : (orderedMatch as RegExpExecArray)[1];
      if (!listBlock || listBlock.ordered !== ordered) {
        flushList();
        listBlock = { ordered, items: [] };
      }
      listBlock.items.push(itemText);
      continue;
    }

    flushQuote();
    flushList();
    paragraphLines.push(trimmed);
  }

  flushAll();
  return blocks;
}

export function parseMarkdownBlocks(text: string): MarkdownBlock[] {
  if (!text.trim()) {
    return [];
  }

  const lines = text.split('\n');
  const blocks: MarkdownBlock[] = [];
  let textBuffer: string[] = [];
  let i = 0;

  const flushTextBuffer = (): void => {
    if (textBuffer.length > 0) {
      blocks.push(...parseTextSegmentToBlocks(textBuffer.join('\n')));
      textBuffer = [];
    }
  };

  while (i < lines.length) {
    const line = lines[i];
    if (FENCE_RE.test(line)) {
      flushTextBuffer();
      const codeLines: string[] = [];
      i += 1;
      while (i < lines.length && !FENCE_RE.test(lines[i])) {
        codeLines.push(lines[i]);
        i += 1;
      }
      if (i < lines.length) {
        i += 1;
      }
      blocks.push({ type: 'code', text: codeLines.join('\n') });
      continue;
    }

    textBuffer.push(line);
    i += 1;
  }

  flushTextBuffer();
  return blocks;
}
