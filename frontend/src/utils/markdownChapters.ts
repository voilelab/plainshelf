import type { ReaderSection } from '@/types/book';
import {
  parseMarkdownHeadingLine,
  updateMarkdownFenceState,
  type MarkdownFenceState
} from './markdownLineSyntax';

export interface MarkdownChapterHeading {
  startOffset: number;
  endOffset: number;
  title: string;
}

export interface MarkdownEditorSection {
  index: number;
  startOffset: number;
  endOffset: number;
  title: string;
  kind: 'opening' | 'chapter';
  headingIndex?: number;
  heading?: MarkdownChapterHeading;
}

function visibleHeadingText(value: string): string {
  return value
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\*\*([^*]*)\*\*/g, '$1')
    .replace(/\*([^*]*)\*/g, '$1')
    .trim();
}

export function buildMarkdownH2Sections(content: string): ReaderSection[] {
  return buildMarkdownEditorSections(content).map((section) => ({
    index: section.index,
    startOffset: section.startOffset,
    endOffset: section.endOffset,
    title: section.title,
    text: content.slice(section.startOffset, section.endOffset)
  }));
}

export interface MarkdownChapterListItem {
  index: number;
  title: string;
}

export function buildMarkdownChapterList(content: string): MarkdownChapterListItem[] {
  const sections = buildMarkdownEditorSections(content);
  if (!sections.some((section) => section.kind === 'chapter')) {
    return [];
  }

  return sections.map((section) => ({ index: section.index, title: section.title }));
}

export function buildMarkdownEditorSections(content: string): MarkdownEditorSection[] {
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
    return [{
      index: 0,
      startOffset: 0,
      endOffset: content.length,
      title: preambleH1 || 'Part 1',
      kind: 'opening'
    }];
  }

  const sections: MarkdownEditorSection[] = [];
  const firstChapterOffset = chapters[0].startOffset;
  if (content.slice(0, firstChapterOffset).trim().length > 0) {
    sections.push({
      index: 0,
      startOffset: 0,
      endOffset: firstChapterOffset,
      title: preambleH1 || 'Opening',
      kind: 'opening'
    });
  }

  chapters.forEach((chapter, chapterIndex) => {
    const startOffset = chapterIndex === 0 && sections.length === 0 ? 0 : chapter.startOffset;
    const endOffset = chapters[chapterIndex + 1]?.startOffset ?? content.length;
    sections.push({
      index: sections.length,
      startOffset,
      endOffset,
      title: chapter.title,
      kind: 'chapter',
      headingIndex: chapterIndex,
      heading: chapter
    });
  });

  return sections;
}

export function findMarkdownEditorSection(
  sections: MarkdownEditorSection[],
  offset: number,
  affinity: 'forward' | 'backward'
): MarkdownEditorSection | null {
  if (sections.length === 0) return null;
  const contentLength = sections[sections.length - 1]?.endOffset ?? 0;
  const clamped = Math.max(0, Math.min(contentLength, offset));
  for (let index = 0; index < sections.length; index += 1) {
    const section = sections[index];
    if (clamped === section.startOffset && affinity === 'backward' && index > 0) {
      return sections[index - 1];
    }
    const isLast = index === sections.length - 1;
    if (
      clamped >= section.startOffset &&
      (clamped < section.endOffset || (isLast && clamped <= section.endOffset))
    ) return section;
  }
  return sections[sections.length - 1] ?? null;
}

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
