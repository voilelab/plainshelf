# Deployment and threat model

PlainShelf is local-first and single-user. Where you bind the server and which
security mode you choose decide what it defends against — and, just as
important, what it does not. This page names three deployment tiers, gives the
configuration each one wants, and states plainly what protects access in each.

It also records two standing non-goals that hold at every tier:

- **PlainShelf holds no third-party credentials.** See
  [PlainShelf holds no third-party credentials](#plainshelf-holds-no-third-party-credentials).
- **`local_token` is a CSRF boundary, not access control.** See
  [What `local_token` actually protects](#what-local_token-actually-protects).

## Three deployment tiers

| Tier | Who it is for | `server_conf.addr` | `security.mode` | `security.protect_read` | `security.allowed_origins` | What actually controls access |
|---|---|---|---|---|---|---|
| **A** | One person, one machine (desktop app, local server) | `127.0.0.1:20000` | `local_token` (default) | `false` (default) | loopback defaults | The loopback bind: nothing off the machine can connect. |
| **B** | Home LAN / NAS, including VPN remote | LAN address or `0.0.0.0:20000` | `local_token` | `true` | your LAN and VPN origins | The network you trust — the LAN itself, or the VPN in front of it. |
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

The desktop and standalone reader apps embed the server in-process and open no
network port, so they are always effectively Tier A.

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
      - "http://nas.local:20000"
      - "http://192.168.1.10:20000"
      # add every hostname/port you actually open the UI through,
      # including the address you reach it at over the VPN
```

Two things to get right on this tier:

- **`allowed_origins` must list the origins you use.** The defaults cover only
  loopback, so a mutating request from `http://nas.local:20000` is rejected as a
  forbidden origin until you add it. Add each LAN hostname, LAN IP, and VPN
  address you open the UI through — with the exact scheme and port.
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

A real answer is planned. `security.mode` reserves two values —
`password` and `external` — for authenticated deployments; setting either today
fails at startup with "reserved but not implemented yet". A public tier waits on
that work landing. Until then, keep an internet-facing deployment behind a VPN
and treat it as Tier B.

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

## PlainShelf holds no third-party credentials

Whatever the tier, PlainShelf **does not store or forward any third-party service
credential.** If PlainShelf itself were compromised, there is no saved cloud
password, API key, or OAuth secret for an attacker to take from it. This has been
true by construction, not by policy, and three facts keep it that way:

- **The server has no config field for a secret.** `app_conf` carries shelf
  paths, a store path, import and logging settings, and the `security` block —
  and nothing that reads a password, token, or API key for any external service.
  The one token PlainShelf does handle, the `local_token`, is generated fresh at
  each startup and never persisted.
- **SMB credentials are the operating system's, not PlainShelf's.** PlainShelf
  reads an SMB/NAS shelf only through a path the OS has already mounted; it does
  not accept `smb://` URLs and never sees the share's username or password. See
  [Configure an SMB shelf file source](configuring-smb-shelf.md).
- **The Android app ships no pCloud secret.** pCloud access uses the
  `poll_token` OAuth flow, which needs no app secret and no redirect URL, so
  there is nothing confidential baked into the APK. The token a user grants
  lives on their device; PlainShelf's servers never see it, because PlainShelf
  runs no server in that path.

This is the local-first promise made concrete: no platform — including this one —
holds your library or your reading records, and none holds the keys to anywhere
else you store them.
