package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/kit"
	"slopement/model"
)

// longTimeline is n messages between the user and a partner, the last one
// replying to the third.
func longTimeline(a *App, n int) (model.Room, []model.Event) {
	var room model.Room
	for _, r := range a.src.Rooms() {
		if r.Kind == model.KindDM {
			room = r
			break
		}
	}
	me := a.src.Me().ID
	now := time.Now().Add(-time.Duration(n) * time.Minute)
	tl := make([]model.Event, n)
	for i := range tl {
		sender := room.Partner
		if i%3 == 0 {
			sender = me
		}
		tl[i] = model.Event{ID: fmt.Sprintf("$%d", i), Room: room.ID, Sender: sender, Kind: model.EventText,
			Body: fmt.Sprintf("message number %d", i), Time: now.Add(time.Duration(i) * time.Minute)}
	}
	tl[n-1].ReplyTo = "$2"
	tl[n-1].Body = "the answer to that"
	return room, tl
}

// typingSource says the room has someone typing.
type typingSource struct {
	model.Source
	on bool
}

func (t typingSource) Room(id string) (model.Room, bool) {
	r, ok := t.Source.Room(id)
	if t.on {
		r.Typing = []string{"Alex"}
	}
	return r, ok
}

// "(edited)" ends the text it belongs to — short or wrapping — and is not a
// line of its own (a snapshot with SNAP_DIR).
func TestEditedMarkerIsInline(t *testing.T) {
	a := testApp()
	room, _ := longTimeline(a, 4)
	long := "Und was wäre, wenn man einfach einen anderen Namen drunterschreibt? Würdest du da mitmachen? " +
		"Dann wärich auf einmal ein Herbert Handwerker oder sowas, und niemand würde je etwas merken, weil niemand nachsieht."
	now := time.Now()
	tl := []model.Event{
		{ID: "1", Room: room.ID, Sender: room.Partner, Kind: model.EventText, Body: "ja da hast du recht", Edited: true, Time: now.Add(-2 * time.Minute)},
		{ID: "2", Room: room.ID, Sender: room.Partner, Kind: model.EventText, Body: long, Edited: true, Time: now.Add(-time.Minute)},
	}
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1000, 520)
	settle(tt)
	snap(t, "edited-inline", tt)

	short, ok := tt.Find("ja da hast du recht")
	mark, ok2 := tt.Find("(edited)")
	if !ok || !ok2 {
		t.Fatal("the message or its marker does not show")
	}
	if mark.Y+mark.H < short.Y || mark.Y > short.Y+short.H+4 {
		t.Fatalf("the marker is not on the line of its message: text %+v marker %+v", short, mark)
	}
}

// readSource counts the read markers it is given, and reports a room's
// unread count as it is set.
type readSource struct {
	timeline
	unread int
	marks  int
}

func (s *readSource) Room(id string) (model.Room, bool) {
	r, ok := s.timeline.Source.Room(id)
	r.Unread = s.unread
	return r, ok
}

func (s *readSource) MarkRead(string) { s.marks++; s.unread = 0 }

// A message that arrives in the room being read is marked read, so its
// count does not climb — but not while the user is scrolled back in the
// history, where they have not seen it.
func TestOpenRoomIsReadAsMessagesArrive(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 80)
	rs := &readSource{timeline: timeline{a.src, tl}}
	a.src = rs
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)
	opened := rs.marks // opening it marked it read

	rs.unread = 3 // messages arrived (the server's count)
	rs.timeline.tl = append(rs.timeline.tl, model.Event{ID: "$new", Room: room.ID, Sender: room.Partner, Kind: model.EventText,
		Body: "a new one", Time: time.Now()})
	settle(tt)
	if rs.marks != opened+1 || rs.unread != 0 {
		t.Fatalf("a message arriving while the room is read was not marked: marks %d -> %d, unread %d", opened, rs.marks, rs.unread)
	}

	// Scrolled back in the history: more arrive, and they are not read.
	a.listState(room.ID).ScrollTo(0, ui.Start)
	settle(tt)
	settle(tt)
	rs.unread = 2
	rs.timeline.tl = append(rs.timeline.tl, model.Event{ID: "$newer", Room: room.ID, Sender: room.Partner, Kind: model.EventText,
		Body: "another", Time: time.Now()})
	settle(tt)
	if rs.unread != 2 {
		t.Fatal("marked read while the user is scrolled back in the history")
	}
	// Back at the end: now they are.
	a.listState(room.ID).ScrollToEnd()
	settle(tt)
	settle(tt)
	if rs.unread != 0 {
		t.Fatal("not marked read when the user came back to the end")
	}

	// Not the room on screen (Settings is): nothing is marked.
	a.view = viewSettings
	rs.unread = 4
	rs.timeline.tl = append(rs.timeline.tl, model.Event{ID: "$third", Room: room.ID, Sender: room.Partner, Kind: model.EventText,
		Body: "third", Time: time.Now()})
	settle(tt)
	if rs.unread != 4 {
		t.Fatal("marked read while the room is not on screen")
	}
}

// Who is typing floats over the timeline: no message moves when it comes
// and goes.
func TestTypingDoesNotMoveMessages(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 12)
	ts := &typingSource{Source: timeline{a.src, tl}}
	a.src = ts
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)

	before, ok := tt.Find("message number 10")
	if !ok {
		t.Fatal("the last messages do not show")
	}
	ts.on = true
	settle(tt)
	if !tt.HasText("Alex is typing") {
		t.Fatal("the typing line does not show")
	}
	after, _ := tt.Find("message number 10")
	if after.Y != before.Y || after.X != before.X {
		t.Fatalf("a message moved when the typing line came: %+v -> %+v", before, after)
	}
	ts.on = false
	settle(tt)
	if again, _ := tt.Find("message number 10"); again.Y != before.Y {
		t.Fatalf("a message moved when the typing line went: %+v -> %+v", before, again)
	}
}

// Pressing the quote of a reply scrolls to the message it replies to.
func TestReplyQuoteJumpsToOriginal(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 80)
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)

	if tt.HasText("message number 2") && !tt.HasText("the answer to that") {
		t.Fatal("the room did not open at its end")
	}
	// The quote above the last message, labelled with who it quotes (the
	// original, far above, is not on screen).
	who, _ := a.src.Person(a.src.(timeline).tl[2].Sender)
	if err := tt.Click("Reply to " + who.Name); err != nil {
		t.Fatalf("no quote to press: %v", err)
	}
	settle(tt)
	settle(tt)
	if !tt.HasText("message number 3") || !tt.HasText("message number 1") {
		t.Fatal("the timeline did not scroll to the message replied to")
	}
}

// olderSource counts the pages asked for.
type olderSource struct {
	timeline
	more  bool
	pages int
}

func (s *olderSource) HasOlder(string) bool { return s.more }
func (s *olderSource) LoadOlder(string)     { s.pages++ }

// Older messages are asked for when the user is near the top of the
// timeline — not while they read at the end, and not when there are none.
func TestHistoryIsPagedAtTheTop(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 200)
	src := &olderSource{timeline: timeline{a.src, tl}, more: true}
	a.src = src
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)
	if src.pages != 0 {
		t.Fatalf("asked for history while at the end: %d", src.pages)
	}
	a.listState(room.ID).ScrollTo(0, ui.Start)
	settle(tt)
	settle(tt)
	if src.pages == 0 {
		t.Fatal("did not ask for older messages at the top")
	}
	src.pages, src.more = 0, false
	settle(tt)
	if src.pages != 0 {
		t.Fatal("asked for history that is not there")
	}
}

// Jump to latest ends on the last message — in one move, not past it.
func TestJumpToLatest(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 200)
	a.src = timeline{a.src, tl}
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)

	a.listState(room.ID).ScrollTo(0, ui.Start)
	settle(tt)
	settle(tt)
	if tt.HasText("the answer to that") {
		t.Fatal("did not scroll away from the end")
	}
	if err := tt.Click("Jump to latest"); err != nil {
		t.Fatalf("no Jump to latest: %v", err)
	}
	settle(tt)
	settle(tt)
	if !tt.HasText("the answer to that") {
		t.Fatal("Jump to latest did not end on the last message")
	}
	if tt.HasText("Jump to latest") {
		t.Fatal("the button stayed at the end")
	}
}

// pagedSource holds the newest `held` of a timeline and pages older ones
// in only when told a page landed: as the Matrix layer does, LoadOlder is
// a no-op while a page is on its way.
type pagedSource struct {
	model.Source
	all        []model.Event
	held, asks int
	busy       bool
}

func (s *pagedSource) Timeline(string) []model.Event { return s.all[len(s.all)-s.held:] }
func (s *pagedSource) HasOlder(string) bool          { return s.held < len(s.all) }
func (s *pagedSource) LoadOlder(string) {
	if !s.busy {
		s.busy = true
		s.asks++
	}
}
func (s *pagedSource) land() { s.held, s.busy = min(s.held+40, len(s.all)), false }

// seekSource is pagedSource with big pages for seeks (HistorySeek), as the
// Matrix layer pages a jump to an old pin.
type seekSource struct{ *pagedSource }

func (s seekSource) LoadOlderBy(room string, n int) { s.LoadOlder(room) }
func (s seekSource) landBy(n int) {
	s.held, s.busy = min(s.held+n, len(s.all)), false
}

// At the size of a real room's months of history (thousands of rows, the
// target near the top of them), the jump shows the message.
func TestJumpFarBackShowsMessage(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 15000)
	src := seekSource{&pagedSource{Source: a.src, all: tl, held: 300}}
	a.src = src
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)

	a.jumpToEvent(room.ID, tl[84])
	tt.Frame()
	if !tt.HasText("Finding message") {
		t.Fatal("no sign that the jump is looking for the message")
	}
	for page := 0; page < 40 && a.jumpID != ""; page++ {
		tt.Frame()
		src.landBy(500)
		tt.Frame()
		if page == 12 {
			tt.Advance(500 * time.Millisecond) // mid-sweep
			tt.Frame()
			snap(t, "jump-seeking", tt)
			// Near the sweep's end the segment runs off the pill's right
			// edge: nothing of it shows past the pill.
			r, ok := tt.Find("Finding message")
			if !ok {
				t.Fatal("no seeking pill")
			}
			tt.Advance(600 * time.Millisecond)
			tt.Frame()
			snap(t, "jump-seeking-edge", tt)
			accent := kit.Light.Accent
			img := tt.Image()
			for x := int(r.X + r.W + 24); x < int(r.X+r.W+120); x++ {
				for y := int(r.Y); y < int(r.Y+r.H+16); y++ {
					px := img.RGBAAt(x, y)
					if px.R == accent.R && px.G == accent.G && px.B == accent.B {
						t.Fatalf("the sweep drew past the pill at %d,%d", x, y)
					}
				}
			}
		}
	}
	settle(tt)
	settle(tt)
	snap(t, "jump-far-back", tt)
	if !tt.HasText("message number 84") {
		t.Fatal("the timeline did not show the old message")
	}
	if tt.HasText("Finding message") {
		t.Fatal("the seeking pill stayed after the message was found")
	}
}

// A jump to a message many pages back (an old pin) keeps paging until it
// is there, however many frames go by while each page loads — the pointer
// moving makes frames, and they used to use the seek up before the first
// page came.
func TestJumpPagesBackUntilFound(t *testing.T) {
	a := testApp()
	room, tl := longTimeline(a, 400)
	src := &pagedSource{Source: a.src, all: tl, held: 30}
	a.src = src
	a.open(room.ID)
	tt := ui.NewTester(a.View, 1200, 800)
	settle(tt)

	a.jumpToEvent(room.ID, tl[5]) // 12 pages back
	for page := 0; page < 20 && a.jumpID != ""; page++ {
		for range 30 { // frames while the page is on its way
			tt.Frame()
		}
		src.land()
		settle(tt)
	}
	settle(tt)
	if a.jumpID != "" || src.asks < 10 {
		t.Fatalf("the seek did not finish: pending %q after %d pages", a.jumpID, src.asks)
	}
	if !tt.HasText("message number 5") {
		t.Fatal("the timeline did not scroll to the old message")
	}
}
