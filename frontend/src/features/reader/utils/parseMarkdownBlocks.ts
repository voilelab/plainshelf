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
const HEADING_RE = /^(#{1,6})\s+(.+)$/;
const UNORDERED_ITEM_RE = /^[-*]\s+(.*)$/;
const ORDERED_ITEM_RE = /^\d+\.\s+(.*)$/;
const INLINE_RE = /`([^`]+)`|\*\*([^*]+?)\*\*|\*([^*]+?)\*/g;
const FENCE_RE = /^\s{0,3}```/;
const LEADING_INDENT_RE = /^[ \t]+\S/;

// Only a line that is nothing but an image becomes an image block. An
// illustration in a book is a block, and treating `![]()` inside a sentence as
// one would mean threading images through InlineSegment for no reader benefit;
// inline occurrences stay literal text.
//
// The destination is captured loosely because the line is anchored at both
// ends: whatever sits between the parentheses is the target, spaces included.
// A file named "A Map.png" is ordinary on a shelf people edit by hand, and
// requiring it to be percent-encoded would make the shelf less writable by
// hand than the format promises.
const IMAGE_LINE_RE = /^!\[([^\]]*)\]\((.*)\)$/;

// A source asset is addressed as a single file inside `assets/`, matching the
// flat directory the server serves from (see shelf/asset.go). Anything else --
// a nested path, `..`, an absolute path, or an external URL -- is not loaded.
// Refusing `http(s):` and `data:` here is deliberate: a book's text must not be
// able to make the reader fetch from the network.
const ASSET_SRC_RE = /^assets\/([^/\\]+)$/;

// Kept in step with shelf.IsSupportedImageExt; a mismatch only costs a failed
// request that falls back to alt text, so the server stays the authority.
const ASSET_EXT_RE = /\.(jpe?g|png|webp|gif)$/i;

/**
 * Normalizes a Markdown link destination to the path it names.
 *
 * Both spellings a hand-edited shelf produces are accepted: the angle-bracket
 * form CommonMark provides for destinations containing spaces, and percent
 * escapes. Decoding happens before validation, never after, so an escaped
 * separator cannot slip a path segment past the checks below.
 */
function normalizeDestination(raw: string): string {
  let dest = raw.trim();
  if (dest.startsWith('<') && dest.endsWith('>')) {
    dest = dest.slice(1, -1).trim();
  }

  try {
    return decodeURIComponent(dest);
  } catch {
    // A malformed escape is not a decoding failure worth reporting; leave the
    // text as written and let the checks below reject it.
    return dest;
  }
}

/**
 * Returns the asset file name a Markdown image target refers to, or null when
 * the target is not an illustration this reader will load.
 */
export function assetNameFromSrc(src: string): string | null {
  const match = ASSET_SRC_RE.exec(normalizeDestination(src));
  if (!match) {
    return null;
  }

  const name = match[1];
  if (name.startsWith('.') || !ASSET_EXT_RE.test(name)) {
    return null;
  }

  return name;
}

/**
 * Lists the source assets a Markdown text actually renders, in first-use order
 * and without duplicates.
 *
 * This runs the real parser rather than its own scan, so an offline download
 * fetches exactly the files the reader will ask for: a link the reader leaves
 * as text is not downloaded, and one it renders is never missed.
 */
export function referencedAssetNames(text: string): string[] {
  const names: string[] = [];
  const seen = new Set<string>();

  for (const block of parseMarkdownBlocks(text)) {
    if (block.type === 'image' && !seen.has(block.name)) {
      seen.add(block.name);
      names.push(block.name);
    }
  }

  return names;
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

    const headingMatch = HEADING_RE.exec(trimmed);
    if (headingMatch) {
      flushAll();
      const level = headingMatch[1].length as 1 | 2 | 3 | 4 | 5 | 6;
      blocks.push({ type: 'heading', level, segments: parseInlineSegments(headingMatch[2].trim()) });
      continue;
    }

    // An image target this reader will not load falls through to the paragraph
    // path, so the line stays visible as its own Markdown rather than silently
    // disappearing.
    const imageMatch = IMAGE_LINE_RE.exec(trimmed);
    if (imageMatch) {
      const name = assetNameFromSrc(imageMatch[2]);
      if (name) {
        flushAll();
        blocks.push({ type: 'image', name, alt: imageMatch[1].trim() });
        continue;
      }
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
