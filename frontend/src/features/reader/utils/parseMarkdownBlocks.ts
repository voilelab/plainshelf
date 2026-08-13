import {
  assetImageFromMarkdownLine,
  assetNameFromSrc,
  referencedAssetNames
} from './markdownAssetImages';

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

export type MarkdownImageBlock = {
  type: 'image';
  /** File name inside the source's `assets/` directory, never a path. */
  name: string;
  alt: string;
};

export type MarkdownBlock =
  | MarkdownHeadingBlock
  | MarkdownParagraphBlock
  | MarkdownQuoteBlock
  | MarkdownListBlock
  | MarkdownCodeBlock
  | MarkdownHrBlock
  | MarkdownImageBlock;

const HR_RE = /^(-{3,}|\*{3,})$/;
const HEADING_RE = /^ {0,3}(#{1,6})[ \t]+(.+)$/;
const UNORDERED_ITEM_RE = /^[-*]\s+(.*)$/;
const ORDERED_ITEM_RE = /^\d+\.\s+(.*)$/;
const INLINE_RE = /`([^`]+)`|\*\*([^*]+?)\*\*|\*([^*]+?)\*/g;
const FENCE_OPENER_RE = /^ {0,3}(`{3,}|~{3,})(.*)$/;
const LEADING_INDENT_RE = /^[ \t]+\S/;

export { assetImageFromMarkdownLine, assetNameFromSrc, referencedAssetNames };

export interface MarkdownHeadingLine {
  level: 1 | 2 | 3 | 4 | 5 | 6;
  title: string;
}

export interface MarkdownFenceState {
  marker: '`' | '~';
  length: number;
}

/** Shared line-level syntax used by both the renderer and chapter scanner. */
export function parseMarkdownHeadingLine(rawLine: string): MarkdownHeadingLine | null {
  const match = HEADING_RE.exec(rawLine);
  if (!match) {
    return null;
  }
  return {
    level: match[1].length as MarkdownHeadingLine['level'],
    title: match[2].trim()
  };
}

function markdownFenceOpener(rawLine: string): MarkdownFenceState | null {
  const match = FENCE_OPENER_RE.exec(rawLine);
  if (!match) return null;
  const run = match[1];
  // Backtick fence info strings cannot contain a backtick. Treating such a
  // line as text keeps the scanner and renderer aligned with CommonMark.
  if (run[0] === '`' && match[2].includes('`')) return null;
  return { marker: run[0] as MarkdownFenceState['marker'], length: run.length };
}

function closesMarkdownFence(rawLine: string, fence: MarkdownFenceState): boolean {
  const match = /^ {0,3}(`+|~+)[ \t]*$/.exec(rawLine);
  return Boolean(match && match[1][0] === fence.marker && match[1].length >= fence.length);
}

/**
 * Advances fenced-code state and reports whether this line is the compatible
 * opener or closer. Fence-like text inside a block is ordinary code unless it
 * uses the opener's marker with at least the opener's run length.
 */
export function updateMarkdownFenceState(
  rawLine: string,
  current: MarkdownFenceState | null
): { state: MarkdownFenceState | null; boundary: boolean } {
  if (current) {
    return closesMarkdownFence(rawLine, current)
      ? { state: null, boundary: true }
      : { state: current, boundary: false };
  }
  const opener = markdownFenceOpener(rawLine);
  return opener ? { state: opener, boundary: true } : { state: null, boundary: false };
}

/** A possible opening fence line; stateful consumers use updateMarkdownFenceState. */
export function isMarkdownFenceLine(rawLine: string): boolean {
  return markdownFenceOpener(rawLine) !== null;
}

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

    const heading = parseMarkdownHeadingLine(rawLine);
    if (heading) {
      flushAll();
      blocks.push({ type: 'heading', level: heading.level, segments: parseInlineSegments(heading.title) });
      continue;
    }

    // An image target this reader will not load falls through to the paragraph
    // path, so the line stays visible as its own Markdown rather than silently
    // disappearing.
    const image = assetImageFromMarkdownLine(trimmed);
    if (image) {
      flushAll();
      blocks.push({ type: 'image', ...image });
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

  // A source may preserve Windows CRLF line endings. Split both bytes here so
  // line-oriented Markdown syntax sees the same text it would in an LF file;
  // otherwise the trailing CR prevents anchored heading and fence regexes
  // from matching.
  const lines = text.split(/\r?\n/);
  const blocks: MarkdownBlock[] = [];
  let textBuffer: string[] = [];
  let i = 0;
  let fence: MarkdownFenceState | null = null;

  const flushTextBuffer = (): void => {
    if (textBuffer.length > 0) {
      blocks.push(...parseTextSegmentToBlocks(textBuffer.join('\n')));
      textBuffer = [];
    }
  };

  while (i < lines.length) {
    const line = lines[i];
    const transition = updateMarkdownFenceState(line, fence);
    if (!fence && transition.boundary && transition.state) {
      fence = transition.state;
      flushTextBuffer();
      const codeLines: string[] = [];
      i += 1;
      while (i < lines.length) {
        const innerTransition = updateMarkdownFenceState(lines[i], fence);
        if (innerTransition.boundary && !innerTransition.state) {
          fence = null;
          i += 1;
          break;
        }
        codeLines.push(lines[i]);
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
