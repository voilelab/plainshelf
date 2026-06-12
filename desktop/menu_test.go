package main

import (
	"strings"
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
		name       string
		action     string
		wantAction string
		wantEmpty  bool
	}{
		{
			name:       "zoom in",
			action:     zoomInAction,
			wantAction: `const action = "in";`,
		},
		{
			name:       "zoom out",
			action:     zoomOutAction,
			wantAction: `const action = "out";`,
		},
		{
			name:       "reset zoom",
			action:     zoomResetAction,
			wantAction: `const action = "reset";`,
		},
		{
			name:      "unsupported action",
			action:    "unsupported",
			wantEmpty: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := zoomScript(tc.action)
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("zoomScript(%q) = %q, want empty script", tc.action, got)
				}
				return
			}

			if got == "" {
				t.Fatalf("zoomScript(%q) returned empty script", tc.action)
			}
			if !strings.Contains(got, tc.wantAction) {
				t.Fatalf("zoomScript(%q) does not contain %q; got %q", tc.action, tc.wantAction, got)
			}
			if !strings.Contains(got, "root.style.setProperty('zoom'") {
				t.Fatalf("zoomScript(%q) does not set CSS zoom; got %q", tc.action, got)
			}
		})
	}
}

func TestZoomMenuAcceleratorFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		goos   string
		action string
		want   *keys.Accelerator
	}{
		{
			name:   "mac zoom in uses shifted equals for plus",
			goos:   "darwin",
			action: zoomInAction,
			want:   keys.Combo("=", keys.CmdOrCtrlKey, keys.ShiftKey),
		},
		{
			name:   "windows zoom in uses plus key name",
			goos:   "windows",
			action: zoomInAction,
			want:   keys.CmdOrCtrl("plus"),
		},
		{
			name:   "zoom out",
			goos:   "linux",
			action: zoomOutAction,
			want:   keys.CmdOrCtrl("-"),
		},
		{
			name:   "reset zoom",
			goos:   "linux",
			action: zoomResetAction,
			want:   keys.CmdOrCtrl("0"),
		},
		{
			name:   "unsupported action",
			goos:   "linux",
			action: "unsupported",
			want:   nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := zoomMenuAcceleratorFor(tc.goos, tc.action)
			assertAcceleratorEqual(t, got, tc.want)
		})
	}
}

func TestZoomActionsWithNilContext(t *testing.T) {
	t.Parallel()

	app := NewDesktopApp()
	app.ZoomIn()
	app.ZoomOut()
	app.ResetZoom()
}

func assertAcceleratorEqual(t *testing.T, got *keys.Accelerator, want *keys.Accelerator) {
	t.Helper()

	if got == nil || want == nil {
		if got != want {
			t.Fatalf("accelerator = %#v, want %#v", got, want)
		}
		return
	}

	if got.Key != want.Key {
		t.Fatalf("accelerator key = %q, want %q", got.Key, want.Key)
	}
	if len(got.Modifiers) != len(want.Modifiers) {
		t.Fatalf("accelerator modifiers length = %d, want %d", len(got.Modifiers), len(want.Modifiers))
	}
	for index := range got.Modifiers {
		if got.Modifiers[index] != want.Modifiers[index] {
			t.Fatalf("accelerator modifier[%d] = %q, want %q", index, got.Modifiers[index], want.Modifiers[index])
		}
	}
}
