export interface SnapshotMeta {
  id: string;
  created_at: string;
  comment: string;
  md5_hash: string;
  split_config?: {
    type?: string;
  };
}
