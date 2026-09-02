// Wire types for the pCloud public API.
//
// Field names mirror the API exactly (lowercase, no underscores) so responses
// can be consumed without an intermediate rename. Everything the shelf reader
// needs is present on a `listfolder` item, which is why a single recursive
// listing is enough to plan all subsequent reads.

/**
 * pCloud accounts live in exactly one region and the other host will not serve
 * them, so the host is part of a session's identity rather than a preference.
 * The region is discovered during authorization (see auth.ts) — never guessed.
 */
export const PCLOUD_API_HOSTS = ['api.pcloud.com', 'eapi.pcloud.com'] as const;

export type PCloudApiHost = (typeof PCLOUD_API_HOSTS)[number];

export const PCLOUD_DEFAULT_HOST: PCloudApiHost = 'api.pcloud.com';

/**
 * A file or folder as returned by `listfolder` and friends.
 *
 * `modified` is RFC 1123Z (e.g. "Sun, 16 Mar 2014 17:26:04 +0000"). Together
 * with `size` it is the pCloud equivalent of the `(mtime, size)` pair the Go
 * shelf uses to decide whether a cached book is stale — see
 * `internal/fsutil`/`shelf/filestate.go`. Both are available for every file in
 * the recursive listing, so freshness can be decided without extra requests.
 */
export interface PCloudItem {
  name: string;
  isfolder: boolean;
  /** Present when isfolder is true. The account's root folder is id 0. */
  folderid?: number;
  /** Present when isfolder is false. Stable across rename and move. */
  fileid?: number;
  parentfolderid?: number;
  size?: number;
  contenttype?: string;
  modified?: string;
  created?: string;
  /** Populated only when the listing was requested recursively. */
  contents?: PCloudItem[];
}

/** Envelope shared by every pCloud JSON response. `result` 0 means success. */
interface PCloudResponse {
  result: number;
  error?: string;
}

export interface PCloudListFolderResult extends PCloudResponse {
  /**
   * Optional so callers have to check. A successful pCloud listing always
   * carries it, but "successful" is only as trustworthy as the response being
   * from pCloud at all — see the envelope check in client.ts.
   */
  metadata?: PCloudItem;
}

/**
 * `getfilelink` returns a set of interchangeable hosts plus a path; the
 * download URL is `https://{hosts[0]}{path}`. The link expires, so it must be
 * fetched per download rather than cached alongside the file metadata.
 */
export interface PCloudGetFileLinkResult extends PCloudResponse {
  hosts?: string[];
  path?: string;
  expires?: string;
  size?: number;
}

/**
 * `getziplink` returns the same host/path/expiry shape as `getfilelink`, so the
 * download URL is `https://{hosts[0]}{path}`. Unlike `savezip` it leaves no file
 * behind in the account, which keeps this client's shelf read-only.
 */
export interface PCloudGetZipLinkResult extends PCloudResponse {
  hosts?: string[];
  path?: string;
  expires?: string;
}

/** Identity returned by `userinfo` for the account behind a bearer token. */
export interface PCloudUserInfoResult extends PCloudResponse {
  userid?: number;
  email?: string;
  emailverified?: boolean;
}

/** Response of the `poll_token` OAuth flow's `oauth2_token` long poll. */
export interface PCloudTokenResult extends PCloudResponse {
  access_token?: string;
  /** 1 = US (api.pcloud.com), 2 = EU (eapi.pcloud.com). */
  locationid?: number;
  userid?: number;
}
