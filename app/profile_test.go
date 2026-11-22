package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// profSource is a source that can change the user's profile, hold colors
// and manage sessions, recording what the app asked.
type profSource struct {
	model.Source
	names    []string
	removed  bool
	colors   map[string]int
	sessions []model.Session
	refresh  int
	signed   []string
	browser  bool
}

func (s *profSource) SetDisplayName(n string, done func(string)) { s.names = append(s.names, n); done("") }
func (s *profSource) SetAvatar(model.File, func(string))         {}
func (s *profSource) RemoveAvatar(done func(string))             { s.removed = true; done("") }
func (s *profSource) UserColors() map[string]int                 { return s.colors }
func (s *profSource) SetUserColor(m string, i int) {
	if s.colors == nil {
		s.colors = map[string]int{}
	}
	if i < 0 {
		delete(s.colors, m)
	} else {
		s.colors[m] = i
	}
}
func (s *profSource) Sessions() []model.Session { return s.sessions }
func (s *profSource) RefreshSessions()          { s.refresh++ }
func (s *profSource) SignOutSession(id string, done func(string, bool)) {
	s.signed = append(s.signed, id)
	done("", s.browser)
}
func (s *profSource) AccountPage() string { return "https://account.example.org" }

func profApp(t *testing.T) (*App, *profSource) {
	t.Helper()
	a := testApp()
	ps := &profSource{Source: a.src}
	a.src = ps
	return a, ps
}

// Settings → Account on an account that can change its profile: the name
// is editable and saved, and the name's color is kept with the account.
func TestAccountPageEditsProfile(t *testing.T) {
	a, ps := profApp(t)
	a.view, a.settings = viewSettings, "Account"
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	if !tt.HasText("Display name") || !tt.HasText("Save name") {
		t.Fatal("no way to change the name")
	}
	a.nameDraft = "Robin R."
	if err := tt.Click("Save name"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(ps.names) != 1 || ps.names[0] != "Robin R." {
		t.Fatalf("names saved: %v", ps.names)
	}

	// A color for the user's own name goes to the account, and the view's
	// resolver follows it.
	if err := tt.Click("Teal"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	me := a.src.Me().ID
	if ps.colors[me] != 4 {
		t.Fatalf("colors kept: %v", ps.colors)
	}
	if a.colors.UserIndex(me) != 4 {
		t.Fatalf("the view does not use the account's color: %d", a.colors.UserIndex(me))
	}
}

// Anyone can be recolored from their card; "Auto" goes back.
func TestRecolorFromTheProfileCard(t *testing.T) {
	a, ps := profApp(t)
	a.open(a.src.Rooms()[0].ID)
	var other model.Person
	for _, p := range a.src.People() {
		if p.ID != a.src.Me().ID {
			other = p
			break
		}
	}
	a.profileOpen, a.profileSheet.Open = other.ID, true
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	if !tt.HasText("Name colour") {
		t.Fatal("no color row on the card")
	}
	if err := tt.Click("Purple"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if ps.colors[other.ID] != 6 || a.colors.UserIndex(other.ID) != 6 {
		t.Fatalf("not recolored: %v", ps.colors)
	}
	if err := tt.Click("Automatic color"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if _, ok := ps.colors[other.ID]; ok {
		t.Fatal("Auto did not clear the color")
	}
}

// Settings → Security asks for the devices once per visit, lists them with
// what is known, and signs another out; a homeserver that wants it
// confirmed in a browser is sent to.
func TestSecuritySessions(t *testing.T) {
	a, ps := profApp(t)
	ps.sessions = []model.Session{
		{ID: "OWN", Name: "slopement on this Mac", Current: true, Verified: true, IP: "1.1.1.1"},
		{ID: "PHONE", Name: "Phone", LastSeen: time.Now().Add(-time.Hour), Verified: true, IP: "2.2.2.2"},
	}
	a.view, a.settings = viewSettings, "Security"
	tt := ui.NewTester(a.View, 1100, 800)
	settle(tt)
	settle(tt)
	if ps.refresh != 1 {
		t.Fatalf("devices asked for %d times, want once", ps.refresh)
	}
	if !tt.HasText("slopement on this Mac") || !tt.HasText("Phone") {
		t.Fatal("the devices are not listed")
	}
	if err := tt.Click("Sign out"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(ps.signed) != 1 || ps.signed[0] != "PHONE" {
		t.Fatalf("signed out: %v", ps.signed)
	}

	ps.browser = true
	if err := tt.Click("Sign out"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(ps.signed) != 2 {
		t.Fatalf("the second sign-out was not asked: %v", ps.signed)
	}
}
