package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPersonSub(t *testing.T) {
	now := time.Date(2026, 11, 21, 15, 0, 0, 0, time.Local)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	for _, tc := range []struct {
		name  string
		ps    Person
		mode  int
		want  string
		faint bool
	}{
		{"status words", Person{Presence: PresenceOnline, Status: Status{Emoji: "🎧", Text: "Focusing"}}, PeopleSubStatus, "Focusing", false},
		{"emoji only falls back to how they are", Person{Presence: PresenceOnline, Status: Status{Emoji: "🎧"}}, PeopleSubStatus, "Online", false},
		{"no status, away", Person{Presence: PresenceAway}, PeopleSubStatus, "Away", false},
		{"no status, offline", Person{Presence: PresenceNone, LastSeen: ago(3 * time.Hour)}, PeopleSubStatus, "Last seen 3h ago", true},
		{"a status that ended", Person{Presence: PresenceNone, Status: Status{Text: "gone", Until: ago(time.Hour)}}, PeopleSubStatus, "Offline", true},
		{"last chat today", Person{LastChat: now.Add(-2 * time.Hour)}, PeopleSubLastChat, "Chatted today, 13:00", false},
		{"last chat yesterday", Person{LastChat: ago(30 * time.Hour)}, PeopleSubLastChat, "Chatted yesterday", false},
		{"last chat this week", Person{LastChat: ago(4 * 24 * time.Hour)}, PeopleSubLastChat, "Chatted Tuesday", false},
		{"last chat long ago", Person{LastChat: time.Date(2026, 3, 2, 10, 0, 0, 0, time.Local)}, PeopleSubLastChat, "Chatted 2 Mar", false},
		{"never chatted", Person{}, PeopleSubLastChat, "No chat yet", true},
	} {
		got, faint := personSub(tc.ps, tc.mode, now)
		if got != tc.want || faint != tc.faint {
			t.Errorf("%s: personSub = %q (faint %v), want %q (faint %v)", tc.name, got, faint, tc.want, tc.faint)
		}
	}
}

// Someone whose status has no emoji still gets a bubble: with a chat glyph,
// and the words as its label — and none when they have no status.
func TestStatusWithoutEmojiStillHasABubble(t *testing.T) {
	people := []Person{
		{ID: "@a:x", Name: "Ada", Presence: PresenceOnline, Status: Status{Text: "Out for lunch"}},
		{ID: "@b:x", Name: "Bo", Presence: PresenceOnline, Status: Status{Emoji: "🎧", Text: "Focusing"}},
		{ID: "@c:x", Name: "Cy", Presence: PresenceOnline},
	}
	st := &PeopleState{}
	colors := &UserColors{}
	view := func(c *ui.Context) {
		Apply(c)
		ui.Column(c).Padding(24).Children(func() { PeopleGrid(c, st, people, colors, time.Now()) })
	}
	tt := ui.NewTester(view, 900, 500)
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("Out for lunch"); !ok {
		t.Fatal("no bubble (or line) for a status without an emoji")
	}
	if _, ok := tt.Find("🎧 Focusing"); !ok {
		t.Fatal("no bubble for a status with an emoji")
	}
	count := 0
	for _, s := range tt.Texts() {
		if s == "Out for lunch" || s == "🎧 Focusing" {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("bubbles = %d", count)
	}
}
