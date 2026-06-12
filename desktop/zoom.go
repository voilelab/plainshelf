package main

import "fmt"

const (
	zoomInAction    = "in"
	zoomOutAction   = "out"
	zoomResetAction = "reset"
)

func zoomScript(action string) string {
	if action != zoomInAction && action != zoomOutAction && action != zoomResetAction {
		return ""
	}

	return fmt.Sprintf(`(() => {
  const minZoom = 0.5;
  const maxZoom = 2;
  const step = 0.1;
  const action = %q;
  const root = document.documentElement;
  const currentZoom = Number(root.dataset.plainshelfZoom || '1');
  const nextZoom = action === 'reset'
    ? 1
    : Math.min(maxZoom, Math.max(minZoom, currentZoom + (action === 'in' ? step : -step)));
  const roundedZoom = Math.round(nextZoom * 100) / 100;

  root.dataset.plainshelfZoom = String(roundedZoom);
  if (roundedZoom === 1) {
    root.style.removeProperty('zoom');
  } else {
    root.style.setProperty('zoom', String(roundedZoom));
  }
})();`, action)
}
