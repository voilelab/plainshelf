import {
  assetImageFromMarkdownLine,
  assetNameFromSrc,
  referencedAssetNames
} from './markdownAssetImages';

const HEADING_RE = /^ {0,3}(#{1,6})[ \t]+(.+)$/;
const FENCE_OPENER_RE = /^ {0,3}(`{3,}|~{3,})(.*)$/;

export { assetImageFromMarkdownLine, assetNameFromSrc, referencedAssetNames };

interface MarkdownHeadingLine {
  level: 1 | 2 | 3 | 4 | 5 | 6;
  title: string;
}

export interface MarkdownFenceState {
  marker: '`' | '~';
  length: number;
}

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
  // CommonMark section 4.5 forbids backticks in the info string of a backtick fence.
  if (run[0] === '`' && match[2].includes('`')) return null;
  return { marker: run[0] as MarkdownFenceState['marker'], length: run.length };
}

function closesMarkdownFence(rawLine: string, fence: MarkdownFenceState): boolean {
  const match = /^ {0,3}(`+|~+)[ \t]*$/.exec(rawLine);
  return Boolean(match && match[1][0] === fence.marker && match[1].length >= fence.length);
}

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
