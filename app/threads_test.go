package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// thrSource is a source with threads: its timeline has a root with replies,
// and it records the replies sent.
type thrSource struct {
	timeline
	sent [][3]string
}

func (s *thrSource) SendInThread(room, root, text string) { s.sent = append(s.sent, [3]string{room, root, text}) }

func thrApp(t *testing.T) (*App, *thrSource, model.Room) {
	t.Helper()
	a := testApp()
	room, tl := longTimeline(a, 6)
	rootID := tl[2].ID
	now := time.Now()
	tl = append(tl,
		model.Event{ID: "$r1", Room: room.ID, Sender: room.Partner, Kind: model.EventText, Body: "first threaded reply", ThreadRoot: rootID, Time: now.Add(-3 * time.Minute)},
		model.Event{ID: "$r2", Room: room.ID, Sender: a.src.Me().ID, Kind: model.EventText, Body: "second threaded reply", ThreadRoot: rootID, Time: now.Add(-2 * time.Minute)},
	)
	tl[2].Thread = &model.Thread{Replies: 2, Last: now.Add(-2 * time.Minute), LastSender: a.src.Me().ID, Participants: []string{room.Partner, a.src.Me().ID}}
	ts := &thrSource{timeline: timeline{a.src, tl}}
	a.src = ts
	a.open(room.ID)
	return a, ts, room
}

// Replies in a thread are not among the room's messages; their root says
// how many there are; the chip opens the thread's panel with the root and its
// replies, where a reply can be written and sent.
func TestThreadsShowInTheirPanel(t *testing.T) {
	a, ts, room := thrApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	settle(tt)
	if tt.HasText("first threaded reply") {
		t.Fatal("a thread reply is among the room's messages")
	}
	if !tt.HasText("2 replies") {
		t.Fatal("the root does not say it has replies")
	}
	if err := tt.Click("Open the thread: 2 replies"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if a.panel != panelThread || a.thread.Root != ts.tl[2].ID {
		t.Fatalf("panel %v thread %+v", a.panel, a.thread)
	}
	if !tt.HasText("first threaded reply") || !tt.HasText("second threaded reply") || !tt.HasText("Thread") {
		t.Fatal("the thread's messages are not in its panel")
	}
	snap(t, "thread-panel", tt)

	a.thread.Draft = "a reply from here"
	if err := tt.Click("Send reply"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if len(ts.sent) != 1 || ts.sent[0] != [3]string{room.ID, a.thread.Root, "a reply from here"} {
		t.Fatalf("sent: %v", ts.sent)
	}
	if a.thread.Draft != "" {
		t.Fatal("the line was not cleared after sending")
	}
}

// "Reply in thread" is in a message's menu where there are threads, and it
// opens the thread on that message; a window too narrow for the panel shows
// the replies among the messages, with no such item.
func TestReplyInThreadFromTheMenu(t *testing.T) {
	a, ts, room := thrApp(t)
	st := a.msgMenu(ts.tl[3])
	if !st.Thread {
		t.Fatal("no Reply in thread on a threaded account")
	}
	a.openThread(room.ID, ts.tl[3])
	if a.panel != panelThread || a.thread.Root != ts.tl[3].ID {
		t.Fatalf("a thread started on a message: %v %+v", a.panel, a.thread)
	}
	// Opening it from one of its replies opens its root's thread.
	a.openThread(room.ID, model.Event{ID: "$r1", ThreadRoot: ts.tl[2].ID})
	if a.thread.Root != ts.tl[2].ID {
		t.Fatalf("a reply opened %q", a.thread.Root)
	}

	a.narrow = true
	if a.msgMenu(ts.tl[3]).Thread || a.threadsOn() {
		t.Fatal("threads shown as threads in a window with no room for the panel")
	}
	a.narrow = false

	// A source without threads has none of it.
	plain := testApp()
	if plain.msgMenu(model.Event{ID: "x"}).Thread {
		t.Fatal("the mock offers threads")
	}
}
