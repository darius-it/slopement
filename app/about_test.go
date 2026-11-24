package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/internal/licenses"
)

// Settings → About lists the credits and the open source software the app is
// built from, each license opening to its full text; the placeholder footer is
// gone from Advanced.
func TestAboutListsTheOpenSourceItUses(t *testing.T) {
	a := testApp()
	tt := ui.NewTester(a.View, 1100, 2600) // tall: the list is long
	a.view, a.settings = viewSettings, "Advanced"
	settle(tt)
	if tt.HasText("mock data") || tt.HasText("slopement 0.1") {
		t.Fatal("the placeholder footer is still in Advanced")
	}

	a.settings = "About"
	settle(tt)
	for _, want := range []string{"Open source software", "Also used", "Element Call", "Lucide", "maunium.net/go/mautrix"} {
		if !tt.HasText(want) {
			t.Fatalf("About has no %q", want)
		}
	}
	snap(t, "settings-about", tt)

	n := licenses.All()
	if len(n) < 30 {
		t.Fatalf("only %d modules listed", len(n))
	}
	for _, x := range n {
		if x.License == "Unknown" || strings.TrimSpace(x.Text) == "" || x.Version == "" {
			t.Errorf("%s: license %q, text %d bytes, version %q", x.Name, x.License, len(x.Text), x.Version)
		}
	}
	// A license opens to its text.
	if tt.HasText("Mozilla Public License Version 2.0") {
		t.Fatal("a license text shows before it is opened")
	}
	if err := tt.Click("maunium.net/go/mautrix, MPL-2.0"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !a.noticeOpen["maunium.net/go/mautrix"] {
		t.Fatal("the press did not open the license")
	}
}

// What the license check says is as it was thought: nothing in the build is
// stronger than the weak (file-level) copyleft that was looked at, the two MPL
// modules below.
func TestOnlyKnownCopyleftIsInTheBuild(t *testing.T) {
	known := map[string]string{
		"maunium.net/go/mautrix": "MPL-2.0",
		"go.mau.fi/util":         "MPL-2.0",
	}
	for _, n := range licenses.All() {
		if licenses.Copyleft(n.License) && known[n.Name] != n.License {
			t.Errorf("%s is %s: look at what it asks of this app, then add it here", n.Name, n.License)
		}
	}
}
