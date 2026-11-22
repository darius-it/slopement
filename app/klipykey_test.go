package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/klipy"
	"slopement/model/mock"
	"slopement/picker"
)

// Settings → GIFs takes the user's own KLIPY key: Save keeps it, the saved key
// is not shown again, Remove takes it away.
func TestKlipyKeyInSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KLIPY_API_KEY", "")
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(klipy.New(klipy.Config{})))
	a.view, a.settings = viewSettings, "GIFs"
	tt := ui.NewTester(a.View, 1100, 1100)
	settle(tt)
	for _, want := range []string{"KLIPY key", "Your KLIPY key", "No key: search is off"} {
		if !tt.HasText(want) {
			t.Fatalf("%q is not on the page", want)
		}
	}
	if a.pick.HasKlipyKey() {
		t.Fatal("a key before one was given")
	}

	a.klipyDraft = " my-own-key "
	settle(tt)
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.pick.KlipyKey() != "my-own-key" || !a.pick.HasKlipyKey() {
		t.Fatalf("saved key = %q", a.pick.KlipyKey())
	}
	if a.klipyDraft != "" {
		t.Fatal("the typed key stays in the field")
	}
	if !tt.HasText("Saved. GIF and sticker search is on.") || tt.HasText("my-own-key") {
		t.Fatal("the page does not say it is saved, or shows the key")
	}

	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.pick.HasKlipyKey() {
		t.Fatal("removed, and still there")
	}
}

// With a key in the environment, the page says so and has no field.
func TestKlipyKeyFromEnvironmentInSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KLIPY_API_KEY", "from-env")
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(klipy.New(klipy.Config{})))
	a.view, a.settings = viewSettings, "GIFs"
	tt := ui.NewTester(a.View, 1100, 1100)
	settle(tt)
	if !tt.HasText("KLIPY_API_KEY") || tt.HasText("Your KLIPY key") {
		t.Fatal("the environment's key is not said, or the field shows")
	}
}
