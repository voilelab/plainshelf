
export const PCLOUD_API_HOSTS = ['api.pcloud.com', 'eapi.pcloud.com'] as const;

export type PCloudApiHost = (typeof PCLOUD_API_HOSTS)[number];

export const PCLOUD_DEFAULT_HOST: PCloudApiHost = 'api.pcloud.com';

export interface PCloudItem {
  name: string;
  isfolder: boolean;
  folderid?: number;
  fileid?: number;
  parentfolderid?: number;
  size?: number;
  contenttype?: string;
  modified?: string;
  created?: string;
  contents?: PCloudItem[];
}

interface PCloudResponse {
  result: number;
  error?: string;
}

export interface PCloudListFolderResult extends PCloudResponse {
  metadata?: PCloudItem;
}

export interface PCloudUserInfoResult extends PCloudResponse {
  userid?: number;
  email?: string;
  emailverified?: boolean;
}

export interface PCloudTokenResult extends PCloudResponse {
  access_token?: string;
  locationid?: number;
  userid?: number;
}
