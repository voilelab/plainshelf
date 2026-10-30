import type { ReaderTextBlock } from '@/features/reader/utils/parseReaderBlocks';
import type { ReaderMarkdownBlock } from '@/utils/renderMarkdownBlocks';

const PROSE_LINE_PX = 38;
const CODE_LINE_PX = 26;
const BLOCK_MARGIN_PX = 18;
const CODE_PADDING_PX = 24;
const CHARS_PER_LINE = 40;
const IMAGE_RESERVE_PX = 320;

const CODE_BLOCK_RE = /<pre class="reader-md-code"><code>([\s\S]*?)<\/code><\/pre>/g;

function wrappedProseHeight(chars: number): number {
  if (chars <= 0) return 0;
  const lines = Math.max(1, Math.ceil(chars / CHARS_PER_LINE));
  return lines * PROSE_LINE_PX + BLOCK_MARGIN_PX;
}

export function estimateMarkdownBlockHeight(block: ReaderMarkdownBlock): number {
  let height = block.images.length * IMAGE_RESERVE_PX;

  const withoutCode = block.html.replace(CODE_BLOCK_RE, (_match, code: string) => {
    height += code.split('\n').length * CODE_LINE_PX + CODE_PADDING_PX;
    return '';
  });

  const prose = withoutCode.replace(/<[^>]+>/g, '').replace(/\s+/g, ' ').trim();
  height += wrappedProseHeight(prose.length);

  return Math.max(height, PROSE_LINE_PX);
}

export function estimateTextBlockHeight(block: ReaderTextBlock): number {
  return Math.max(wrappedProseHeight(block.text.length), PROSE_LINE_PX);
}
