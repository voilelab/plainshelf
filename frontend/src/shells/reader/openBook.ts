interface ReaderBindingWindow extends Window {
  go?: {
    main?: {
      ReaderApp?: {
        OpenBookPackage?: () => Promise<string>;
      };
    };
  };
}

export function hasOpenBookBinding(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }
  return Boolean((window as ReaderBindingWindow).go?.main?.ReaderApp?.OpenBookPackage);
}

export async function openBookPackage(): Promise<void> {
  const open = (window as ReaderBindingWindow).go?.main?.ReaderApp?.OpenBookPackage;
  if (!open) {
    throw new Error('OpenBookPackage binding not available');
  }

  await open();
}
