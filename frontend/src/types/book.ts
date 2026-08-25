export type BookFormat = string;
export type BookTimestamp = string;
export type DownloadState =
  | 'not_downloaded'
  | 'downloaded'
  | 'update_available'
  | 'downloading'
  | 'failed';

export interface Book {
  id: string;
  title: string;
  authors: string[];
  tags: string[];
  language?: string;
  format?: BookFormat;
  comment?: string;
  cover?: string;
  cover_url?: string;
  folders: string[];
  created_at?: BookTimestamp;
  updated_at?: BookTimestamp;
  /** Date-only ("YYYY-MM-DD"); the backend normalizes any legacy full timestamp before it reaches the API. */
  published_at?: BookTimestamp;
  current_source?: string;
  star?: number;
  identifiers?: Record<string, string>;
  /** Total character count of the book's content. Optional: only present when the
   *  backend is asked to include it (see listBooks' includeCharCount option) or in
   *  mock data; omit rather than guess when unavailable. */
  char_count?: number;

  // Mobile/offline cache metadata. These fields are optional so existing
  // server and Wails responses remain valid when they do not include local
  // download information.
  download_state?: DownloadState;
  local_version?: string;
  remote_version?: string;
  downloaded_at?: BookTimestamp;
}

export interface TrashedBook {
  id: string;
  title: string;
  authors: string[];
  original_path?: string;
  original_folder?: string[];
  deleted_at?: BookTimestamp;
}

export interface BookDetail extends Book {
  progress?: ReadingProgress;
}

export interface ReadingProgress {
  file_path?: string;
  char_offset: number;
  percent?: number;
}

export interface BookContent {
  content: string;
}

export interface ReaderSection {
  index: number;
  startOffset: number;
  endOffset: number;
  title: string;
  text: string;
}

export interface BookmarkPayload {
  char_offset: number;
  /**
   * Epoch ms the position changed (not when it was flushed). Used to arbitrate
   * concurrent cross-process writes newest-wins; optional, defaulting to now when
   * a caller does not supply it. Ignored by backends that do not share the
   * reading-progress document (mobile keeps its own per-book files).
   */
  at?: number;
}

export interface BookUpdateRequest {
  title?: string;
  tags?: string[];
  authors?: string[];
  language?: string;
  comment?: string;
  published_at?: string;
  star?: number;
  identifiers?: Record<string, string>;
  /** Only 'txt' and 'md' are accepted; the server rejects anything else. */
  format?: BookFormat;
}

/** The formats a book can be stored as. Both read the same bytes on disk: the
 *  value only decides whether the reader parses the text as Markdown. */
export const BOOK_FORMAT_OPTIONS: { value: BookFormat; label: string }[] = [
  { value: 'txt', label: 'Plain text' },
  { value: 'md', label: 'Markdown' }
];

/** Built-in EPUB output layouts. The preset also decides the stored book format. */
export type EpubImportPreset = 'markdown' | 'plain';

export interface EpubImportStrategy {
  preset: EpubImportPreset;
  include_description: boolean;

  /**
   * Whether the conversion stores the EPUB's illustrations. Optional because
   * omitting it means "unspecified": the server then falls back to the
   * configured setting rather than a built-in default. Dropping it while
   * round-tripping a strategy would silently re-enable images for a user who
   * turned them off.
   */
  keep_images?: boolean;
}

export const DEFAULT_EPUB_IMPORT_STRATEGY: EpubImportStrategy = {
  preset: 'markdown',
  include_description: true
};

export interface BookCreateRequest {
  title: string;
  folder?: string;
  file: File;
  /** Only meaningful for .epub uploads; ignored by the server for other formats. */
  strategy?: EpubImportStrategy;
}

export type UpdateBookPayload = BookUpdateRequest;
export type ImportBookPayload = BookCreateRequest;

export interface PaginatedBooks {
  items: Book[];
  total: number;
  page: number;
  pageSize: number;
}
