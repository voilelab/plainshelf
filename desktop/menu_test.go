package main

import (
	"testing"
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := historyNavigationScript(tc.step); got != tc.want {
				t.Fatalf("historyNavigationScript(%d) = %q, want %q", tc.step, got, tc.want)
			}
		})
	}
}

func TestApplicationMenuAccelerators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos      string
		label     string
		wantAccel string
	}{
		{goos: "darwin", label: "Previous Page", wantAccel: "CmdOrCtrl+Left"},
		{goos: "darwin", label: "Next Page", wantAccel: "CmdOrCtrl+Right"},
		{goos: "linux", label: "Previous Page", wantAccel: "OptionOrAlt+Left"},
		{goos: "windows", label: "Next Page", wantAccel: "OptionOrAlt+Right"},
		{goos: "windows", label: "Zoom In", wantAccel: "CmdOrCtrl+="},
		{goos: "darwin", label: "Reset Zoom", wantAccel: "CmdOrCtrl+0"},
		{goos: "windows", label: "Library", wantAccel: "CmdOrCtrl+Shift+L"},
	}

	for _, tc := range tests {
		t.Run(tc.goos+" "+tc.label, func(t *testing.T) {
			t.Parallel()

			item := findMenuItem(t, applicationMenu(NewDesktopApp(&fakeNative{}), tc.goos), tc.label)
			if item.accel != tc.wantAccel {
				t.Fatalf("%s on %s accelerator = %q, want %q", tc.label, tc.goos, item.accel, tc.wantAccel)
			}
		})
	}
}

func TestApplicationMenuAppRoleOnMacOnly(t *testing.T) {
	t.Parallel()

	app := NewDesktopApp(&fakeNative{})
	if got := applicationMenu(app, "darwin")[0].role; got != appMenuRole {
		t.Fatalf("darwin first menu role = %v, want app menu", got)
	}
	for _, spec := range applicationMenu(app, "windows") {
		if spec.role == appMenuRole {
			t.Fatal("windows menu includes the macOS app menu")
		}
	}
}

func TestApplicationMenuActionsRunScripts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label string
		want  string
	}{
		{label: "Previous Page", want: historyNavigationScript(-1)},
		{label: "Next Page", want: historyNavigationScript(1)},
		{label: "Zoom In", want: zoomScript("in")},
		{label: "Zoom Out", want: zoomScript("out")},
		{label: "Reset Zoom", want: zoomScript("reset")},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()

			native := &fakeNative{}
			findMenuItem(t, applicationMenu(NewDesktopApp(native), "linux"), tc.label).action()
			if len(native.scripts) != 1 || native.scripts[0] != tc.want {
				t.Fatalf("%s ran scripts %q, want [%q]", tc.label, native.scripts, tc.want)
			}
		})
	}
}

func TestApplicationMenuTranslates(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"darwin", "linux", "windows"} {
		if _, err := toWailsMenu(applicationMenu(NewDesktopApp(&fakeNative{}), goos)); err != nil {
			t.Fatalf("toWailsMenu on %s: %v", goos, err)
		}
	}
}

func findMenuItem(t *testing.T, specs []menuSpec, label string) menuItem {
	t.Helper()
	for _, spec := range specs {
		for _, item := range spec.items {
			if item.label == label {
				return item
			}
		}
	}
	t.Fatalf("menu item %q not found", label)
	return menuItem{}
}

func TestZoomScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		want   string
	}{
		{
			name:   "zoom in",
			action: "in",
			want:   "window.__plainshelfZoom && window.__plainshelfZoom('in');",
		},
		{
			name:   "zoom out",
			action: "out",
			want:   "window.__plainshelfZoom && window.__plainshelfZoom('out');",
		},
		{
			name:   "reset zoom",
			action: "reset",
			want:   "window.__plainshelfZoom && window.__plainshelfZoom('reset');",
		},
		{
			name:   "unknown action",
			action: "unknown",
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := zoomScript(tc.action); got != tc.want {
				t.Fatalf("zoomScript(%q) = %q, want %q", tc.action, got, tc.want)
			}
		})
	}
}
