import { Directory, Encoding, Filesystem } from '@capacitor/filesystem';

/** Writes a readable copy to the shared Documents folder, as before Go downloads. */
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
