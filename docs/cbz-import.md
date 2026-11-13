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

## Reading comics

The web, desktop, standalone reader, and Android clients display one page at a
time. Tap the left or right edge, use the arrow
keys, or swipe horizontally to turn pages. The bottom controls also let you
choose a page.

For left-to-right books, tap right, press Right Arrow, or swipe left for the
next page. For right-to-left books, tap left, press Left Arrow, or swipe right.

The page size menu in the reader toolbar offers **Fit height** (the default,
which shows the whole page), **Fit width**, and **Original size**. The zoom
buttons scale any of them from 50% to 300%; `+` and `-` change the zoom and `0`
resets it to 100%. A page larger than the screen scrolls with the mouse wheel or
by dragging, and scrolling past its end does not turn the page. Each new page
starts at its top corner on the reading side. Horizontal swipes pan instead of
turning while the page is wider than the screen, and tapping or swiping does not
turn pages while the browser itself is pinch-zoomed; use the buttons or arrow
keys then. The size and zoom are saved on the device for every comic, not in the
shelf.

The reader preloads two pages in each direction. Missing or unreadable images
show their page number, and you can continue reading.

Progress saves the zero-based page index on the current device. Completion is
based on the displayed page's number divided by the number of listed pages;
the final page is 100%, and missing pages still count.

On Android, download a book before going offline. Server connections download
listed images through asset bundles and cache the parsed page list. A failed
download of an available page does not mark the book as downloaded. pCloud
shelves remain read-only; their embedded Go backend downloads the whole book
package. Both paths preserve page order and reading direction offline.

## Editing pages

Open the book's sources editor and select an `images` source to see its pages
as a grid of thumbnails. Every change saves at once:

- Drag a page onto the left or right half of another page to place it before
  or after that page; a line marks where it lands. Each page's arrow buttons
  move it one place and appear when the pointer is over the page, it has
  keyboard focus, or it is selected (always, on a touch screen). Only
  `source.txt` is rewritten; no image is renamed.
- Choose **Add pages** to upload one or more images. They go after the selected
  page, or at the end when no page is selected, and each is stored under a new
  random name in its original format.
- Choose **×** on a page to delete it and its image file. A missing page shows
  as a placeholder and **×** only removes it from the list.
- Images in the source folder that the list does not name, such as files copied
  in by hand, appear under **Not in the page list**. Add one to the end of the
  list or delete it.
- **Reading direction** switches between left to right, right to left, and top
  to bottom, and rewrites only the source's `meta.json`. Top to bottom marks a
  vertical strip comic; the reader still turns its pages left to right until
  vertical scrolling is available.

If `source.txt` changed since the editor loaded it, for example through a sync
tool, the edit is refused and the editor asks you to reload the pages.

## Limits

| Limit | Maximum |
|---|---:|
| HTTP upload request, including form fields | 100 MiB |
| ZIP entries, including skipped entries and directories | 2,000 |
| Uncompressed size of any entry, including skipped files | 20 MiB |
| Total declared uncompressed size of all entries | 2 GiB |
| Pixels per image, checked from its header | 100 million |

An image added in the editor has the same 20 MiB and 100-million-pixel limits.

The importer rejects absolute paths, Windows drive paths, `..` path components,
and symbolic links. No archive entry is extracted to its original path.

If a limit, image check, or write fails, the import fails without publishing a
partial book. The error identifies the failed check. Images are checked one at
a time in temporary storage before the book is published; temporary files are
removed on success or failure.

CBR, CB7, PDF, and encrypted archives are not supported.
