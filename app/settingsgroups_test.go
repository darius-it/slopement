package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The small switches of Appearance share one group, without notes between
// them; the three of them are still there.
func TestAppearanceSwitchesShareAGroup(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 1700)
	a.view, a.settings = viewSettings, "Appearance"
	settle(tt)
	for _, want := range []string{"Chats and people", "Show usernames", "Status on my picture", "Jump to last chat"} {
		if !tt.HasText(want) {
			t.Fatalf("Appearance has no %q", want)
		}
	}
	for _, gone := range []string{"Switching", "Your status", "Names"} {
		if tt.HasText(gone) {
			t.Fatalf("Appearance still has the group %q", gone)
		}
	}
	snap(t, "settings-appearance-switches", tt)
}
