export type BookFormat = string;
type BookTimestamp = string;
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
  published_at?: BookTimestamp;
  current_source?: string;
  star?: number;
  identifiers?: Record<string, string>;
  char_count?: number;

  nsfw?: boolean;

  nsfw_folder?: NsfwFolderRule;

  download_state?: DownloadState;
  local_version?: string;
  remote_version?: string;
  downloaded_at?: BookTimestamp;
}

export interface NsfwFolderRule {
  path: string;
  reason?: string;
}

export function isBookNsfw(book: NsfwMarks): boolean {
  return book.nsfw === true || book.nsfw_folder !== undefined;
}

export type NsfwMarks = Pick<Book, 'nsfw' | 'nsfw_folder'>;

export interface TrashedBook {
  id: string;
  title: string;
  authors: string[];
  original_path?: string;
  original_folder?: string[];
  deleted_at?: BookTimestamp;
}

export interface TrashedBookListing {
  books: TrashedBook[];
  complete: boolean;
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
  format?: BookFormat;
  nsfw?: boolean;
}

export type EpubImportPreset = 'markdown' | 'plain';

export interface EpubImportStrategy {
  preset: EpubImportPreset;
  include_description: boolean;

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
  strategy?: EpubImportStrategy;
}

export interface PaginatedBooks {
  items: Book[];
  total: number;
  page: number;
  pageSize: number;
}
