package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// The Appearance choices apply (the tint reaches the theme) and persist.
func TestAppearanceSettings(t *testing.T) {
	defer kit.SetAccent(0)
	defer func() { kit.Dense = false }()
	a := testApp()
	a.settings = "Appearance"
	a.view = viewSettings
	tt := ui.NewTester(a.View, 1100, 1400)
	settle(tt)
	if err := tt.Click("Rose accent"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.prefs.accent != 4 || kit.Accent() != 4 {
		t.Fatalf("accent pref %d, kit %d", a.prefs.accent, kit.Accent())
	}
	if !tt.HasText("Compact timeline") {
		t.Fatal("no compact timeline row")
	}
	a.prefs.dense = true // the switch itself is mygo's
	settle(tt)
	if !a.prefs.dense || !kit.Dense {
		t.Fatal("compact timeline not applied")
	}
	// Round trip through the saved state.
	st := a.prefs.state()
	if got := st.prefs(); got.accent != 4 || !got.dense || got.uiSize != a.prefs.uiSize {
		t.Fatalf("state round trip: %+v", got)
	}
}
