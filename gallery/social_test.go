package gallery

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
)

func TestVote(t *testing.T) {
	one := kit.PollData{Answers: []kit.PollAnswer{{Votes: 1}, {Votes: 2}}}
	vote(&one, 0)
	vote(&one, 1) // one choice: moves the vote
	if one.Answers[0].Mine || one.Answers[0].Votes != 1 || !one.Answers[1].Mine || one.Answers[1].Votes != 3 {
		t.Fatalf("single = %+v", one.Answers)
	}
	vote(&one, 1) // again: takes it back
	if one.Answers[1].Mine || one.Answers[1].Votes != 2 {
		t.Fatalf("unvote = %+v", one.Answers)
	}
	many := kit.PollData{Multiple: true, Answers: []kit.PollAnswer{{}, {}}}
	vote(&many, 0)
	vote(&many, 1)
	if !many.Answers[0].Mine || !many.Answers[1].Mine {
		t.Fatalf("multiple = %+v", many.Answers)
	}
	ended := kit.PollData{Ended: true, Answers: []kit.PollAnswer{{}}}
	vote(&ended, 0)
	if ended.Answers[0].Mine {
		t.Fatal("voted on an ended poll")
	}
}

func TestClearTime(t *testing.T) {
	now := time.Date(2026, 11, 19, 14, 0, 0, 0, time.UTC) // a Thursday
	for i, want := range []time.Time{
		now.Add(30 * time.Minute),
		now.Add(time.Hour),
		now.Add(4 * time.Hour),
		time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), // today: midnight
		time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), // this week: Monday
		{},
	} {
		if got := kit.ClearTime(i, now); !got.Equal(want) {
			t.Errorf("ClearTime(%s) = %v, want %v", kit.ClearAfter[i], got, want)
		}
	}
	s := kit.Status{Emoji: "🍔", Text: "Lunch", Until: now.Add(time.Minute)}
	if !s.Set(now) || s.Set(now.Add(2*time.Minute)) {
		t.Error("status expiry")
	}
}

// TestPeopleOrder: online before offline, statuses first in each, and
// the Online filter hides the offline.
func TestPeopleOrder(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Status & profile", 1
	tt := ui.NewTester(a.View, 1280, 1400)
	tt.Frame()
	alex, _ := tt.Find("Alex Rivera")
	jun, _ := tt.Find("Jun")
	kai, _ := tt.Find("Kai")
	ola, _ := tt.Find("Ola")
	// Reading order: top to bottom, then left to right (the grid).
	before := func(a, b ui.Rect) bool { return a.Y < b.Y-1 || (a.Y < b.Y+1 && a.X < b.X) }
	if !(before(alex, jun) && before(jun, kai) && before(kai, ola)) {
		t.Fatalf("order: alex %v jun %v kai %v ola %v", alex, jun, kai, ola)
	}
	if !tt.HasText("Online — 4") || !tt.HasText("Offline — 3") {
		t.Fatal("section counts")
	}
	a.people.Filter = 1
	tt.Frame()
	if tt.HasText("Offline — 3") {
		t.Fatal("Online filter shows the offline")
	}
}

// TestPollVoters clicks an ended poll's "21 votes": who voted for what
// opens, you first, with "and N more" for voters not loaded. A running
// hidden poll's count isn't a link (it would give the results away).
func TestPollVoters(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Polls", 1
	tt := ui.NewTester(a.View, 1440, 1400)
	tt.Frame()
	if _, ok := tt.Find("See who voted (11 votes)"); ok {
		t.Fatal("a running hidden poll shows its voters")
	}
	r, ok := tt.Find("See who voted (21 votes)")
	if !ok {
		t.Fatal("no votes link on the ended poll")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "poll-votes-hover.png"), tt)
	}
	if err := tt.Click("See who voted (21 votes)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	for _, want := range []string{"Tabs or spaces?", "You", "Theo", "and 7 more"} {
		if !tt.HasText(want) {
			t.Fatalf("who voted lacks %q", want)
		}
	}
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "poll-voters.png"), tt)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if tt.HasText("and 7 more") {
		t.Fatal("Escape did not close who voted")
	}
}

// TestSetStatus types a status in the editor and saves it.
func TestSetStatus(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Status & profile", 1
	tt := ui.NewTester(a.View, 1280, 900)
	tt.Frame()
	if err := tt.Click("Set a status"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("Lunch") // the text field has the focus
	tt.Frame()
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	// The default emoji, cleared at midnight ("Today").
	if a.myStatus.Text != "Lunch" || a.myStatus.Emoji != "💬" || a.myStatus.Until.IsZero() {
		t.Fatalf("status = %+v", a.myStatus)
	}
	if a.statusDraft.Open {
		t.Fatal("editor still open")
	}
}
