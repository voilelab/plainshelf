import type { ReaderSection } from '@/types/book';
import {
  parseMarkdownHeadingLine,
  updateMarkdownFenceState,
  type MarkdownFenceState
} from './parseMarkdownBlocks';

export interface MarkdownChapterHeading {
  startOffset: number;
  endOffset: number;
  title: string;
}

function visibleHeadingText(value: string): string {
  return value
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\*\*([^*]*)\*\*/g, '$1')
    .replace(/\*([^*]*)\*/g, '$1')
    .trim();
}

/**
 * Builds the schema-versioned Markdown chapter model.
 *
 * JavaScript string indexes are UTF-16 code-unit offsets, which is the same
 * coordinate system used by persisted reading progress. Scanning lines while
 * accumulating their original lengths also preserves CRLF offsets exactly.
 */
export function buildMarkdownH2Sections(content: string): ReaderSection[] {
  const chapters = scanMarkdownH2Headings(content);
  let preambleH1 = '';
  let fence: MarkdownFenceState | null = null;
  let offset = 0;

  for (const lineWithNewline of content.split(/(?<=\n)/)) {
    const rawLine = lineWithNewline.endsWith('\n')
      ? lineWithNewline.slice(0, -1).replace(/\r$/, '')
      : lineWithNewline.replace(/\r$/, '');

    const transition = updateMarkdownFenceState(rawLine, fence);
    if (transition.boundary) {
      fence = transition.state;
      offset += lineWithNewline.length;
      continue;
    }

    if (!fence) {
      const heading = parseMarkdownHeadingLine(rawLine);
      if (heading?.level === 1 && offset < (chapters[0]?.startOffset ?? content.length) && !preambleH1) {
        preambleH1 = visibleHeadingText(heading.title);
      }
    }

    offset += lineWithNewline.length;
  }

  if (chapters.length === 0) {
    return [{ index: 0, startOffset: 0, endOffset: content.length, title: preambleH1 || 'Part 1', text: content }];
  }

  const sections: ReaderSection[] = [];
  const firstChapterOffset = chapters[0].startOffset;
  if (content.slice(0, firstChapterOffset).trim().length > 0) {
    sections.push({
      index: 0,
      startOffset: 0,
      endOffset: firstChapterOffset,
      title: preambleH1 || 'Opening',
      text: content.slice(0, firstChapterOffset)
    });
  }

  chapters.forEach((chapter, chapterIndex) => {
    // Whitespace before the first H2 belongs to that chapter when there is no
    // meaningful preamble, keeping offset zero reachable from navigation.
    const startOffset = chapterIndex === 0 && sections.length === 0 ? 0 : chapter.startOffset;
    const endOffset = chapters[chapterIndex + 1]?.startOffset ?? content.length;
    sections.push({
      index: sections.length,
      startOffset,
      endOffset,
      title: chapter.title,
      text: content.slice(startOffset, endOffset)
    });
  });

  return sections;
}

/** Returns editable H2 line ranges, excluding headings inside fenced code. */
export function scanMarkdownH2Headings(content: string): MarkdownChapterHeading[] {
  const headings: MarkdownChapterHeading[] = [];
  let fence: MarkdownFenceState | null = null;
  let offset = 0;

  for (const lineWithNewline of content.split(/(?<=\n)/)) {
    const lineWithoutNewline = lineWithNewline.endsWith('\n') ? lineWithNewline.slice(0, -1) : lineWithNewline;
    const rawLine = lineWithoutNewline.replace(/\r$/, '');
    const transition = updateMarkdownFenceState(rawLine, fence);
    if (transition.boundary) {
      fence = transition.state;
    } else if (!fence) {
      const heading = parseMarkdownHeadingLine(rawLine);
      if (heading?.level === 2) {
        headings.push({
          startOffset: offset,
          endOffset: offset + lineWithoutNewline.length,
          title: visibleHeadingText(heading.title) || 'Untitled chapter'
        });
      }
    }
    offset += lineWithNewline.length;
  }

  return headings;
}
