export interface SourceEditorViewRange {
  startOffset: number;
  endOffset: number;
  key: number;
}

export interface SourceDocumentEdit {
  value: string;
  selectionStart: number;
  selectionEnd: number;
  affinity: 'forward' | 'backward';
  deferView?: boolean;
}

export type SourceFindScope = 'section' | 'source';

export interface SourceEditorAdapter {
  getCurrentParagraphStart(): number;
  replaceRange(startOffset: number, endOffset: number, replacement: string): void;
  jumpToOffset(offset: number): void;
  focusAndSelect(startOffset: number, endOffset: number): void;
}
