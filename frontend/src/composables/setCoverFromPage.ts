import { bookshelfWriter, getBookshelfProvider } from '@/providers';

export async function setCoverFromPage(bookId: string, sourceId: string, pageName: string): Promise<void> {
  const image = await getBookshelfProvider().getSourceAsset(bookId, sourceId, pageName);
  await bookshelfWriter().uploadBookCoverBlob(bookId, image);
}
