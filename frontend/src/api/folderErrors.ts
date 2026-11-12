export class FolderHttpError extends Error {}

export class FolderNotEmptyError extends Error {
  constructor(readonly entries: string[]) {
    super('Cannot delete this folder because it is not empty.');
    this.name = 'FolderNotEmptyError';
  }
}

export class FolderMetaConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'FolderMetaConflictError';
  }
}
