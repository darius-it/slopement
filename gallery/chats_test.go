package gallery

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestNameOpensProfile clicks a sender's name in the timeline: their card
// opens, with their status; Escape closes it.
func TestNameOpensProfile(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Desktop", 1
	tt := ui.NewTester(a.View, 1440, 1000)
	tt.Frame()
	if err := tt.Click("Show Mira's profile"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.profileMsg != "e1" || !tt.HasText("@mira:matrix.org") {
		t.Fatalf("profile not open: %q", a.profileMsg)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.profileMsg != "" {
		t.Fatal("Escape did not close the card")
	}
}

// TestThoughtBubbleOpens hovers a person in the People grid: their
// bubble opens to the whole status. With SNAP_DIR it writes the frame.
func TestThoughtBubbleOpens(t *testing.T) {
	a := testApp()
	a.page, a.themeMode, a.room = "Desktop", 1, "people"
	tt := ui.NewTester(a.View, 1440, 1000)
	tt.Frame()
	// Kai: a status, and no chat, so the chat list's bubbles (whose text
	// is always laid out, clipped) don't answer for the grid's.
	if _, open := tt.Find("Travelling this month"); open {
		t.Fatal("the bubble is open at rest")
	}
	kai, ok := tt.Find("Kai")
	if !ok {
		t.Fatal("no Kai tile")
	}
	tt.Move(kai.X+kai.W/2, kai.Y+kai.H/2) // over Kai's tile
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if _, open := tt.Find("Travelling this month"); !open {
		t.Fatal("the bubble did not open on hover")
	}
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "thought-hover.png"), tt)
	}
}

// TestChatBubbleHover: in the chat list, the status emoji on an avatar
// opens to the whole status when hovered, without when it clears, and
// the preview line stays the last message.
func TestChatBubbleHover(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Desktop", 1
	tt := ui.NewTester(a.View, 1440, 1000)
	tt.Frame()
	// The bubble is found by its label, the whole status.
	r, ok := tt.Find("🎧 Focusing on the kit")
	if !ok || r.W > 30 {
		t.Fatalf("no closed status bubble on Alex's avatar: %v", r)
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "chat-bubble-hover.png"), tt)
	}
	if open, _ := tt.Find("🎧 Focusing on the kit"); open.W < 100 {
		t.Fatalf("the bubble did not open on hover: %v", open)
	}
	if !tt.HasText("sounds good, tomorrow 10?") {
		t.Fatal("the preview line was replaced")
	}
	if tt.HasText("until") {
		t.Fatal("someone else's status shows when it clears")
	}
	// Frame by frame, through a close and an open: the emoji never moves,
	// and the cloud's width only ever goes one way (no back and forth).
	emoji, _ := tt.Find("🎧")
	steps := func(name string, grow bool) {
		last := float32(-1)
		for i := 0; i < 20; i++ {
			tt.Advance(16 * time.Millisecond)
			tt.Frame()
			e, _ := tt.Find("🎧")
			if d := e.X - emoji.X; d > 0.5 || d < -0.5 || e.Y != emoji.Y {
				t.Fatalf("%s, frame %d: the emoji moved from %v to %v", name, i, emoji, e)
			}
			w, _ := tt.Find("🎧 Focusing on the kit")
			if last >= 0 && ((grow && w.W < last-0.01) || (!grow && w.W > last+0.01)) {
				t.Fatalf("%s, frame %d: width went back, %v after %v", name, i, w.W, last)
			}
			last = w.W
		}
	}
	tt.Move(5, 5) // away: it closes
	steps("closing", false)
	if w, _ := tt.Find("🎧 Focusing on the kit"); w.W > 21 {
		t.Fatalf("not closed: %v", w)
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2) // back over it: it opens
	steps("opening", true)
	// The bubble's box doesn't block the row under it.
	a.page = "B · Chats on the rail"
	tt.Frame()
	if err := tt.Click("Mira"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.concepts().chatOpen != "c-mira" {
		t.Fatalf("row click blocked: open %q", a.concepts().chatOpen)
	}
}

// TestComposerPlusMenu opens the composer's +: both items, with their
// icons; choosing Create poll opens the poll editor.
func TestComposerPlusMenu(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Desktop", 1
	tt := ui.NewTester(a.View, 1440, 1000)
	tt.Frame()
	if err := tt.Click("Attach or create"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Upload a file") || !tt.HasText("Create poll") {
		t.Fatal("the + menu's items are missing")
	}
	if dir := os.Getenv("SNAP_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "composer-plus.png"), tt)
	}
	if err := tt.ChooseMenuItem("Create poll"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.pollSheet.Open {
		t.Fatal("Create poll did not open the editor")
	}
}

// TestFavouritesSection: starred chats lead under Favourites, and the
// chat menu stars another.
func TestFavouritesSection(t *testing.T) {
	a := testApp()
	a.page, a.themeMode = "Desktop", 1
	tt := ui.NewTester(a.View, 1440, 1000)
	tt.Frame()
	fav, ok1 := tt.Find("Favourites")
	alex, ok2 := tt.Find("Alex Rivera")
	mira, ok3 := tt.Find("Mira")
	if !ok1 || !ok2 || !ok3 || !(fav.Y < alex.Y && alex.Y < mira.Y) {
		t.Fatalf("favourites %v alex %v mira %v", fav, alex, mira)
	}
	a.favChats["c-sam"] = true
	tt.Frame()
	sam, _ := tt.Find("Sam")
	if chats, _ := tt.Find("Chats"); sam.Y > chats.Y && chats.Y > fav.Y {
		// Sam is now above the Chats section header.
		t.Fatalf("Sam not among favourites: sam %v chats %v", sam, chats)
	}
}
