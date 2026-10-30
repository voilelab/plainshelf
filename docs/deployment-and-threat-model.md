# Deployment and threat model

PlainShelf is local-first and single-user. Where you bind the server and which
security mode you choose decide what it defends against and what it does not.
This page names three deployment tiers, gives the configuration each one wants,
and states what protects access in each. It then does the same for the desktop
and reader apps, which are neither.

For the type and default of each `security` key on its own, see the
[Configuration reference](reference/configuration.md#app_confsecurity); this
page is about which combinations to choose.

Two boundaries apply at every tier:

- **PlainShelf operates no service that holds your data or credentials.** Your
  own server or device does read the secrets needed for configured remote
  sources, and may persist export tokens. No confidential app secret is baked
  into the binaries. See [Where credentials live](#where-credentials-live).
- **`local_token` is a CSRF boundary, not access control.** See
  [What `local_token` actually protects](#what-local_token-actually-protects).

## Three deployment tiers

| Tier | Who it is for | `server_conf.addr` | `security.mode` | `security.protect_read` | `security.allowed_origins` | What actually controls access |
|---|---|---|---|---|---|---|
| **A** | One person, one machine (a local server you run) | `127.0.0.1:20000` | `local_token` (default) | `false` (default) | loopback defaults | The loopback bind: nothing off the machine can connect. |
| **B** | Home LAN / NAS, including VPN remote | LAN address or `0.0.0.0:20000` | `local_token` | `true` | loopback **and** your LAN and VPN origins | The network you trust — the LAN itself, or the VPN in front of it. |
| **C** | The public internet | — | *not supported yet* | — | — | Nothing PlainShelf ships. Do not deploy here. |

### Tier A — loopback, single user

This is the default and needs no configuration. The server binds to loopback,
so only the machine it runs on can reach it, and `local_token` is on to stop a
web page open in your browser from forging requests to it.

```yaml
server_conf:
  addr: "127.0.0.1:20000"
app_conf:
  security:
    mode: local_token   # the default when security is unset; shown for clarity
```

### Tier B — home LAN / NAS, including VPN remote

This is what the Docker image is really for: a shelf on a NAS or home server that
the household reads over the LAN, and that you occasionally reach from outside
through a VPN. The access boundary here is **the network, not PlainShelf**: a
trusted LAN, or a VPN (WireGuard, Tailscale, and the like) that only your own
devices can join. PlainShelf authenticates no one — it assumes everyone who can
reach the port is someone you trust.

Given that, keep `mode: local_token`. It is not access control (see
[below](#what-local_token-actually-protects)), but it keeps the browser-CSRF and
origin checks that `mode: none` throws away, at no cost to the people you do
trust.

```yaml
server_conf:
  addr: "0.0.0.0:20000"          # reachable from the LAN
app_conf:
  security:
    mode: local_token
    protect_read: true           # require the token for reads too, not only writes
    allowed_origins:
      # Setting this list REPLACES the loopback defaults, it does not add to
      # them — so keep the local addresses if you ever open the UI on the
      # machine itself.
      - "http://127.0.0.1:20000"
      - "http://localhost:20000"
      - "http://nas.local:20000"
      - "http://192.168.1.10:20000"
      # add every hostname/port you actually open the UI through,
      # including the address you reach it at over the VPN
```

Two things to get right on this tier:

- **`allowed_origins` replaces the defaults, it does not extend them.**
  PlainShelf applies its four loopback defaults (`http://127.0.0.1:20000`,
  `http://localhost:20000`, and the same two on the Vite dev port `5173`) *only
  while the list is empty* (`server/security.go`). Set it, and those defaults
  are gone: a browser opened on the NAS itself at `http://127.0.0.1:20000` then
  gets `403 forbidden origin` on every write. So the list has to name every
  origin you use, loopback included, with the exact scheme and port: each LAN
  hostname, each LAN IP, each VPN address, and the local ones you would
  otherwise lose.
- **`protect_read: true` is defence in depth, not a login.** It makes even read
  requests carry the token, so a bare API scrape that never loads a page gets
  `401`. It does *not* keep anyone out who opens the Web UI: the token is handed
  to every page load (again, see [below](#what-local_token-actually-protects)),
  so on a network where you do not trust every device, this setting is not the
  control you want — the network boundary is.

!!! warning "Do not reach for `mode: none`"
    `mode: none` on a non-loopback address disables the token and the origin
    checks entirely, and any website open in any browser on that network can
    then drive your server. The Web UI shows a persistent "no API auth" warning
    whenever it detects this. Use `local_token` and a trusted network instead.

### Tier C — the public internet

**Not supported.** PlainShelf has no password or session login, so there is no
configuration that makes it safe to expose directly to the public internet, and
this page will not offer a workaround that pretends otherwise. Putting a reverse
proxy in front does not change this: `local_token` hands its token to every
page load, so the first visitor to reach the proxy can read and write your
shelf.

A real answer is planned. `security.mode` reserves two values, `password` and
`external`, for authenticated deployments; setting either today fails at startup
with "reserved but not implemented yet". A public tier waits on that work
landing. Until then, keep an internet-facing deployment behind a VPN and treat
it as Tier B.

### The desktop and standalone reader apps

Neither app is a row on that table. Both embed their API in the Wails window and
open **no network port** for it — neither serves the API with `ListenAndServe`,
so there is nothing for another machine to connect to, and the tiers' question
of who can reach the port does not arise. The desktop's optional LAN share is
not the API; see [the read-only WebDAV export](#the-read-only-webdav-export). What they are not is `local_token`:

- The **desktop app** runs the same server in-process with `security.mode: none`
  hard-coded (`desktop/app.go`). Under `none` the middleware returns before the
  token check, the origin allowlist and CORS (`server/security.go`), so its API
  has no token and no origin check. The static browser-hardening headers
  (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`) are still set.
- The **standalone reader** does not go through `server/security.go` at all. It
  serves its own `readerapi` handler from the Wails asset server
  (`reader/main.go`), so it has no `security.mode` to set, and likewise no token
  and no origin check.

What protects them is the process boundary rather than a mode: the API answers
only inside the app's own webview, and a browser you open separately cannot reach
it, because there is no port to connect to. Against the web-origin CSRF that
Tier A spends its token on, that is the stronger position of the two rather than
a weaker one — Tier A's loopback port *is* reachable from a page in your ordinary
browser, which is exactly why it needs the token, while here the request cannot
be made at all.

The missing mode therefore costs these two apps nothing, because what remains is
local rather than networked and a token would not close it either: any program
already running as you can read the shelf files directly, with or without
PlainShelf, and anything that can inject script into the app's own webview is
inside the boundary already — under `local_token` too, since the token is written
into the page (`server/spa.go`) where same-origin script can simply read it.

### The read-only WebDAV export

`dav_export` (off by default) opens a **second** listener that serves every local
shelf's files read-only, for another PlainShelf to open as a virtual shelf. It
sits outside the `local_token` boundary on purpose, and is gated by its own token
instead:

- Every request needs `Authorization: Bearer <token>`, compared in constant
  time; the token lives in `token_file` (mode `0600`) and is replaced with
  `plainshelf-srv -rotate-dav-token`, which takes effect on the next request.
- No method that writes is answered, with or without the token, and the
  listener has no `/api/` route, no web UI and no shelf management.
- Only shelf files are served: lock files, the scan cache and `app/tmp/` are not.
- Over plain HTTP the token and the files cross the network unencrypted, so the
  export belongs on Tier B's trusted network only. See
  [`dav_export`](reference/configuration.md#dav_export).

The desktop app's **Share on local network** switch opens the same listener,
limited to the shelves ticked on the settings page. It is turned on only through
a Wails binding, never an HTTP route, so nothing on the network can switch it
on; it binds one detected LAN address rather than `0.0.0.0`, and off it opens no
port. See
[Share shelves on your local network](configuring-local-shelf.md#share-shelves-on-your-local-network-desktop-app).

The Android app's **Share on local network** opens it the same way for ticked
local shelves, through a Capacitor plugin rather than an HTTP route. It runs only
while a foreground service shows a notification, binds the phone's Wi-Fi IPv4
address, and closes itself after 30 minutes without a request the token admitted
or when that network changes. See
[Share local shelves on the local network](development/android.md#share-local-shelves-on-the-local-network).

### Desktop LAN sharing decision record

**Recorded retrospectively on 2026-10-20 (Asia/Taipei).** This is the date this
record was written, not the original decision date, which has not been
established. The implementation is traceable to
[the separate WebDAV export (#741)](https://github.com/voilelab/plainshelf/pull/741)
and [the desktop sharing switch (#745)](https://github.com/voilelab/plainshelf/pull/745),
both committed on 2026-10-17, followed by
[per-shelf QR URLs (#757)](https://github.com/voilelab/plainshelf/pull/757)
on 2026-10-18.

The requirement is to let another trusted device read selected local shelves
while their owner uses the desktop app, without a second server installation
or remote control of the desktop. The alternatives were:

| Option | Benefit | Cost and decision |
|---|---|---|
| No desktop sharing; run a separate server or Docker | Keeps the desktop entirely in-process and offers an independently managed service. | Requires separate setup and shelf configuration for a simple device-to-device read. Still available for NAS or always-on use, but not the chosen desktop experience. |
| Expose the complete desktop API on the network | Reuses the UI API and its management operations. | Exposes writes and management beyond the reading requirement. The desktop's hard-coded `SecurityModeNone` relies on its process boundary and cannot secure a network listener. Rejected. |
| Separate, read-only WebDAV listener | Reuses the shelf-file export, restricts the shared shelves, and gives remote readers only the access they need. | Adds listener lifecycle and bearer-token handling; HTTP provides no transport confidentiality. Chosen within the trusted-network boundary. |

The decision applies to the desktop's optional shelf-file export, not to its
complete UI API, which remains inside Wails. The export has its own bearer
token; it neither inherits `SecurityModeNone` as network protection nor uses
`local_token` as access control. A token holder can read the selected shelf
files but cannot write, manage shelves, or call the desktop API.

The deployment tiers above remain the framework: the desktop API stays
in-process; an enabled export belongs only on **Tier B**, a trusted LAN or VPN.
Its single local IPv4 bind is not a guarantee that the network is trusted;
choose the interface and firewall rules accordingly. HTTP transmits both token
and content in cleartext. This decision adds no TLS, login, write access or
**Tier C** public deployment support, and preserves single-user operation on
trusted devices.

The operational contract is documented in
[Share shelves on your local network](configuring-local-shelf.md#share-shelves-on-your-local-network-desktop-app):
sharing starts disabled with no selected shelves; an enabled share binds one
local LAN IPv4 address (never `0.0.0.0`), defaults to port `20001`, and exposes
only the selected local shelves. The settings page reports errors and shows
the URL, token and QR while running. Turning sharing off closes the listener,
including active transfers. Saved settings and the token survive restarts;
replacing the token revokes the old token for subsequent requests.

This record documents the implemented boundary. It does not establish that
native Wails interaction, firewall prompts, cross-device reading, or port
release after switching off have been verified on a physical desktop. Those
remain separate device-validation work. Broader source-credential storage is
covered by [Shelf sources and credentials](concepts/shelf-sources.md).

## What `local_token` actually protects

`local_token` mode generates a random token when the server starts and writes it,
in cleartext, into a `<script>` tag in `index.html` — that is how the frontend
picks it up on load. So **anyone who can load the homepage already has the
token.** On a loopback bind that is only you; on a LAN bind it is everyone on the
network.

That is the whole point to understand: `local_token` is **not access control.**
It does not authenticate a user and it does not keep a reachable network from
reading or writing your shelf. What it defends is narrower and still worth having:

- **Web-origin CSRF.** A malicious or compromised website open in your browser
  cannot silently forge a mutating request to PlainShelf, because it cannot read
  the token out of PlainShelf's own page (the browser's same-origin policy stops
  it) and its requests carry a foreign `Origin` that the allowlist rejects.
- Combined with `protect_read: true`, it also turns away an unauthenticated API
  client that never loads a page at all.

Read `mode: local_token` as "CSRF protection for a machine you already control,"
never as "a login." When you need a login, that is Tier C, and it is not built
yet.

### Exactly which requests need the token

The rule is not "every POST." It is:

| Request | Needs the token |
|---|---|
| `/health` and anything outside `/api/` | No |
| Reads of the shelf (`GET`) | Only with `protect_read: true` |
| Refreshing the book list (`POST .../scans`) | Only with `protect_read: true` |
| Every other write | Always |
| `/api/logs` | Always, whatever `protect_read` says |

Two of those rows are deliberate exceptions rather than oversights.

**Refreshing the book list is a read.** It walks the shelf and rebuilds the
server's cache; it changes nothing on disk, and its reply is two counts. So
`protect_read` governs it the way it governs any other read, and read-only mode
accepts it. Requiring a token for it would have contradicted this page for the
common case: the shipped defaults are `local_token` with `protect_read: false`,
under which a user is told reading needs no token and then shown a refresh
button that always failed. It is still not exempt from CSRF — a browser attaches
`Origin` to every cross-site `POST`, so a refresh arriving from a page that is
not in `allowed_origins` is refused, which is what stops a random website making
your server walk an SMB shelf. A request carrying no `Origin` at all is not a
browser and cannot be a forged cross-site request; the Android client is the
case that matters, since its native HTTP bridge sends none.

The token was never what stopped a device on your own network from looping that
request: it is printed into every page the server sends, so anything that can
reach the port can read it. The refresh is rate limited instead — five walks back
to back, then one more every 10 seconds, and `429` beyond that. That is what
bounds the CPU and SMB bandwidth a compromised or misconfigured LAN device can
take, with or without a token, and it is set far above the pace of pressing the
button by hand. See [Rescanning on demand](concepts/shelf-cache-and-io.md#rescanning-on-demand).

**`/api/logs` is not shelf content.** The logs record every request path, and so
the shelf's structure, together with access times and remote addresses. Turning
`protect_read` off to let the household read books says nothing about publishing
that, so the logs stay behind the token either way. It is not a setting, for the
same reason a safe default is not a choice worth offering.

## Where credentials live

PlainShelf operates no hosted service that holds your accounts, library,
reading records or credentials. This is a statement about the project, not a
claim that a server you run never handles secrets. A server or device opening a
remote shelf must resolve its credential and use it to authenticate to that
source. A saved reference keeps the secret out of the shelf configuration; it
does not keep the plaintext secret out of the running process.

### Remote source credentials

| Client / connection | Secret and purpose | Persistent location | Sent to |
|---|---|---|---|
| Server / Docker → WebDAV source | WebDAV password with `username`, or bearer token without it | Operator-provided environment variable (`env:NAME`) or file (`file:/path`) | The configured WebDAV source, as HTTP Basic or `Authorization: Bearer` |
| Server / Docker → PlainShelf export | Export bearer token granting read access to the remote shelf files | Operator-provided environment variable or file, referenced by `source.credential` | The configured export URL, as `Authorization: Bearer` |
| Desktop → WebDAV or PlainShelf export | WebDAV password / bearer token, or PlainShelf export bearer token | OS keychain: macOS Keychain, Windows Credential Manager or Linux Secret Service; `shelves.json` stores a `host:` reference | The configured source URL, using the same Basic / bearer rules |
| Android → WebDAV or PlainShelf export | WebDAV password / bearer token, or PlainShelf export bearer token | Keystore-backed secure storage, per shelf entry | The configured source URL, using the same Basic / bearer rules |
| Android → pCloud | OAuth access token granting access to the user's pCloud account | Keystore-backed secure storage, per shelf entry | The authorized regional pCloud API (`api.pcloud.com` or `eapi.pcloud.com`) over HTTPS, as a bearer header; never to a PlainShelf server |
| Android → full PlainShelf UI API (`server` entry) | That server's `local_token`; needed for reads when `protect_read: true` | Keystore-backed secure storage, per shelf entry | The configured server API, in the token header (default `X-PlainShelf-Token`); this is not an export connection |

The server's `DefaultCredentialResolver` reads `env:` and `file:` references.
The desktop resolves `host:` through the OS keychain; Android uses its native
secure-storage resolver. The source client holds the resolved value to make
requests. Source credentials are configured through the server's local config
or host bindings, not through a route that accepts or returns secrets on the UI
HTTP API. See [Shelf sources and credentials](concepts/shelf-sources.md#credentials)
and the [source configuration reference](reference/configuration.md#shelvessource).

On Android, secure storage encrypts values in private SharedPreferences with an
AES-GCM key held in Android Keystore. Non-secret shelf settings live separately
in Capacitor Preferences; per-entry secrets use
`plainshelf.mobile.secret.<entryID>`. The app can decrypt them to use them, so
storage encryption does not protect against an attacker controlling the app and
device. pCloud's `poll_token` authorization needs no app secret or redirect URI;
no confidential pCloud app secret ships in the APK. If the device is lost,
revoke its pCloud grant from the account settings.

HTTP Basic encodes credentials; it does not encrypt them. A WebDAV source may
use HTTPS, but PlainShelf's own LAN export currently speaks plain HTTP: its
token and shelf contents are visible to anyone able to observe that traffic.
Keep it on Tier B's trusted LAN / VPN, never the public internet. The same
transport concern applies to a full-API token sent over HTTP; a token is not a
substitute for transport protection or a user login.

### Locally generated tokens

These have different purposes from remote-source passwords and OAuth tokens:

- **UI API `local_token`:** generated afresh at server startup, not persisted by
  the server, and delivered to its web UI on page load. Android may save a copy
  for a full-API connection. It is the [CSRF boundary](#what-local_token-actually-protects),
  not user authentication or the LAN export's bearer token.
- **Server WebDAV export token:** generated into `dav_export.token_file` with
  mode `0600`, persists across restarts, and is checked on export requests.
  The operator supplies a copy to reading clients; the file must not be in the
  exported tree. See [Token file location](reference/configuration.md#token-file-location).
- **Desktop LAN share token:** generated into `lan_share.token` in the desktop
  data root, outside shelves. It persists independently of imported source
  credentials in the keychain. The running share displays it and includes it in
  the QR payload; replacing it invalidates the old token on subsequent requests.
- **Android LAN share token:** generated into `lan_share.token` in the embedded
  runtime's private data directory. This publisher token is a local file, not a
  source credential in Keystore-backed storage. It persists and can be rotated;
  the running share exposes it to the user through its connection details / QR.

Export tokens authorize read access to the files the listener exports. They do
not enable writes, shelf management, or access to the desktop's in-process UI
API. `SecurityModeNone` still relies on the Wails process boundary; neither this
credential model nor the export changes `allowed_origins` replacement semantics.

SMB remains an OS-mounted filesystem: PlainShelf receives a mount path and does
not handle the SMB account or password. See
[Configure an SMB shelf file source](configuring-smb-shelf.md).

Shelf files, host configuration, secret stores and export-token files are
separate storage. Copying a shelf or a `host:` reference does not copy its
keychain / Keystore credential, while copying an export-token file does copy a
read-access secret. Consult [Backup and restore](backup-and-restore.md) for the
existing host procedures; protect any credential files you back up and plan to
reconnect sources whose secret store was not restored.
