import MarkdownIt from 'markdown-it';

export interface MarkdownAssetImage {
  /** File name inside the source's `assets/` directory, never a path. */
  name: string;
  alt: string;
}

// Only a line that is nothing but an image becomes an illustration. Inline
// occurrences stay readable text rather than being pulled out of their prose.
const IMAGE_LINE_RE = /^!\[([^\]]*)\]\((.*)\)$/;

// A source asset is one supported image directly inside assets/. URL-bearing,
// nested, hidden, and traversal targets are deliberately rejected.
const ASSET_SRC_RE = /^assets\/([^/\\]+)$/;
const ASSET_EXT_RE = /\.(jpe?g|png|webp|gif)$/i;

// Source maps from the real Markdown parser tell us which physical lines are
// Markdown inline content. That excludes fenced/indented code and raw HTML
// blocks such as <pre>, while still including list continuations and the body
// of an HTML container after a blank line. Rendering and offline collection
// both consume this exact classification.
const markdownAnalyzer = new MarkdownIt({
  html: true,
  linkify: false,
  typographer: false
});

function normalizeDestination(raw: string): string {
  let dest = raw.trim();
  if (dest.startsWith('<') && dest.endsWith('>')) {
    dest = dest.slice(1, -1).trim();
  }

  try {
    return decodeURIComponent(dest);
  } catch {
    return dest;
  }
}

/** Returns a safe flat source-asset file name, or null for every other URL. */
export function assetNameFromSrc(src: string): string | null {
  const match = ASSET_SRC_RE.exec(normalizeDestination(src));
  if (!match) return null;

  const name = match[1];
  if (name.startsWith('.') || !ASSET_EXT_RE.test(name)) return null;
  return name;
}

/** Parses the deliberately narrow standalone Markdown image form. */
export function assetImageFromMarkdownLine(rawLine: string): MarkdownAssetImage | null {
  const match = IMAGE_LINE_RE.exec(rawLine.trim());
  if (!match) return null;
  const name = assetNameFromSrc(match[2]);
  return name ? { name, alt: match[1].trim() } : null;
}

function markdownInlineLineIndexes(text: string): Set<number> {
  const indexes = new Set<number>();
  for (const token of markdownAnalyzer.parse(text, {})) {
    if (token.type !== 'inline' || !token.map) continue;
    for (let line = token.map[0]; line < token.map[1]; line += 1) {
      indexes.add(line);
    }
  }
  return indexes;
}

interface RewrittenMarkdownAssets {
  text: string;
  images: MarkdownAssetImage[];
}

/**
 * Rewrites only local image lines that Markdown would render as inline
 * content, returning their metadata in the same order. The callback lets the
 * reader replace them with inert component slots while offline collection can
 * inspect the identical image list without maintaining a second parser.
 */
export function rewriteMarkdownAssetImages(
  text: string,
  rewrite: (image: MarkdownAssetImage, index: number, rawLine: string) => string = (_image, _index, rawLine) => rawLine
): RewrittenMarkdownAssets {
  const inlineLines = markdownInlineLineIndexes(text);
  const images: MarkdownAssetImage[] = [];
  const lines = text.split(/\r?\n/);

  const rewritten = lines.map((rawLine, lineIndex) => {
    if (!inlineLines.has(lineIndex)) return rawLine;
    const image = assetImageFromMarkdownLine(rawLine);
    if (!image) return rawLine;

    const imageIndex = images.length;
    images.push(image);
    return rewrite(image, imageIndex, rawLine);
  });

  return { text: rewritten.join('\n'), images };
}

/** Lists rendered local illustrations once, in first-use order. */
export function referencedAssetNames(text: string): string[] {
  const { images } = rewriteMarkdownAssetImages(text);
  const seen = new Set<string>();
  return images.flatMap(({ name }) => {
    if (seen.has(name)) return [];
    seen.add(name);
    return [name];
  });
}
