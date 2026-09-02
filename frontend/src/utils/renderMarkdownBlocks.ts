import MarkdownIt from 'markdown-it';
import { BOOK_TEXT_MARKDOWN_OPTIONS } from './safeHtml';
import {
  updateMarkdownFenceState,
  type MarkdownFenceState
} from './markdownLineSyntax';
import {
  rewriteMarkdownAssetImages,
  type MarkdownAssetImage
} from './markdownAssetImages';

export type ReaderMarkdownAsset = MarkdownAssetImage & {
  /** Opaque renderer-owned marker used to locate the sanitized component slot. */
  token: string;
};

type ReaderMarkdownHtmlBlock = {
  type: 'html';
  /** Unsanitized renderer output. SafeHtml is the only permitted sink. */
  html: string;
  images: ReaderMarkdownAsset[];
};

export type ReaderMarkdownBlock = ReaderMarkdownHtmlBlock;

// The dialect is the one a description is parsed in; the rules below are what
// the reader adds on top, which is why the instance is its own.
const markdown = new MarkdownIt(BOOK_TEXT_MARKDOWN_OPTIONS);

// Links were not part of the old reader and could navigate away. Images stay
// enabled only so renderer-owned synthetic targets can become inert component
// slots; every source-authored target is emitted as literal text below.
markdown.disable(['link', 'autolink']);

type RenderTokens = Parameters<typeof markdown.renderer.renderToken>[0];

function renderToken(tokens: RenderTokens, index: number): string {
  return markdown.renderer.renderToken(tokens, index, markdown.options);
}

interface ReaderMarkdownEnvironment {
  [key: string]: unknown;
  [key: symbol]: unknown;
  assetTokenPrefix: string;
  images: ReaderMarkdownAsset[];
}

function readerEnvironment(env: unknown): ReaderMarkdownEnvironment {
  if (
    env &&
    typeof env === 'object' &&
    typeof Reflect.get(env, 'assetTokenPrefix') === 'string' &&
    Array.isArray(Reflect.get(env, 'images'))
  ) {
    return env as ReaderMarkdownEnvironment;
  }
  return { assetTokenPrefix: '', images: [] };
}

function assetFromToken(
  rawSrc: string | number | null,
  env: ReaderMarkdownEnvironment
): ReaderMarkdownAsset | null {
  const src = rawSrc === null ? '' : String(rawSrc);
  if (!env.assetTokenPrefix || !src.startsWith(env.assetTokenPrefix)) return null;
  const index = Number(src.slice(env.assetTokenPrefix.length));
  return Number.isInteger(index) ? env.images[index] ?? null : null;
}

function paragraphContainsOnlyAsset(
  tokens: RenderTokens,
  index: number,
  env: ReaderMarkdownEnvironment,
  inlineOffset: -1 | 1
): boolean {
  const inline = tokens[index + inlineOffset];
  const children = inline?.type === 'inline' ? inline.children : null;
  if (!children || children.length !== 1 || children[0].type !== 'image') return false;
  return assetFromToken(children[0].attrGet('src'), env) !== null;
}

markdown.renderer.rules.heading_open = (tokens, index) => {
  const sourceLevel = Number(tokens[index].tag.slice(1));
  tokens[index].tag = sourceLevel === 1 ? 'h2' : sourceLevel === 2 ? 'h3' : 'h4';
  tokens[index].attrSet(
    'class',
    sourceLevel === 1 ? 'reader-md-h1' : sourceLevel === 2 ? 'reader-md-h2' : 'reader-md-h3'
  );
  return renderToken(tokens, index);
};

markdown.renderer.rules.heading_close = (tokens, index) => {
  const sourceLevel = Number(tokens[index].tag.slice(1));
  tokens[index].tag = sourceLevel === 1 ? 'h2' : sourceLevel === 2 ? 'h3' : 'h4';
  return renderToken(tokens, index);
};

markdown.renderer.rules.paragraph_open = (tokens, index, _options, env) => {
  if (paragraphContainsOnlyAsset(tokens, index, readerEnvironment(env), 1)) return '';
  tokens[index].attrJoin('class', 'reader-text-block');
  return renderToken(tokens, index);
};

markdown.renderer.rules.paragraph_close = (tokens, index, _options, env) =>
  paragraphContainsOnlyAsset(tokens, index, readerEnvironment(env), -1) ? '' : renderToken(tokens, index);

markdown.renderer.rules.blockquote_open = (tokens, index) => {
  tokens[index].attrJoin('class', 'reader-text-block reader-text-quote');
  return renderToken(tokens, index);
};

for (const rule of ['bullet_list_open', 'ordered_list_open'] as const) {
  markdown.renderer.rules[rule] = (tokens, index) => {
    tokens[index].attrJoin('class', 'reader-md-list');
    return renderToken(tokens, index);
  };
}

markdown.renderer.rules.fence = (tokens, index) => {
  const content = markdown.utils.escapeHtml(tokens[index].content);
  return `<pre class="reader-md-code"><code>${content}</code></pre>\n`;
};

markdown.renderer.rules.code_block = markdown.renderer.rules.fence;

markdown.renderer.rules.code_inline = (tokens, index) =>
  `<code class="reader-md-inline-code">${markdown.utils.escapeHtml(tokens[index].content)}</code>`;

markdown.renderer.rules.hr = () => '<hr class="reader-md-hr">\n';

markdown.renderer.rules.image = (tokens, index, _options, rawEnv) => {
  const env = readerEnvironment(rawEnv);
  const token = tokens[index];
  const rawSrc = token.attrGet('src');
  const src = rawSrc === null ? '' : String(rawSrc);
  const asset = assetFromToken(rawSrc, env);
  if (asset) {
    return `<span class="reader-asset-slot" title="${markdown.utils.escapeHtml(asset.token)}"></span>`;
  }

  // No source-authored image reaches an <img> sink. Keeping unsupported and
  // inline spellings visible also preserves the reader's previous behavior.
  return markdown.utils.escapeHtml(`![${token.content}](${src})`);
};

// plot is a transparent authoring wrapper, not an element in the rendered
// document. Removing standalone markers before Markdown parsing lets the prose
// inside become ordinary paragraphs instead of relying on white-space CSS to
// preserve the raw HTML block's text nodes.
const PLOT_MARKER_LINE_RE = /^\s*<\/?plot(?:\s[^>]*)?>\s*$/i;

function renderHtmlBlock(
  source: string,
  env: ReaderMarkdownEnvironment
): ReaderMarkdownHtmlBlock | null {
  if (!source.trim()) return null;
  const html = markdown.render(source, env);
  if (!html.trim()) return null;
  // Split rendering emits one block per chunk, so a block must carry only the
  // illustrations whose slot it actually rendered — not the whole document's
  // asset list — or every block would re-scan every book image.
  const images = env.images.filter((image) => html.includes(`title="${image.token}"`));
  return { type: 'html', html, images };
}

/** Character budget after which a section is cut at the next block boundary. */
export const READER_BLOCK_CHAR_BUDGET = 2000;
/** Line budget after which a section is cut at the next block boundary. */
export const READER_BLOCK_LINE_BUDGET = 80;

/**
 * Cuts an oversized section into render chunks at top-level Markdown block
 * boundaries. Each markdown-it block token carries its own `[startLine,
 * endLine)` span, so a fenced code block, list, blockquote or table is always
 * kept whole: a cut never lands inside a multi-line construct, and an ordered
 * list is never split into separately renumbered halves. A single block larger
 * than the budget becomes its own chunk instead of being cut mid-line.
 *
 * The chunk boundaries are a render concern only. They carry no offset meaning,
 * so the reader's char-offset progress bridge is unaffected by how a section is
 * divided here (see `.claude/rules/50-lessons.md` on `markdownChapters.ts`).
 */
function splitSourceIntoRenderChunks(
  source: string,
  env: ReaderMarkdownEnvironment
): string[] {
  const lines = source.split('\n');
  const blocks: Array<{ start: number; end: number }> = [];
  for (const token of markdown.parse(source, env)) {
    // A level-0 opening or self-closing token spans one whole top-level block;
    // its close token (nesting -1) repeats the range, so it is skipped.
    if (token.level === 0 && token.map && token.nesting !== -1) {
      blocks.push({ start: token.map[0], end: token.map[1] });
    }
  }
  if (blocks.length <= 1) {
    return [source];
  }

  const chunks: string[] = [];
  let start = -1;
  let end = 0;
  let chars = 0;
  let lineCount = 0;

  const flush = (): void => {
    if (start < 0) return;
    chunks.push(lines.slice(start, end).join('\n'));
    start = -1;
    chars = 0;
    lineCount = 0;
  };

  for (const block of blocks) {
    if (start < 0) start = block.start;
    end = block.end;
    chars += lines.slice(block.start, block.end).join('\n').length;
    lineCount += block.end - block.start;
    if (chars >= READER_BLOCK_CHAR_BUDGET || lineCount >= READER_BLOCK_LINE_BUDGET) {
      flush();
    }
  }
  flush();
  return chunks;
}

let assetDocumentSerial = 0;

function nextAssetTokenPrefix(): string {
  assetDocumentSerial = (assetDocumentSerial + 1) % Number.MAX_SAFE_INTEGER;
  return `plainshelf-reader-asset-${assetDocumentSerial}-`;
}

function removePlotMarkers(source: string): string {
  let fence: MarkdownFenceState | null = null;
  return source.split(/\r?\n/).map((line) => {
    const transition = updateMarkdownFenceState(line, fence);
    const canRemove = !fence && !transition.boundary;
    if (transition.boundary) fence = transition.state;
    return canRemove && PLOT_MARKER_LINE_RE.test(line) ? '' : line;
  }).join('\n');
}

/**
 * Produces reader blocks while keeping local illustrations as Vue components.
 * All other Markdown, including raw HTML, remains an untrusted HTML string and
 * must pass through SafeHtml before it reaches the DOM.
 */
export function renderMarkdownBlocks(source: string): ReaderMarkdownBlock[] {
  if (!source.trim()) return [];

  const assetTokenPrefix = nextAssetTokenPrefix();
  const rewritten = rewriteMarkdownAssetImages(source, (_image, index, rawLine) => {
    const indent = rawLine.match(/^[ \t]*/)?.[0] ?? '';
    return `${indent}![](${assetTokenPrefix}${index})`;
  });
  const images = rewritten.images.map((image, index) => ({
    ...image,
    token: `${assetTokenPrefix}${index}`
  }));
  const env: ReaderMarkdownEnvironment = { assetTokenPrefix, images };
  const prepared = removePlotMarkers(rewritten.text);

  const blocks: ReaderMarkdownBlock[] = [];
  for (const chunk of splitSourceIntoRenderChunks(prepared, env)) {
    // A fresh env per chunk keeps markdown-it's per-document state (e.g. link
    // reference definitions) from bleeding between chunks; the shared `images`
    // array is read-only during rendering, so reusing it is safe.
    const block = renderHtmlBlock(chunk, { assetTokenPrefix, images });
    if (block) blocks.push(block);
  }
  return blocks;
}
