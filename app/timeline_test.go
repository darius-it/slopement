package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

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
