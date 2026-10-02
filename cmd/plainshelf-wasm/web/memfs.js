// In-memory stand-in for the Node `fs` callback API that Go's js/wasm
// syscall layer calls (GOROOT/src/syscall/fs_js.go). Load before wasm_exec.js.
(() => {
  const S_IFDIR = 0o040000;
  const S_IFREG = 0o100000;
  const constants = {
    O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128,
    O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536,
  };

  let nextIno = 1;
  const nodes = new Map(); // normalized path -> node
  const fds = new Map(); // fd -> { node, flags }
  let nextFd = 3;

  const fsError = (code) => Object.assign(new Error(code), { code });

  function norm(p) {
    const out = [];
    for (const part of String(p).split('/')) {
      if (part === '' || part === '.') continue;
      if (part === '..') out.pop();
      else out.push(part);
    }
    return '/' + out.join('/');
  }
  const parentOf = (p) => (p === '/' ? '/' : p.slice(0, p.lastIndexOf('/')) || '/');

  function newNode(type, mode) {
    const now = Date.now();
    return { type, mode, ino: nextIno++, data: new Uint8Array(0), size: 0, atimeMs: now, mtimeMs: now, ctimeMs: now };
  }
  nodes.set('/', newNode('dir', 0o755));

  function lookup(p) {
    const n = nodes.get(norm(p));
    if (!n) throw fsError('ENOENT');
    return n;
  }
  function requireParentDir(p) {
    const parent = nodes.get(parentOf(p));
    if (!parent) throw fsError('ENOENT');
    if (parent.type !== 'dir') throw fsError('ENOTDIR');
  }
  function childrenOf(dir) {
    const prefix = dir === '/' ? '/' : dir + '/';
    const names = [];
    for (const key of nodes.keys()) {
      if (key !== dir && key.startsWith(prefix) && !key.slice(prefix.length).includes('/')) {
        names.push(key.slice(prefix.length));
      }
    }
    return names.sort();
  }
  function stat(n) {
    return {
      dev: 1, ino: n.ino, mode: (n.type === 'dir' ? S_IFDIR : S_IFREG) | n.mode, nlink: 1,
      uid: 0, gid: 0, rdev: 0, size: n.type === 'dir' ? 0 : n.size, blksize: 4096,
      blocks: Math.ceil(n.size / 512), atimeMs: n.atimeMs, mtimeMs: n.mtimeMs, ctimeMs: n.ctimeMs,
      isDirectory: () => n.type === 'dir',
    };
  }
  function ensureCapacity(n, size) {
    if (size <= n.data.length) return;
    const grown = new Uint8Array(Math.max(size, n.data.length * 2, 64));
    grown.set(n.data.subarray(0, n.size));
    n.data = grown;
  }
  function resize(n, size) {
    ensureCapacity(n, size);
    if (size < n.size) n.data.fill(0, size, n.size);
    n.size = size;
    n.mtimeMs = Date.now();
  }
  function fdEntry(fd) {
    const e = fds.get(fd);
    if (!e) throw fsError('EBADF');
    return e;
  }

  // Each call runs synchronously and reports through the Node-style callback.
  const wrap = (fn) => (...args) => {
    const cb = args.pop();
    let result;
    try {
      result = fn(...args);
    } catch (err) {
      if (!err.code) throw err;
      cb(err);
      return;
    }
    cb(null, result);
  };

  const decoder = new TextDecoder();
  const outBuf = { 1: '', 2: '' };

  globalThis.fs = {
    constants,
    // Go writes stdout/stderr through writeSync; forward complete lines to the console.
    writeSync(fd, buf) {
      if (fd === 1 || fd === 2) {
        outBuf[fd] += decoder.decode(buf);
        const nl = outBuf[fd].lastIndexOf('\n');
        if (nl !== -1) {
          (fd === 2 ? console.warn : console.log)(outBuf[fd].slice(0, nl));
          outBuf[fd] = outBuf[fd].slice(nl + 1);
        }
        return buf.length;
      }
      const e = fdEntry(fd);
      return writeAt(e, buf, 0, buf.length, null);
    },

    open: wrap((path, flags, mode) => {
      const p = norm(path);
      let n = nodes.get(p);
      if (n && (flags & constants.O_CREAT) && (flags & constants.O_EXCL)) throw fsError('EEXIST');
      if (!n) {
        if (!(flags & constants.O_CREAT)) throw fsError('ENOENT');
        requireParentDir(p);
        n = newNode('file', mode & 0o777);
        nodes.set(p, n);
      }
      if ((flags & constants.O_DIRECTORY) && n.type !== 'dir') throw fsError('ENOTDIR');
      if (n.type === 'dir' && (flags & 3) !== constants.O_RDONLY) throw fsError('EISDIR');
      if ((flags & constants.O_TRUNC) && n.type === 'file') resize(n, 0);
      const fd = nextFd++;
      fds.set(fd, { node: n, flags, pos: 0 });
      return fd;
    }),
    close: wrap((fd) => { fdEntry(fd); fds.delete(fd); }),
    read: wrap((fd, buf, offset, length, position) => {
      const e = fdEntry(fd);
      if (e.node.type === 'dir') throw fsError('EISDIR');
      const at = position ?? e.pos;
      const n = Math.max(0, Math.min(length, e.node.size - at));
      buf.set(e.node.data.subarray(at, at + n), offset);
      if (position == null) e.pos += n;
      return n;
    }),
    write: wrap((fd, buf, offset, length, position) => writeAt(fdEntry(fd), buf, offset, length, position)),
    fstat: wrap((fd) => stat(fdEntry(fd).node)),
    stat: wrap((path) => stat(lookup(path))),
    lstat: wrap((path) => stat(lookup(path))),
    fsync: wrap((fd) => { fdEntry(fd); }),
    ftruncate: wrap((fd, len) => resize(fdEntry(fd).node, len)),
    truncate: wrap((path, len) => resize(lookup(path), len)),
    mkdir: wrap((path, perm) => {
      const p = norm(path);
      if (nodes.has(p)) throw fsError('EEXIST');
      requireParentDir(p);
      nodes.set(p, newNode('dir', perm & 0o777));
    }),
    readdir: wrap((path) => {
      const p = norm(path);
      if (lookup(p).type !== 'dir') throw fsError('ENOTDIR');
      return childrenOf(p);
    }),
    rmdir: wrap((path) => {
      const p = norm(path);
      if (lookup(p).type !== 'dir') throw fsError('ENOTDIR');
      if (childrenOf(p).length) throw fsError('ENOTEMPTY');
      nodes.delete(p);
    }),
    unlink: wrap((path) => {
      const p = norm(path);
      if (lookup(p).type === 'dir') throw fsError('EISDIR');
      nodes.delete(p);
    }),
    rename: wrap((from, to) => {
      const src = norm(from);
      const dst = norm(to);
      const n = lookup(src);
      requireParentDir(dst);
      if (src === dst) return;
      const existing = nodes.get(dst);
      if (existing) {
        if (existing.type === 'dir' && childrenOf(dst).length) throw fsError('ENOTEMPTY');
        if (existing.type !== n.type) throw fsError(existing.type === 'dir' ? 'EISDIR' : 'ENOTDIR');
        nodes.delete(dst);
      }
      const moved = [];
      for (const [key, node] of nodes) {
        if (key === src || key.startsWith(src + '/')) moved.push([key, node]);
      }
      for (const [key] of moved) nodes.delete(key);
      for (const [key, node] of moved) nodes.set(dst + key.slice(src.length), node);
    }),
    utimes: wrap((path, atime, mtime) => {
      const n = lookup(path);
      n.atimeMs = atime * 1000;
      n.mtimeMs = mtime * 1000;
    }),
    chmod: wrap((path, mode) => { lookup(path).mode = mode & 0o777; }),
    fchmod: wrap((fd, mode) => { fdEntry(fd).node.mode = mode & 0o777; }),
    chown: wrap((path) => { lookup(path); }),
    fchown: wrap((fd) => { fdEntry(fd); }),
    lchown: wrap((path) => { lookup(path); }),
    link: wrap(() => { throw fsError('ENOSYS'); }),
    symlink: wrap(() => { throw fsError('ENOSYS'); }),
    readlink: wrap(() => { throw fsError('EINVAL'); }),
  };

  function writeAt(e, buf, offset, length, position) {
    if (e.node.type === 'dir') throw fsError('EISDIR');
    const n = e.node;
    const at = (e.flags & constants.O_APPEND) ? n.size : (position ?? e.pos);
    ensureCapacity(n, at + length);
    n.data.set(buf.subarray(offset, offset + length), at);
    if (at + length > n.size) n.size = at + length;
    n.mtimeMs = Date.now();
    if (position == null) e.pos = at + length;
    return length;
  }

  // Go's syscall layer calls process.cwd(); the shim has no working directory but root.
  globalThis.process ??= {
    getuid: () => -1, getgid: () => -1, geteuid: () => -1, getegid: () => -1,
    getgroups: () => { throw fsError('ENOSYS'); },
    pid: -1, ppid: -1, umask: () => 0o022, cwd: () => '/', chdir: () => { throw fsError('ENOSYS'); },
  };
})();
