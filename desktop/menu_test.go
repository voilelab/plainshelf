package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

func TestHistoryNavigationScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		step int
		want string
	}{
		{
			name: "previous page",
			step: -1,
			want: "window.history.back();",
		},
		{
			name: "next page",
			step: 1,
			want: "window.history.forward();",
		},
		{
			name: "unsupported step",
			step: 0,
			want: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := historyNavigationScript(tc.step); got != tc.want {
				t.Fatalf("historyNavigationScript(%d) = %q, want %q", tc.step, got, tc.want)
			}
		})
	}
}

func TestHistoryMenuAcceleratorFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goos string
		key  string
		want *keys.Accelerator
	}{
		{
			name: "mac uses cmd",
			goos: "darwin",
			key:  "left",
			want: keys.CmdOrCtrl("left"),
		},
		{
			name: "linux uses alt",
			goos: "linux",
			key:  "right",
			want: keys.OptionOrAlt("right"),
		},
		{
			name: "windows uses alt",
			goos: "windows",
			key:  "left",
			want: keys.OptionOrAlt("left"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := historyMenuAcceleratorFor(tc.goos, tc.key)
			if got.Key != tc.want.Key {
				t.Fatalf("historyMenuAcceleratorFor(%q, %q) key = %q, want %q", tc.goos, tc.key, got.Key, tc.want.Key)
			}

			if len(got.Modifiers) != len(tc.want.Modifiers) {
				t.Fatalf(
					"historyMenuAcceleratorFor(%q, %q) modifiers length = %d, want %d",
					tc.goos,
					tc.key,
					len(got.Modifiers),
					len(tc.want.Modifiers),
				)
			}

			for index := range got.Modifiers {
				if got.Modifiers[index] != tc.want.Modifiers[index] {
					t.Fatalf(
						"historyMenuAcceleratorFor(%q, %q) modifier[%d] = %q, want %q",
						tc.goos,
						tc.key,
						index,
						got.Modifiers[index],
						tc.want.Modifiers[index],
					)
				}
			}
		})
	}
}

func TestHistoryNavigationWithNilContext(t *testing.T) {
	t.Parallel()

	app := NewDesktopApp()
	app.PreviousPage()
	app.NextPage()
}

func TestZoomScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		factor float64
		want   string
	}{
		{
			name:   "default zoom",
			factor: 1.0,
			want:   "document.body.style.zoom = '1.00';",
		},
		{
			name:   "zoom in",
			factor: 1.1,
			want:   "document.body.style.zoom = '1.10';",
		},
		{
			name:   "zoom out",
			factor: 0.9,
			want:   "document.body.style.zoom = '0.90';",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := zoomScript(tc.factor); got != tc.want {
				t.Fatalf("zoomScript(%.2f) = %q, want %q", tc.factor, got, tc.want)
			}
		})
	}
}

func TestZoomWithNilContext(t *testing.T) {
	t.Parallel()

	app := NewDesktopApp()
	app.ZoomIn()
	app.ZoomOut()
	app.ResetZoom()
}

func TestZoomClamping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		operations func(app *DesktopApp)
		wantZoom   float64
	}{
		{
			name: "clamps at min",
			operations: func(app *DesktopApp) {
				app.zoomFactor = minZoomFactor
				app.ZoomOut()
			},
			wantZoom: minZoomFactor,
		},
		{
			name: "clamps at max",
			operations: func(app *DesktopApp) {
				app.zoomFactor = maxZoomFactor
				app.ZoomIn()
			},
			wantZoom: maxZoomFactor,
		},
		{
			name: "reset returns to default",
			operations: func(app *DesktopApp) {
				app.zoomFactor = 1.5
				app.ResetZoom()
			},
			wantZoom: defaultZoomFactor,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := NewDesktopApp()
			tc.operations(app)
			if got := app.GetZoomFactor(); got != tc.wantZoom {
				t.Fatalf("GetZoomFactor() = %.2f, want %.2f", got, tc.wantZoom)
			}
		})
	}
}
