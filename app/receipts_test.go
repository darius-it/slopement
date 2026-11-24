package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/model"
)

// dmWithReader is a DM whose partner has read the user's last message.
func dmWithReader(t *testing.T, a *App) (model.Room, model.Event) {
	t.Helper()
	for _, r := range a.src.Rooms() {
		if r.Kind != model.KindDM {
			continue
		}
		for _, ev := range a.src.Timeline(r.ID) {
			if len(ev.ReadBy) > 0 {
				return r, ev
			}
		}
	}
	t.Fatal("the mock has no DM with a reader")
	return model.Room{}, model.Event{}
}

// timeline is a Source whose room timeline is given.
type timeline struct {
	model.Source
	tl []model.Event
}

func (t timeline) Timeline(string) []model.Event { return t.tl }

// The user's last message is "sent" until somebody's place reaches it;
// the last one anybody has read is "read".
func TestReceiptState(t *testing.T) {
	me := testApp().src.Me().ID
	ev := func(id, sender string, readBy ...string) model.Event {
		return model.Event{ID: id, Sender: sender, Kind: model.EventText, ReadBy: readBy}
	}
	state := func(on bool, tl ...model.Event) receiptState {
		a := testApp()
		a.src = timeline{a.src, tl}
		a.prefs.receipts = on
		return a.receiptStateOf("r")
	}

	// Their place is past my first message, not my second.
	got := state(true, ev("1", me), ev("2", "@kai", "@kai"), ev("3", me))
	if got.readID != "1" || got.sentID != "3" {
		t.Fatalf("read=%q sent=%q", got.readID, got.sentID)
	}
	// Read to the end: nothing is merely "sent".
	got = state(true, ev("1", me), ev("2", me, "@kai"))
	if got.readID != "2" || got.sentID != "" {
		t.Fatalf("read=%q sent=%q", got.readID, got.sentID)
	}
	// Nobody has read anything.
	got = state(true, ev("1", me))
	if got.readID != "" || got.sentID != "1" {
		t.Fatalf("read=%q sent=%q", got.readID, got.sentID)
	}
	// Off: nothing of who has read what (the place that was read is not looked
	// at), but the last message is still sent: that says nothing of anyone else.
	if got = state(false, ev("1", me), ev("2", "@kai", "@kai")); got != (receiptState{sentID: "1"}) {
		t.Fatalf("off: %+v", got)
	}
	if got = state(false); got != (receiptState{}) {
		t.Fatalf("off, no messages: %+v", got)
	}
	// The check is for a message the server has: not one still being sent, and
	// not one that failed — the last delivered one carries it.
	pending, failed := ev("3", me), ev("4", me)
	pending.Pending = true
	failed.Failed, failed.Error = true, "Not sent: No internet connection."
	got = state(true, ev("1", me), ev("2", "@kai"), pending)
	if got.sentID != "1" {
		t.Fatalf("a message still being sent took the check: %q", got.sentID)
	}
	got = state(true, ev("1", me), failed)
	if got.sentID != "1" {
		t.Fatalf("a message that failed took the check: %q", got.sentID)
	}
	if got = state(true, failed); got.sentID != "" || got.readID != "" {
		t.Fatalf("a message that failed was marked: %+v", got)
	}
}

// The hollow check on the user's unread last message, on desktop and phone
// (a snapshot with SNAP_DIR).
func TestSentCheckRenders(t *testing.T) {
	a := testApp()
	r, _ := dmWithReader(t, a)
	me := a.src.Me().ID
	now := time.Now()
	tl := []model.Event{
		{ID: "1", Sender: r.Partner, Kind: model.EventText, Body: "are you there?", Time: now.Add(-3 * time.Minute), ReadBy: []string{r.Partner}},
		{ID: "2", Sender: me, Kind: model.EventText, Body: "yes, one second", Time: now.Add(-2 * time.Minute)},
	}
	a.src = timeline{a.src, tl}
	a.prefs.receipts = true
	a.open(r.ID)
	for _, phone := range []bool{false, true} {
		w, h, name := 900, 500, "sent-check"
		if phone {
			w, h, name = 390, 500, "sent-check-phone"
		}
		tt := ui.NewTester(a.View, w, h)
		settle(tt)
		if phone && !tt.HasText("Sent") {
			t.Fatal("the phone bubble does not say Sent")
		}
		snap(t, name, tt)
	}
}

// Read receipts are off until the user turns them on: nothing shows, and
// the receipts the app sends are private.
func TestReceiptsOffByDefault(t *testing.T) {
	a := testApp()
	if a.prefs.receipts {
		t.Fatal("read receipts default on")
	}
	_, ev := dmWithReader(t, a)
	if got := a.readers(ev); len(got) != 0 {
		t.Fatalf("readers shown while off: %v", got)
	}
}

func TestReceiptsOn(t *testing.T) {
	a := testApp()
	r, ev := dmWithReader(t, a)
	a.prefs.receipts = true
	if got := a.readers(ev); len(got) != 1 {
		t.Fatalf("readers = %v", got)
	}
	a.open(r.ID)
	for _, phone := range []bool{false, true} {
		w, h := 1200, 800
		if phone {
			w, h = 390, 844
		}
		tt := ui.NewTester(a.View, w, h)
		settle(tt)
		name := "receipts-on"
		if phone {
			name += "-phone"
			if !tt.HasText("Read") {
				t.Fatal("the phone bubble does not say Read")
			}
		}
		snap(t, name, tt)
	}
}
