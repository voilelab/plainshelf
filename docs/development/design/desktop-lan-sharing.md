# Desktop LAN sharing decision record

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
[Share shelves on your local network](../../configuring-local-shelf.md#share-shelves-on-your-local-network-desktop-app):
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
covered by [Shelf sources and credentials](../../concepts/shelf-sources.md).
