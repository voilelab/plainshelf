# Layers

Layers are the way PlainShelf organizes books into groups. They map directly to sub-directories inside the `books/` folder of your shelf.

---

## What is a layer?

A layer is a named directory that sits between the `books/` root and one or more book folders. You can nest layers inside other layers to build a tree-shaped hierarchy.

```text
books/
├─ {book}.novl/          # book at the root (no layer)
├─ Fiction/
│  ├─ {bookA}.novl/      # book inside the "Fiction" layer
│  └─ Classics/
│     └─ {bookB}.novl/   # book nested two levels deep
└─ Non-Fiction/
   └─ {bookC}.novl/
```

---

## Key properties

- **Filesystem-backed** — layers are real directories; they survive a server restart and can be browsed in any file manager.
- **Nestable** — there is no hard limit on nesting depth.
- **Independent from book IDs** — moving a book between layers does not change its ID or break reading progress.
- **Managed via the UI** — the web interface lets you create layers, delete empty layers, and move books between layers without touching the filesystem manually.

---

## Layer rules

- A layer cannot be deleted while it still contains books (you must move or delete the books first).
- A book can only belong to one layer at a time (it lives in exactly one directory).
- The `books/` root itself acts as a "no layer" / top-level group.

---

## Example use cases

| Use case | Layer structure |
|---|---|
| Genre classification | `Fiction/`, `Non-Fiction/`, `Poetry/` |
| Reading status | `To Read/`, `Reading/`, `Done/` |
| Language | `English/`, `中文/`, `Français/` |
| Mixed | `Fiction/English/`, `Fiction/中文/` |

Because layers are just directories, you can reorganize them freely without losing any book data.
