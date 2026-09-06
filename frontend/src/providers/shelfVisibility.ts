import { isBookNsfw, type Book } from '@/types/book';

/** What `collectFolders` names the top level; a book with no folder is in it. */
const ROOT_FOLDER = '/';

/** `createNSFWFolderLookup` as a yes or no, over the path's segments. */
type IsNsfwFolder = (folders: readonly string[]) => boolean;

interface ShelfVisibilityOptions {
  /** The device's answer; see useDeviceNsfwPreference. */
  showNsfw: boolean;
  /** The shelf's folder rules. Omit only where `filterFolders` is not used. */
  isNsfwFolder?: IsNsfwFolder;
}

/**
 * Which of a shelf's books exist at all, as far as this client is concerned.
 *
 * One object rather than a filter per call site, for the reason
 * `bookVisibility` in server/visibility.go is one type: an entry point that did
 * not filter would still hand the book to anyone holding its id. `showNsfw` is
 * read once, so a listing and a folder tree derived from it cannot disagree.
 */
export class ShelfVisibility {
  private readonly showNsfw: boolean;
  private readonly isNsfwFolder: IsNsfwFolder;

  constructor(options: ShelfVisibilityOptions) {
    this.showNsfw = options.showNsfw;
    this.isNsfwFolder = options.isNsfwFolder ?? (() => false);
  }

  /** Whether this client is served `book` at all. */
  allows(book: Pick<Book, 'nsfw' | 'nsfw_folder'>): boolean {
    return this.showNsfw || !isBookNsfw(book);
  }

  /** The listing without the books this client may not see. */
  keepBooks<T extends Pick<Book, 'nsfw' | 'nsfw_folder'>>(books: readonly T[]): readonly T[] {
    return this.showNsfw ? books : books.filter((book) => this.allows(book));
  }

  /**
   * Mirrors `bookVisibility.filterFolders`: a folder in a marked subtree goes,
   * and so does one whose books are all hidden, since its name is the
   * disclosure. One that was always empty is kept. `books` must be the
   * *unfiltered* listing — the filtered one cannot tell those two apart.
   */
  filterFolders(folders: readonly string[], books: readonly Book[]): string[] {
    if (this.showNsfw) {
      return [...folders];
    }

    const holdsBook = new Set<string>();
    const holdsVisibleBook = new Set<string>();
    for (const book of books) {
      const allowed = this.allows(book);
      // A book counts for its own folder and for every folder above it, so a
      // parent is judged by everything in its subtree.
      for (let depth = 0; depth <= book.folders.length; depth += 1) {
        const key = folderKey(book.folders.slice(0, depth));
        holdsBook.add(key);
        if (allowed) {
          holdsVisibleBook.add(key);
        }
      }
    }

    return folders.filter((folder) => {
      if (this.isNsfwFolder(folderSegments(folder))) {
        return false;
      }
      return !holdsBook.has(folder) || holdsVisibleBook.has(folder);
    });
  }
}

/** The listing's name for the folder these segments reach. */
function folderKey(segments: readonly string[]): string {
  return segments.length === 0 ? ROOT_FOLDER : segments.join('/');
}

/** The inverse of {@link folderKey}: the top level is no segments, not one. */
function folderSegments(folder: string): string[] {
  return folder === ROOT_FOLDER ? [] : folder.split('/');
}
