export type BookFormat = string;
export type BookTimestamp = string;

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
  published_at?: BookTimestamp;
  current_snapshot?: string;
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

export interface BookmarkPayload {
  char_offset: number;
}

export interface BookUpdateRequest {
  title?: string;
  tags?: string[];
  authors?: string[];
  language?: string;
  comment?: string;
}

export interface BookCreateRequest {
  title: string;
  alias?: string;
  layer?: string;
  file: File;
  coverFile?: File;
}

export type UpdateBookPayload = BookUpdateRequest;
export type ImportBookPayload = BookCreateRequest;

export interface PaginatedBooks {
  items: Book[];
  total: number;
  page: number;
  pageSize: number;
}
