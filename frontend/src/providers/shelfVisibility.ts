import { isBookNsfw, type Book } from '@/types/book';

/**
 * The folder path a listing names the top level by. `collectFolders` produces
 * it, and a book sitting directly under `books/` belongs to it.
 */
const ROOT_FOLDER = '/';

/**
 * Whether a folder path lies in a marked subtree — `createNSFWFolderLookup`
 * read as a yes or no. Takes the path's segments, as the shelf rules do.
 */
type IsNsfwFolder = (folders: readonly string[]) => boolean;

interface ShelfVisibilityOptions {
  /** The device's answer; see useDeviceNsfwPreference. */
  showNsfw: boolean;
  /**
   * The shelf's folder rules. Omitted where the caller has books but no rules —
   * the downloads list, which reads the mark off each stored book — in which
   * case only a folder inside a marked subtree cannot be recognised as one, and
   * `filterFolders` must not be used.
   */
  isNsfwFolder?: IsNsfwFolder;
}

/**
 * Which of a shelf's books exist at all, as far as this client is concerned.
 *
 * One object rather than a filter per call site, for the reason `bookVisibility`
 * in server/visibility.go is one type: the answer has to be the same on every
 * entry point. A listing that filtered while the single-book lookup did not
 * would still hand the book to anyone holding its id, and a folder tree that
 * did not would name the folder the books were hidden from.
 *
 * `showNsfw` is read once, when the filter is built, so a listing and the folder
 * filter derived from it cannot disagree because the setting changed halfway
 * through.
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

  /**
   * The listing with the books this client may not see removed. Returns the
   * array it was given when nothing is filtered, so a shelf that marks nothing
   * pays no copy.
   */
  keepBooks<T extends Pick<Book, 'nsfw' | 'nsfw_folder'>>(books: readonly T[]): readonly T[] {
    return this.showNsfw ? books : books.filter((book) => this.allows(book));
  }

  /**
   * The folder list with the folders this client must not see dropped, keeping
   * the rest in the order they were given.
   *
   * Mirrors `bookVisibility.filterFolders`. Two kinds go: a folder inside a
   * marked subtree, and a folder whose books are all hidden — the second stops
   * the mark showing through its own absence, since a folder holding nothing but
   * marked books would otherwise stay in the tree with its name as the
   * disclosure. A folder holding no book at all is kept: someone made it, and
   * dropping it would take away a destination.
   *
   * `books` must be the *unfiltered* listing: the filtered one alone cannot tell
   * a folder emptied by the filter from one that was always empty.
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
