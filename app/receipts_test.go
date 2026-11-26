package app

import (
	"strings"
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

	readers := func(st receiptState, id string) string { return strings.Join(st.readBy[id], ",") }

	// Their place is past my first message, not my second: their picture fills
	// the first's check, the second is "sent".
	got := state(true, ev("1", me), ev("2", "@kai", "@kai"), ev("3", me))
	if got.readID != "1" || got.sentID != "3" || readers(got, "1") != "@kai" || readers(got, "3") != "" {
		t.Fatalf("%+v", got)
	}
	// Read to the end: nothing is merely "sent".
	got = state(true, ev("1", me), ev("2", me, "@kai"))
	if got.readID != "2" || got.sentID != "" || readers(got, "2") != "@kai" || readers(got, "1") != "" {
		t.Fatalf("%+v", got)
	}
	// A group: each reader once, on my last message they have reached.
	got = state(true, ev("1", me, "@leo"), ev("2", me), ev("3", "@kai"), ev("4", "@mia", "@mia"))
	if readers(got, "1") != "@leo" || readers(got, "2") != "@kai,@mia" || got.sentID != "" {
		t.Fatalf("group: %+v", got)
	}
	// Nobody has read anything.
	got = state(true, ev("1", me))
	if got.readID != "" || got.sentID != "1" || len(got.readBy) != 0 {
		t.Fatalf("%+v", got)
	}
	// Off: the receipts are not looked at, but the last message is still sent:
	// that says nothing of anyone else.
	got = state(false, ev("1", me, "@kai"))
	if got.sentID != "1" || len(got.readBy) != 0 {
		t.Fatalf("off: %+v", got)
	}
	// Someone answered: they have read it, setting or not (their answer is
	// there for all to see). Their picture, not a lone "sent".
	for _, on := range []bool{false, true} {
		got = state(on, ev("1", me), ev("2", "@kai"))
		if got.sentID != "" || readers(got, "1") != "@kai" {
			t.Fatalf("answered (receipts %v): %+v", on, got)
		}
	}
	if got = state(false); got.sentID != "" || got.readID != "" || len(got.readBy) != 0 {
		t.Fatalf("off, no messages: %+v", got)
	}
	// The check is for a message the server has: not one still being sent, and
	// not one that failed — the last delivered one carries it.
	pending, failed := ev("3", me), ev("4", me)
	pending.Pending = true
	failed.Failed, failed.Error = true, "Not sent: No internet connection."
	got = state(true, ev("1", me), pending)
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

// In a group the check fills with every reader's picture, overlapping, on
// desktop and phone (snapshots with SNAP_DIR).
func TestGroupReadersRender(t *testing.T) {
	a := testApp()
	me := a.src.Me().ID
	now := time.Now()
	a.src = timeline{a.src, []model.Event{
		{ID: "1", Sender: "@mira:matrix.org", Kind: model.EventText, Body: "who's in for friday?", Time: now.Add(-4 * time.Minute)},
		{ID: "2", Sender: me, Kind: model.EventText, Body: "me!", Time: now.Add(-3 * time.Minute),
			ReadBy: []string{"@sam:matrix.org", "@jun:kde.org", "@kai:example.org", "@ola:matrix.org"}},
	}}
	a.prefs.receipts = true
	a.open("!crit")
	for _, phone := range []bool{false, true} {
		w, h, name := 900, 400, "group-readers"
		if phone {
			w, h, name = 390, 400, "group-readers-phone"
			a.pushed = true
		}
		tt := ui.NewTester(a.View, w, h)
		settle(tt)
		snap(t, name, tt)
		if !tt.HasText("Read by") || !tt.HasText("+1") {
			t.Fatalf("%s: no readers (texts: %v)", name, tt.Texts())
		}
	}
}

// Read receipts are off until the user turns them on: a receipt alone shows
// nothing, and the receipts the app sends are private.
func TestReceiptsOffByDefault(t *testing.T) {
	a := testApp()
	if a.prefs.receipts {
		t.Fatal("read receipts default on")
	}
	me := a.src.Me().ID
	a.src = timeline{a.src, []model.Event{{ID: "1", Sender: me, Kind: model.EventText, ReadBy: []string{"@kai"}}}}
	a.rs = a.receiptStateOf("r")
	if got := a.readers(model.Event{ID: "1"}); len(got) != 0 {
		t.Fatalf("readers shown while off: %v", got)
	}
}

// With receipts on, the partner's picture fills the check of the user's
// message they have read, on desktop and phone.
func TestReceiptsOn(t *testing.T) {
	a := testApp()
	r, _ := dmWithReader(t, a)
	me := a.src.Me().ID
	now := time.Now()
	a.src = timeline{a.src, []model.Event{
		{ID: "1", Sender: r.Partner, Kind: model.EventText, Body: "are you there?", Time: now.Add(-3 * time.Minute)},
		{ID: "2", Sender: me, Kind: model.EventText, Body: "yes, one second", Time: now.Add(-2 * time.Minute), ReadBy: []string{r.Partner}},
	}}
	a.prefs.receipts = true
	a.rs = a.receiptStateOf(r.ID)
	if got := a.readers(model.Event{ID: "2"}); len(got) != 1 {
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
