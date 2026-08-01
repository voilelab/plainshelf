export interface MobileBackEvent {
  canGoBack: boolean;
}

export interface MobileBackActions {
  selectionActive: boolean;
  downloadRunning: boolean;
  clearSelection: () => void;
  goBack: () => void;
  exitApp: () => void | Promise<void>;
}

export function handleLibraryMobileBack(event: MobileBackEvent, actions: MobileBackActions): void {
  if (actions.selectionActive && !actions.downloadRunning) {
    actions.clearSelection();
    return;
  }
  if (event.canGoBack) {
    actions.goBack();
    return;
  }
  void actions.exitApp();
}
