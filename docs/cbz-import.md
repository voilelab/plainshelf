# CBZ import

Choose **Import** in the library and select a `.cbz` file, or drop it onto the
import dialog. The desktop file picker also accepts CBZ files.

PlainShelf extracts the archive into an `images` source. It keeps each supported
image's original bytes and extension in `sources/{source-id}/assets/`, under a
random name. It does not convert images or create thumbnails. The original CBZ
is not stored; keep your own copy if you need it.

## Pages and metadata

Pages follow the natural order of their full paths inside the ZIP. For example,
`chapter2/2.jpg` comes before `chapter2/10.jpg`, and `chapter2/10.jpg` comes
before `chapter10/1.jpg`. The ordered random asset names go into `source.txt`,
one per line. The first page is also copied as the book cover, unchanged.

Supported extensions are `.jpg`, `.jpeg`, `.png`, `.webp`, and `.gif`, ignoring
case. Other files, hidden files and directories, and `__MACOSX/` are skipped.
A supported image whose header cannot be decoded fails the entire import.

If the archive contains `ComicInfo.xml`, PlainShelf reads `Title` as the title
and `Writer` as the authors, separated by commas. A title entered in the import
dialog takes precedence over the metadata. Without a metadata title, the
filename without `.cbz` is the title. `Manga=YesAndRightToLeft` sets
`page_direction` to `rtl`; otherwise the source defaults to left to right.
Malformed ComicInfo XML fails the import.

This import stores pages and reading direction. The existing text reader does
not display an `images` source as a comic reader yet.

## Limits

| Limit | Maximum |
|---|---:|
| HTTP upload request, including form fields | 100 MiB |
| ZIP entries, including skipped entries and directories | 2,000 |
| Uncompressed size of any entry, including skipped files | 20 MiB |
| Total declared uncompressed size of all entries | 2 GiB |
| Pixels per image, checked from its header | 100 million |

The importer rejects absolute paths, Windows drive paths, `..` path components,
and symbolic links. No archive entry is extracted to its original path.

If a limit, image check, or write fails, the import fails without publishing a
partial book. The error identifies the failed check. Images are checked one at
a time in temporary storage before the book is published; temporary files are
removed on success or failure.

CBR, CB7, PDF, and encrypted archives are not supported.
