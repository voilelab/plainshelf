# Library and reader

What the library, the book detail page and the reader do once books are on a
shelf. Importing a first book is in [Getting started](getting-started.md#3-add-a-book).

## Searching the library

The library's search box matches titles, authors, tags and comments. It ignores
case, accents and full-width or half-width forms, treats katakana as hiragana,
and matches Traditional Chinese, Simplified Chinese and Japanese shinjitai or
kyujitai forms of the same character, so `红楼梦` finds `紅樓夢`. Name separators
such as `·` and `・` match a space. It does not search book contents.

## The book detail page

The book detail page shows the current source's size, including `source.txt`
and all files in its `assets/` directory. Other sources, metadata and the cover
are excluded. Images sources show their listed page count instead of line and
character counts. Size is measured when opening the detail page, including on
read-only shelves; if it cannot be measured, the size row is hidden.

A Markdown book's detail page also lists the chapters its source splits into,
the same H2 headings the reader navigates by. Selecting one opens the reader at
that chapter instead of your last position. Plain text has no chapters, so the
list is absent there.

When the current source is an images source, **Cover options → Choose from
pages** shows its pages and makes the one you pick the book cover. The comic
reader's **Set as cover** button does the same for the page on screen. The page
image is copied into the book's cover file, so later edits to the pages, or
deleting the source, leave the cover as it is. **Settings → Import & covers →
Convert uploaded covers to JPG** applies here as it does to an uploaded cover.

## Reading on a phone or narrow screen

On the Android app and narrow browser screens, the reader uses an immersive
layout: tap the center of the page to show or hide its controls, swipe left for
the next chapter, and swipe right for the previous chapter. Vertical swipes
continue to scroll within the current chapter. These gestures are shown once the
first time you open a book; the **?** button in the reader's controls brings the
reminder back at any time.

## Importing several files on macOS

In the macOS desktop app you can select or drop several files at once, and
switch to another app while they import. When an import of two or more books, or
an import of shelves from another device's LAN share, finishes while PlainShelf
is in the background, macOS shows a notification with how many were added and
how many failed; clicking it brings PlainShelf back. macOS asks for permission
the first time, and if you decline, imports carry on without notifications. Turn
them off under **Settings → Import & covers → Notify when an import finishes**.
