export interface MobileCoverCache {
  getCover(bookId: string): Promise<Blob | null>;
  setCover(bookId: string, blob: Blob): Promise<void>;
  deleteCover(bookId: string): Promise<void>;
}

export class InMemoryMobileCoverCache implements MobileCoverCache {
  private readonly covers = new Map<string, Blob>();

  async getCover(bookId: string): Promise<Blob | null> {
    return this.covers.get(bookId) ?? null;
  }

  async setCover(bookId: string, blob: Blob): Promise<void> {
    this.covers.set(bookId, blob);
  }

  async deleteCover(bookId: string): Promise<void> {
    this.covers.delete(bookId);
  }
}
