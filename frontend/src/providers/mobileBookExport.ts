import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

export async function exportMobileBookContent(blob: Blob, suggestedName: string): Promise<string> {
  const path = `PlainShelf/${suggestedName}`;
  await Filesystem.writeFile({
    path,
    data: await blob.text(),
    directory: Directory.Documents,
    encoding: Encoding.UTF8,
    recursive: true
  });
  return `Documents/${path}`;
}
