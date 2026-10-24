/// <reference types="vite/client" />

interface ImportMetaEnv {
	readonly VITE_API_BASE?: string;
	readonly VITE_USE_MOCK_API?: string;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
}

declare module 'opencc-js/dict/*' {
	const dict: string;
	export default dict;
}
