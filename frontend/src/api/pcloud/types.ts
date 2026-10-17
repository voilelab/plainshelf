// Wire types for the pCloud public API.
//
// Field names mirror the API exactly (lowercase, no underscores) so responses
// can be consumed without an intermediate rename.

/**
 * pCloud accounts live in exactly one region and the other host will not serve
 * them, so the host is part of a session's identity rather than a preference.
 * The region is discovered during authorization (see auth.ts) — never guessed.
 */
export const PCLOUD_API_HOSTS = ['api.pcloud.com', 'eapi.pcloud.com'] as const;

export type PCloudApiHost = (typeof PCLOUD_API_HOSTS)[number];

export const PCLOUD_DEFAULT_HOST: PCloudApiHost = 'api.pcloud.com';

/** A file or folder as returned by `listfolder`. */
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
  /** The listed folder's children. */
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
