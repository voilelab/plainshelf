interface DesktopSelectedBookFile {
  path?: string;
  name?: string;
  content?: string;
}

interface DesktopAppBinding {
  OpenBookFiles?: () => Promise<DesktopSelectedBookFile[]>;
}

interface DesktopWindow extends Window {
  go?: {
    main?: {
      DesktopApp?: DesktopAppBinding;
    };
  };
}

export function isDesktopRuntime(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const params = new URLSearchParams(window.location.search);
  return (
    window.location.protocol === 'wails:' ||
    window.location.host.endsWith('.wails.localhost') ||
    params.get('desktop-shell-preview') === '1'
  );
}

function decodeBase64(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function filenameFromPath(path: string): string {
  const normalized = path.replace(/\\/g, '/');
  const parts = normalized.split('/');
  return parts[parts.length - 1] || 'book.txt';
}

export async function openDesktopBookFiles(): Promise<File[] | null> {
  if (!isDesktopRuntime()) {
    return null;
  }

  const desktopApp = (window as DesktopWindow).go?.main?.DesktopApp;
  if (!desktopApp?.OpenBookFiles) {
    return null;
  }

  const selectedFiles = await desktopApp.OpenBookFiles();
  return (selectedFiles ?? []).map((item) => {
    const name = item.name?.trim() || filenameFromPath(item.path ?? '');
    const bytes = decodeBase64(item.content ?? '');
    const content = new ArrayBuffer(bytes.length);
    new Uint8Array(content).set(bytes);
    return new File([content], name, { type: 'text/plain' });
  });
}
