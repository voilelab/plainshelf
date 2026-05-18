/// <reference types="vite/client" />

interface ImportMetaEnv {
	readonly VITE_API_BASE?: string;
	readonly VITE_USE_MOCK_API?: string;
	readonly VITE_PLAINSHELF_TOKEN?: string;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
}


interface Window {
  plainshelf?: {
    getApiToken?: () => string | Promise<string>;
    getApiTokenHeader?: () => string | Promise<string>;
    getApiBaseURL?: () => string | Promise<string>;
    getProfileDir?: () => string | Promise<string>;
  };
}
