package main

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func newTrayEnv(t *testing.T) (*lanShareEnv, *fakeNative, *DesktopApp) {
	t.Helper()
	env := newLANShareEnv(t)
	native := &fakeNative{}
	app := NewDesktopApp(native)
	app.lanShare = env.share
	app.watchLANShare()
	return env, native, app
}

func trayLabels(items []menuItem) []string {
	var labels []string
	for _, item := range items {
		if !item.separator {
			labels = append(labels, item.label)
		}
	}
	return labels
}

func trayItem(t *testing.T, items []menuItem, label string) menuItem {
	t.Helper()
	i := slices.IndexFunc(items, func(item menuItem) bool { return item.label == label })
	if i < 0 {
		t.Fatalf("tray item %q missing from %q", label, trayLabels(items))
	}
	return items[i]
}

func lastLANEvent(t *testing.T, native *fakeNative) LANShareStatus {
	t.Helper()
	native.mu.Lock()
	defer native.mu.Unlock()
	if len(native.events) == 0 {
		t.Fatal("no LAN share event emitted")
	}
	ev := native.events[len(native.events)-1]
	if ev.name != lanShareChangedEvent {
		t.Fatalf("event name = %q, want %q", ev.name, lanShareChangedEvent)
	}
	return ev.data.(LANShareStatus)
}

func TestLANShareTrayHiddenWhileOff(t *testing.T) {
	_, native, _ := newTrayEnv(t)
	if items := native.trayItems(); len(items) != 0 {
		t.Fatalf("tray shown while sharing is off: %q", trayLabels(items))
	}
}

func TestLANShareTrayFollowsSettings(t *testing.T) {
	env, native, _ := newTrayEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}

	items := native.trayItems()
	labels := trayLabels(items)
	for _, want := range []string{"Shelf a", st.URL, "Copy URL", "Stop Sharing", "Open PlainShelf"} {
		if !slices.Contains(labels, want) {
			t.Errorf("tray %q lacks %q", labels, want)
		}
	}
	if slices.Contains(labels, "Shelf b") {
		t.Errorf("tray lists an unshared shelf: %q", labels)
	}
	for _, l := range labels {
		if strings.Contains(l, st.Token) {
			t.Errorf("tray label %q shows the access token", l)
		}
	}
	if ev := lastLANEvent(t, native); !ev.Running || ev.URL != st.URL {
		t.Errorf("event = %+v, want the running status", ev)
	}

	rotated, err := env.share.rotateToken()
	if err != nil {
		t.Fatal(err)
	}
	if ev := lastLANEvent(t, native); ev.Token != rotated.Token {
		t.Errorf("event after rotation carries token %q, want %q", ev.Token, rotated.Token)
	}

	if _, err := env.share.apply(LANShareParams{Enabled: false, ShelfIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if items := native.trayItems(); len(items) != 0 {
		t.Errorf("tray still shown after stopping in settings: %q", trayLabels(items))
	}
}

func TestLANShareTrayCopiesSettingsURL(t *testing.T) {
	env, native, _ := newTrayEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	trayItem(t, native.trayItems(), "Copy URL").action()
	if len(native.copied) != 1 || native.copied[0] != st.URL {
		t.Fatalf("copied %q, want [%q]", native.copied, st.URL)
	}
}

func TestLANShareTrayStopClosesExport(t *testing.T) {
	env, native, _ := newTrayEnv(t)
	st, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	if code := lanGet(t, st, "a/", st.Token); code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND before stopping = %d, want 207", code)
	}

	trayItem(t, native.trayItems(), "Stop Sharing").action()

	if lanDialable(strings.TrimSuffix(strings.TrimPrefix(st.URL, "http://"), "/dav/")) {
		t.Error("export still accepts connections after Stop Sharing")
	}
	if items := native.trayItems(); len(items) != 0 {
		t.Errorf("tray still shown after Stop Sharing: %q", trayLabels(items))
	}
	if ev := lastLANEvent(t, native); ev.Enabled || ev.Running {
		t.Errorf("settings event = %+v, want stopped", ev)
	}
	saved, err := loadLANShareConfig(filepath.Join(env.dataRoot, lanShareConfigName))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Enabled || !slices.Equal(saved.ShelfIDs, []string{"a"}) {
		t.Errorf("saved config = %+v, want disabled keeping shelf a", saved)
	}
}

func TestLANShareTrayOpensWindow(t *testing.T) {
	env, native, _ := newTrayEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	trayItem(t, native.trayItems(), "Open PlainShelf").action()
	if native.shown != 1 {
		t.Fatalf("showWindow called %d times, want 1", native.shown)
	}
}

func TestLANShareTrayDropsRemovedShelf(t *testing.T) {
	env, native, _ := newTrayEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: true, ShelfIDs: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := env.share.forgetShelf("a"); err != nil {
		t.Fatal(err)
	}
	labels := trayLabels(native.trayItems())
	if slices.Contains(labels, "Shelf a") || !slices.Contains(labels, "Shelf b") {
		t.Errorf("tray after removing shelf a = %q", labels)
	}
}

func TestLANShareTrayTranslates(t *testing.T) {
	env, native, _ := newTrayEnv(t)
	if _, err := env.share.apply(LANShareParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	items := native.trayItems()
	trayItem(t, items, "No shelves shared")
	if err := addWailsMenuItems(application.NewMenu(), items); err != nil {
		t.Fatal(err)
	}
}
