import type { ReaderTextBlock } from '@/features/reader/utils/parseReaderBlocks';
import type { ReaderMarkdownBlock } from '@/utils/renderMarkdownBlocks';

// Placeholder heights only need to be in the right ballpark: a block is measured
// for real the moment it mounts, and these estimates just keep the scroll height
// close enough that a restored reading offset lands within tolerance. The tuning
// tracks reader-content.css at the default font size.
const PROSE_LINE_PX = 38; // ~1.9 line-height * 20px base
const CODE_LINE_PX = 26; // ~1.6 line-height * 0.78 * 20px
const BLOCK_MARGIN_PX = 18;
const CODE_PADDING_PX = 24;
const CHARS_PER_LINE = 40;
// A loaded illustration renders at its natural height, which the stripped text
// cannot see; reserve a typical image box so an asset-only block is not sized as
// a single empty line. Refined to the real height once the block mounts.
const IMAGE_RESERVE_PX = 320;

const CODE_BLOCK_RE = /<pre class="reader-md-code"><code>([\s\S]*?)<\/code><\/pre>/g;

function wrappedProseHeight(chars: number): number {
  if (chars <= 0) return 0;
  const lines = Math.max(1, Math.ceil(chars / CHARS_PER_LINE));
  return lines * PROSE_LINE_PX + BLOCK_MARGIN_PX;
}

/** Estimated pixel height of a rendered Markdown block for virtualization. */
export function estimateMarkdownBlockHeight(block: ReaderMarkdownBlock): number {
  let height = block.images.length * IMAGE_RESERVE_PX;

  // Fenced code uses white-space: pre and does not wrap, so each source line is
  // one row; counting its characters as wrapped prose badly overcounts a long
  // single line.
  const withoutCode = block.html.replace(CODE_BLOCK_RE, (_match, code: string) => {
    height += code.split('\n').length * CODE_LINE_PX + CODE_PADDING_PX;
    return '';
  });

  const prose = withoutCode.replace(/<[^>]+>/g, '').replace(/\s+/g, ' ').trim();
  height += wrappedProseHeight(prose.length);

  return Math.max(height, PROSE_LINE_PX);
}

/** Estimated pixel height of a plain-text reader block for virtualization. */
export function estimateTextBlockHeight(block: ReaderTextBlock): number {
  return Math.max(wrappedProseHeight(block.text.length), PROSE_LINE_PX);
}
