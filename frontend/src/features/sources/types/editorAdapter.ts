export interface SourceEditorViewRange {
  startOffset: number;
  endOffset: number;
}

export interface SourceDocumentEdit {
  value: string;
  selectionStart: number;
  selectionEnd: number;
}

export type SourceFindScope = 'section' | 'source';

export interface SourceEditorAdapter {
  getCurrentParagraphStart(): number;
  replaceRange(startOffset: number, endOffset: number, replacement: string): void;
  jumpToOffset(offset: number): void;
  focusAndSelect(startOffset: number, endOffset: number): void;
}

export interface SourceEditorHandle extends SourceEditorAdapter {
  replaceRangeQuietly(startOffset: number, endOffset: number, replacement: string): void;
  flushDocument(): void;
}
