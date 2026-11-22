package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A setting that is not built yet looks it — faded, with "Soon" — and a
// press answers "Coming soon", where it used to say what layer it waits for.
func TestUnbuiltSettingsSaySoon(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 1500) // tall: the Appearance page is long
	for _, tc := range []struct{ page, row string }{
		{"Security", "Key storage"},
		{"Security", "Export room keys"},
		{"Advanced", "Export logs"},
		{"Advanced", "Reset local cache"},
		{"Privacy", "Fetch previews from this device"},
		{"Appearance", "Hide joins and leaves"},
		{"Calls", "Camera off when joining"},
	} {
		a.view, a.settings, a.toast = viewSettings, tc.page, ""
		settle(tt)
		if !tt.HasText("Soon") {
			t.Fatalf("%s: no Soon tag", tc.page)
		}
		if err := tt.Click(tc.row + ", coming soon"); err != nil {
			t.Fatalf("%s / %s: %v", tc.page, tc.row, err)
		}
		settle(tt)
		if !tt.HasText("Coming soon") {
			t.Fatalf("%s / %s did not answer \"Coming soon\"", tc.page, tc.row)
		}
		if a.prefs.typing != defaultPrefs().typing {
			t.Fatal("a soon switch changed a setting")
		}
	}
	snap(t, "settings-soon", tt)
}
