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
  current_source?: string;
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
