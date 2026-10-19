/**
 * Raised for folder operations that failed in a way the UI reports verbatim.
 * Lives in its own module so the HTTP layer and the mock backend can both throw
 * it without importing each other.
 */
export class FolderHttpError extends Error {}

/**
 * Thrown when folder.json changed after the version the caller wrote against
 * was read. The server's listing is fresh again by then, so re-reading it and
 * retrying is the whole recovery.
 */
export class FolderMetaConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'FolderMetaConflictError';
  }
}
