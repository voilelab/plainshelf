export class FolderHttpError extends Error {}

export class FolderMetaConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'FolderMetaConflictError';
  }
}
