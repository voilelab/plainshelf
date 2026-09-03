# Logs

PlainShelf writes one log file per day and keeps them under the log directory
its configuration names. The desktop app writes to
`<user config dir>/PlainShelf/logs`; a server writes to the `dir` under
`log_file` in its config file.

## Reading a log

Open **Settings → Logs** in the sidebar's admin section to pick a log by name
and date.

The page shows the **end** of the file, not all of it. A log file grows for as
long as the day lasts, and every HTTP request adds a line, so a busy day can
leave a file far larger than a browser can render at once. When the page is
showing only part of a file it says so above the content, and **Load more**
reaches further back.

## Reporting a problem

Every response the server sends carries a request number in the `X-Request-Id`
header, and every error it answers with repeats that number in the response
body as `incident`. It is eight characters long and deliberately avoids the
characters people mix up when reading a number aloud — no `0`, `O`, `1`, `I` or
`L` appears in one.

Include that number in a bug report. Searching a log file for it finds the line
for that request, and for a failure the server did not expect that line also
carries the route, the shelf, and the underlying error the response itself
withholds. Background work — a batch move, emptying the trash, fingerprinting —
is logged under the number of the request that started it, so one number covers
the whole operation.

The number is currently visible in the response rather than on screen; reading
it takes the browser's developer tools, or `curl -i` against the API. Numbers
are also only useful while the log that holds them still exists, so report a
problem before the retention window below expires, or copy the log file aside.

## Retention

Rotated log files are deleted once they are older than the retention window.
**Nothing else deletes them**, so without a window a log directory grows for as
long as the application is installed.

The default window is **30 days**. Change it in **Settings → Logs**:

- A number of days: files older than that are deleted.
- `0`: no log file is ever deleted.

Deletion happens when the log rotates — the first time the application writes a
log line on a new day — so an application that is not running deletes nothing,
and one that has been idle for a while cleans up when it next writes. A change
saved from the settings page applies to the next rotation; there is no need to
restart.

Only files the log page itself lists can be deleted: a file is a log file when
it sits directly in the log directory and its name is the configured prefix, a
`YYYY-MM-DD` date and `.log`. Anything else in that directory — including
subdirectories and symbolic links — is left alone.

A server can seed the window from its config file:

```yaml
app_conf:
  logger:
    log_file:
      type: filename_rotate
      dir: ./logs
      prefix: root
      retention_days: 30   # 0 keeps every file
```

A value saved from the settings page takes precedence over the config file.
Deleting the saved value reverts to the config file, and then to the built-in
default of 30 days. The desktop app has no config file, so the settings page is
the only place its window is set.
