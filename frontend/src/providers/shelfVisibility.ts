import { isBookNsfw, type Book } from '@/types/book';

const ROOT_FOLDER = '/';

type IsNsfwFolder = (folders: readonly string[]) => boolean;

interface ShelfVisibilityOptions {
  showNsfw: boolean;
  isNsfwFolder?: IsNsfwFolder;
}

export class ShelfVisibility {
  private readonly showNsfw: boolean;
  private readonly isNsfwFolder: IsNsfwFolder;

  constructor(options: ShelfVisibilityOptions) {
    this.showNsfw = options.showNsfw;
    this.isNsfwFolder = options.isNsfwFolder ?? (() => false);
  }

  allows(book: Pick<Book, 'nsfw' | 'nsfw_folder'>): boolean {
    return this.showNsfw || !isBookNsfw(book);
  }

  keepBooks<T extends Pick<Book, 'nsfw' | 'nsfw_folder'>>(books: readonly T[]): readonly T[] {
    return this.showNsfw ? books : books.filter((book) => this.allows(book));
  }

  filterFolders(folders: readonly string[], books: readonly Book[]): string[] {
    if (this.showNsfw) {
      return [...folders];
    }

    const holdsBook = new Set<string>();
    const holdsVisibleBook = new Set<string>();
    for (const book of books) {
      const allowed = this.allows(book);
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

function folderKey(segments: readonly string[]): string {
  return segments.length === 0 ? ROOT_FOLDER : segments.join('/');
}

function folderSegments(folder: string): string[] {
  return folder === ROOT_FOLDER ? [] : folder.split('/');
}
