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
  layers: string[];
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
  original_layer?: string[];
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

export type SplitType = 'none' | 'line_count' | 'regex' | 'boundary';

export interface SplitConfig {
  type: SplitType;
  line_count?: number;
  regex?: string;
  boundaries?: number[];
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
}

export interface BookCreateRequest {
  title: string;
  layer?: string;
  file: File;
}

export type UpdateBookPayload = BookUpdateRequest;
export type ImportBookPayload = BookCreateRequest;

export interface PaginatedBooks {
  items: Book[];
  total: number;
  page: number;
  pageSize: number;
}
