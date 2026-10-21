import MarkdownIt from 'markdown-it';

export interface MarkdownAssetImage {
  name: string;
  alt: string;
}

const IMAGE_LINE_RE = /^!\[([^\]]*)\]\((.*)\)$/;

const ASSET_SRC_RE = /^assets\/([^/\\]+)$/;
const ASSET_EXT_RE = /\.(jpe?g|png|webp|gif)$/i;

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

export function assetNameFromSrc(src: string): string | null {
  const match = ASSET_SRC_RE.exec(normalizeDestination(src));
  if (!match) return null;

  const name = match[1];
  if (name.startsWith('.') || !ASSET_EXT_RE.test(name)) return null;
  return name;
}

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

export function referencedAssetNames(text: string): string[] {
  const { images } = rewriteMarkdownAssetImages(text);
  const seen = new Set<string>();
  return images.flatMap(({ name }) => {
    if (seen.has(name)) return [];
    seen.add(name);
    return [name];
  });
}
