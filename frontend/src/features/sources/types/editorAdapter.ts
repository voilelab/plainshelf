export interface SourceEditorAdapter {
  getCurrentParagraphStart(): number;
  replaceRange(startOffset: number, endOffset: number, replacement: string): void;
  jumpToOffset(offset: number): void;
  focusAndSelect(startOffset: number, endOffset: number): void;
}
