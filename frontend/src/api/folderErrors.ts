/**
 * Raised for folder operations that failed in a way the UI reports verbatim.
 * Lives in its own module so the HTTP layer and the mock backend can both throw
 * it without importing each other.
 */
export class FolderHttpError extends Error {}
