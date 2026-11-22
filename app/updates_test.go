package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

// fakeUpdater counts the questions put to "GitHub" and offers a version.
type fakeUpdater struct {
	enabled   bool
	checks    atomic.Int32
	installed atomic.Int32
	offer     *updateOffer
	err       error
}

func (f *fakeUpdater) Enabled() bool { return f.enabled }
func (f *fakeUpdater) Check(ctx context.Context) (*updateOffer, error) {
	f.checks.Add(1)
	if f.offer != nil {
		o := *f.offer
		o.Install = func(ctx context.Context, progress func(done, total int64)) error {
			f.installed.Add(1)
			progress(50, 100)
			return nil
		}
		return &o, f.err
	}
	return nil, f.err
}

// updateApp is an app with the updater faked, its choices in a scratch file.
func updateApp(t *testing.T, f *fakeUpdater) (*App, *ui.Tester, *atomic.Int32) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldU, oldR := theUpdater, relaunchApp
	oldPath := upd.path
	var relaunched atomic.Int32
	theUpdater, relaunchApp = f, func() { relaunched.Add(1) }
	upd.path = filepath.Join(t.TempDir(), "updates.json")
	upd.loaded, upd.prefs, upd.started = false, updatePrefs{}, false
	t.Cleanup(func() {
		theUpdater, relaunchApp, upd.path = oldU, oldR, oldPath
		upd.loaded, upd.prefs, upd.started = false, updatePrefs{}, false
	})
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.runUI = func(fn func()) { fn() }
	tt := ui.NewTesterWith(a.View, 1200, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	return a, tt, &relaunched
}

func waitUntil(t *testing.T, tt *ui.Tester, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		settle(tt)
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Updates are opt-in: by default nothing asks GitHub, however long the app runs.
func TestUpdatesAreOffByDefault(t *testing.T) {
	f := &fakeUpdater{enabled: true, offer: &updateOffer{Version: "9.9.9"}}
	a, tt, _ := updateApp(t, f)
	if getUpdatePrefs().Check {
		t.Fatal("checking for updates is on by default")
	}
	for i := 0; i < 5; i++ {
		settle(tt)
	}
	time.Sleep(50 * time.Millisecond)
	if f.checks.Load() != 0 || a.upd.Sheet.Open {
		t.Fatalf("asked %d times, dialog open %v: nothing should happen unless it was turned on", f.checks.Load(), a.upd.Sheet.Open)
	}
}

// The switch is in Settings → Privacy, and what it sets is kept for the next run.
func TestUpdateSwitchInSettings(t *testing.T) {
	f := &fakeUpdater{enabled: true}
	a, tt, _ := updateApp(t, f)
	a.view, a.settings = viewSettings, "Privacy"
	settle(tt)
	if !tt.HasText("Check for updates at startup") {
		t.Fatal("no switch for it in Settings → Privacy")
	}
	clickSwitch(t, tt, "Check for updates at startup")
	if !getUpdatePrefs().Check {
		t.Fatal("the switch did not turn it on")
	}
	b, err := os.ReadFile(upd.path)
	if err != nil || !contains(string(b), `"check": true`) {
		t.Fatalf("not kept for the next run: %q %v", b, err)
	}
	clickSwitch(t, tt, "Check for updates at startup")
	if getUpdatePrefs().Check {
		t.Fatal("the switch did not turn it off again")
	}
}

// clickSwitch presses the switch of a row (the row's text carries the same
// name, so a click by name would land on that).
func clickSwitch(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	for _, acc := range tt.AccessAll() {
		if acc.Name == name && acc.Focusable {
			tt.ClickAtWith(0, acc.Bounds.X+acc.Bounds.W/2, acc.Bounds.Y+acc.Bounds.H/2)
			settle(tt)
			return
		}
	}
	t.Fatalf("no switch named %q", name)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Turned on: one question at startup (not one per frame, not one per account),
// an offer in a dialog, and nothing installed until Install is pressed.
func TestUpdateAtStartOffersAndInstallsOnlyOnConfirm(t *testing.T) {
	f := &fakeUpdater{enabled: true, offer: &updateOffer{Version: "9.9.9", Notes: "Fixes things."}}
	a, tt, relaunched := updateApp(t, f)
	setUpdatePrefs(updatePrefs{Check: true})
	waitUntil(t, tt, "the offer", func() bool { return a.upd.Sheet.Open })
	for i := 0; i < 4; i++ {
		settle(tt)
	}
	if f.checks.Load() != 1 {
		t.Fatalf("asked %d times, want once", f.checks.Load())
	}
	for _, want := range []string{"Version 9.9.9 is available", "Install and restart", "Later", "Skip this version"} {
		if !tt.HasText(want) {
			t.Fatalf("%q is not in the dialog", want)
		}
	}
	if f.installed.Load() != 0 {
		t.Fatal("installed before the user confirmed")
	}

	// Later: closed, nothing installed.
	if err := tt.Click("Later"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.upd.Sheet.Open || f.installed.Load() != 0 {
		t.Fatal("Later installed something or left the dialog open")
	}

	// A second app (another account) in the same run does not ask again.
	b := New(mock.New(time.Now()), picker.New(nil))
	b.runUI = func(fn func()) { fn() }
	tb := ui.NewTester(b.View, 800, 600)
	settle(tb)
	time.Sleep(30 * time.Millisecond)
	if f.checks.Load() != 1 {
		t.Fatalf("a second app asked again: %d", f.checks.Load())
	}

	// Install and restart.
	a.upd.Sheet.Open = true
	settle(tt)
	if err := tt.Click("Install and restart"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, tt, "the install", func() bool { return f.installed.Load() == 1 })
	if relaunched.Load() != 1 {
		t.Fatalf("relaunched %d times", relaunched.Load())
	}
}

// "Skip this version" is kept: it is not offered at startup again, but a check
// the user asks for still shows it.
func TestSkippedUpdateIsNotOfferedAtStart(t *testing.T) {
	f := &fakeUpdater{enabled: true, offer: &updateOffer{Version: "9.9.9"}}
	a, tt, _ := updateApp(t, f)
	setUpdatePrefs(updatePrefs{Check: true})
	waitUntil(t, tt, "the offer", func() bool { return a.upd.Sheet.Open })
	if err := tt.Click("Skip this version"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.upd.Sheet.Open || getUpdatePrefs().Skip != "9.9.9" {
		t.Fatalf("not skipped: open %v prefs %+v", a.upd.Sheet.Open, getUpdatePrefs())
	}

	// The next run: asked, but silent about the skipped version.
	upd.started = false
	b, tb, _ := updateApp2(t, f)
	for i := 0; i < 5; i++ {
		settle(tb)
		time.Sleep(10 * time.Millisecond)
	}
	if b.upd.Sheet.Open {
		t.Fatal("a skipped version was offered at startup")
	}
	// On request it is shown.
	b.checkUpdates(true)
	waitUntil(t, tb, "the manual offer", func() bool { return b.upd.Phase == updAvailable })
}

// updateApp2 is another app of the same run, sharing the updater and choices.
func updateApp2(t *testing.T, f *fakeUpdater) (*App, *ui.Tester, *atomic.Int32) {
	t.Helper()
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.runUI = func(fn func()) { fn() }
	tt := ui.NewTesterWith(a.View, 1200, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	return a, tt, nil
}

// A check the user asks for says what came of it; a failure is said plainly,
// and a failure at startup is silent.
func TestManualUpdateCheck(t *testing.T) {
	f := &fakeUpdater{enabled: true}
	a, tt, _ := updateApp(t, f)
	a.checkUpdates(true)
	waitUntil(t, tt, "the answer", func() bool { return a.upd.Phase == updUpToDate })
	if !tt.HasText("slopement is up to date.") {
		t.Fatal("up to date is not said")
	}
	a.upd.Sheet.Open = false

	f.err = errors.New("offline")
	a.checkUpdates(true)
	waitUntil(t, tt, "the failure", func() bool { return a.upd.Phase == updFailed })
	if !tt.HasText("Couldn't check for updates") {
		t.Fatal("the failure is not said")
	}
}

// Where the app cannot update itself (a package manager's copy, a development
// build) nothing is asked, and Settings says so.
func TestUpdatesDisabledBuild(t *testing.T) {
	f := &fakeUpdater{enabled: false, offer: &updateOffer{Version: "9.9.9"}}
	a, tt, _ := updateApp(t, f)
	setUpdatePrefs(updatePrefs{Check: true})
	a.view, a.settings = viewSettings, "Advanced"
	for i := 0; i < 5; i++ {
		settle(tt)
	}
	time.Sleep(30 * time.Millisecond)
	if f.checks.Load() != 0 {
		t.Fatal("asked although the build cannot update")
	}
	if tt.HasText("Check for updates now") {
		t.Fatal("a button for what cannot be done")
	}
	if !tt.HasText("package manager") {
		t.Fatal("Settings does not say why")
	}
}
