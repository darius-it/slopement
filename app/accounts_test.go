package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

// The picture's menu has one submenu for the accounts: a row for each (its
// Log out in the row), and Add account as the last row.
func TestAccountMenu(t *testing.T) {
	a := testApp()
	room, _ := longTimeline(a, 3)
	a.open(room.ID)
	me := a.src.Me().ID
	var switched, added, removed string
	a.SetAccounts(&Accounts{
		List: func() []AccountEntry {
			return []AccountEntry{{User: me, Server: "matrix.org", Current: true}, {User: "@kai:example.org", Server: "example.org"}}
		},
		Switch: func(u string) { switched = u },
		Add:    func() { added = "yes" },
		Remove: func(u string) { removed = u },
	})
	tt := ui.NewTester(a.View, 1100, 700)
	settle(tt)
	if err := tt.Click("Account menu"); err != nil {
		t.Fatalf("the picture opens no menu: %v", err)
	}
	settle(tt)
	menu := tt.Menu()
	for _, want := range []string{"Set status…", "View profile", "Switch account"} {
		found := false
		for _, l := range menu {
			found = found || l == want
		}
		if !found {
			t.Fatalf("menu %q lacks %q", menu, want)
		}
	}
	for _, gone := range []string{"Add account…", "Remove account"} {
		for _, l := range menu {
			if l == gone {
				t.Fatalf("the menu still has %q at its top level", gone)
			}
		}
	}
	if err := tt.Click("Switch account"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !tt.HasText("@kai:example.org") || !tt.HasText("Add account") {
		t.Fatalf("the submenu: %q", tt.Menu())
	}
	// A row the pointer is on is highlighted in the hover gray, not the
	// accent fill, which drowned its picture and its Log out button.
	r, ok := tt.Find("@kai:example.org")
	if !ok {
		t.Fatal("no row for the other account")
	}
	tt.Move(r.X+r.W+4, r.Y+r.H/2)
	settle(tt)
	snap(t, "account-menu-hover", tt)
	accent := kit.Light.Accent // the tester draws the light theme
	px := tt.Image().RGBAAt(int(r.X-2), int(r.Y+r.H/2))
	near := func(a, b uint8) bool { return a-b < 24 || b-a < 24 }
	if near(px.R, accent.R) && near(px.G, accent.G) && near(px.B, accent.B) {
		t.Fatalf("the hovered account row is filled with the accent: %v", px)
	}
	if err := tt.Click("Log out of @kai:example.org"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.removing != "@kai:example.org" || !a.removeAccount.Open {
		t.Fatalf("Log out did not ask: removing %q", a.removing)
	}
	_, _, _ = switched, added, removed
}
