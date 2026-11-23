package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model/mock"
	"slopement/picker"
)

func TestUsernameOf(t *testing.T) {
	for in, want := range map[string]string{
		"@ada:example.org":         "ada",
		"@_discord_123:matrix.org": "_discord_123",
		"@kai:kde.org":             "kai",
		"plain":                    "plain",
		"":                         "",
	} {
		if got := usernameOf(in); got != want {
			t.Errorf("usernameOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// Display names are the default, as in other clients; the setting shows
// usernames in the chat list, the header, the messages and People — and goes
// back.
func TestShowUsernamesInsteadOfDisplayNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.nav = kit.NavRail
	a.open("!dm-alex")
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)

	if a.prefs.usernames {
		t.Fatal("usernames are shown by default: display names should be")
	}
	if !tt.HasText("Alex Rivera") {
		t.Fatal("the display name is not shown by default")
	}
	if tt.HasText("alex") && !tt.HasText("Alex Rivera") {
		t.Fatal("a username shows with names off")
	}

	a.prefs.usernames = true
	settle(tt)
	if !tt.HasText("alex") {
		t.Fatal("the username is not shown")
	}
	if tt.HasText("Alex Rivera") {
		t.Fatal("the display name still shows with usernames on")
	}
	// The messages' senders too.
	found := false
	for _, acc := range tt.AccessAll() {
		if acc.Name == "alex" {
			found = true
		}
	}
	if !found {
		t.Fatal("no element named by the username")
	}

	// It is kept for the next run.
	b := New(mock.New(time.Now()), picker.New(nil))
	b.applyState(a.state())
	if !b.prefs.usernames {
		t.Fatal("the choice was not kept with the state")
	}

	a.prefs.usernames = false
	settle(tt)
	if !tt.HasText("Alex Rivera") {
		t.Fatal("turning it off did not bring the display names back")
	}
}

// The switch is in Settings → Appearance.
func TestUsernamesSwitchInAppearance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(m, picker.New(nil))
	a.view, a.settings = viewSettings, "Appearance"
	tt := ui.NewTesterWith(a.View, 1100, 1500, ui.TesterOptions{Dark: true})
	settle(tt)
	if !tt.HasText("Show usernames") {
		t.Fatal("no switch for it in Settings → Appearance")
	}
	clickSwitch(t, tt, "Show usernames")
	if !a.prefs.usernames {
		t.Fatal("the switch did not turn it on")
	}
}
