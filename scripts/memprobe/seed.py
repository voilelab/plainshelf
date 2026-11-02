#!/usr/bin/env python3
import argparse
import json
import random
import struct
import urllib.request
import uuid
import zlib

SHELF_ID = "default_shelf"


def png(width, height, rgb):
    row = b"\x00" + bytes(rgb) * width
    raw = row * height

    def chunk(tag, data):
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data))

    header = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", header) + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b"")


def book_text(index, size_kb, rng):
    words = ["書架", "閱讀", "章節", "夜色", "海風", "plain", "shelf", "reader", "memory", "chapter"]
    parts = []
    size = 0
    chapter = 0
    while size < size_kb * 1024:
        if size // (64 * 1024) >= chapter:
            chapter += 1
            heading = f"\n## Chapter {chapter}\n\n"
            parts.append(heading)
            size += len(heading.encode())
        line = " ".join(rng.choice(words) for _ in range(40)) + "\n\n"
        parts.append(line)
        size += len(line.encode())
    return f"Book {index}\n\n" + "".join(parts)


def multipart(fields, filename, content):
    boundary = uuid.uuid4().hex
    body = b""
    for name, value in fields.items():
        body += f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode()
    body += (
        f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\n'
        "Content-Type: text/markdown\r\n\r\n"
    ).encode()
    body += content + f"\r\n--{boundary}--\r\n".encode()
    return body, f"multipart/form-data; boundary={boundary}"


def request(method, url, body, content_type):
    req = urllib.request.Request(url, data=body, method=method, headers={"Content-Type": content_type})
    with urllib.request.urlopen(req, timeout=120) as resp:
        return resp.read()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--books", type=int, default=500)
    parser.add_argument("--book-kb", type=int, default=256)
    parser.add_argument("--large-book-mb", type=int, default=20)
    args = parser.parse_args()

    rng = random.Random(1)
    api = f"{args.base_url}/api/shelves/{SHELF_ID}/books"
    specs = [("0000 Large book", args.large_book_mb * 1024)]
    specs += [(f"{i:04d} Book", args.book_kb) for i in range(1, args.books + 1)]
    for index, (title, size_kb) in enumerate(specs):
        folder = f"Folder {index % 10}"
        body, ctype = multipart({"title": title, "folder": folder}, f"{title}.md", book_text(index, size_kb, rng).encode())
        book = json.loads(request("POST", f"{api}/import", body, ctype))
        cover = png(600, 900, [rng.randrange(256) for _ in range(3)])
        request("PUT", f"{api}/{book['meta']['id']}/cover", cover, "image/png")
    print(f"seeded {len(specs)} books")


if __name__ == "__main__":
    main()
