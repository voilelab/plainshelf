/** The chapter the editor is focused on, in whole-document coordinates. */
export interface SourceEditorViewRange {
  startOffset: number;
  endOffset: number;
}

/** A document snapshot published by the editor, with the caret it left behind. */
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

/**
 * The adapter plus the two operations only the hosting page needs.
 *
 * `SourceEditorAdapter` stays the contract every other caller is written
 * against; these two exist because the editor now owns the document and
 * publishes it in batches, so the page needs a way to edit without stealing the
 * caret and a way to demand an exact, current copy.
 */
export interface SourceEditorHandle extends SourceEditorAdapter {
  /** Applies an edit without moving focus or the selection. */
  replaceRangeQuietly(startOffset: number, endOffset: number, replacement: string): void;
  /** Publishes any batched document change immediately. */
  flushDocument(): void;
}
