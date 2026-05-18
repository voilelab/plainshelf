export interface SnapshotMeta {
  id: string;
  created_at: string;
  comment: string;
  md5_hash: string;
  line_count?: number;
  char_count?: number;
  split_config?: {
    type?: string;
  };
}
