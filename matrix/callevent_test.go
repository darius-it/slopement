package matrix

import (
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// callNote is a call notification event (kind: "ring" or "notification") sent
// age ago.
func callNote(room, evtID, sender, kind string, age time.Duration) *event.Event {
	return &event.Event{
		ID: id.EventID(evtID), RoomID: id.RoomID(room), Sender: id.UserID(sender), Type: rtcNotificationTypes[0],
		Timestamp: time.Now().Add(-age).UnixMilli(), Content: event.Content{Raw: map[string]any{"notification_type": kind}},
	}
}

// Somebody calling notifies once, while it is fresh; one's own call, a stale
// one, a muted room and a plain room notification (not a ring) do not.
func TestIncomingCallNotifies(t *testing.T) {
	s, _ := testSynced()
	s.me.ID = "@me:x"
	s.people["@kai:x"] = &model.Person{ID: "@kai:x", Name: "Kai"}
	r := s.roomOf("!dm")
	r.dm, r.partner, r.name = true, "@kai:x", "Kai"
	var got []model.Notice
	s.notify = func(n model.Notice) { got = append(got, n) }

	ring := callNote("!dm", "$1", "@kai:x", "ring", 0)
	s.onCallNotification(nil, ring)
	s.onCallNotification(nil, ring) // the same again: not twice
	if len(got) != 1 || got[0].Title != "Kai" || got[0].Body != "Incoming call" || got[0].Room != "!dm" {
		t.Fatalf("notices: %+v", got)
	}
	s.onCallNotification(nil, callNote("!dm", "$2", "@me:x", "ring", 0))
	s.onCallNotification(nil, callNote("!dm", "$3", "@kai:x", "ring", 10*time.Minute))
	r.muted = true
	s.onCallNotification(nil, callNote("!dm", "$4", "@kai:x", "ring", 0))
	if len(got) != 1 {
		t.Fatalf("it notified for what it should not: %+v", got)
	}

	// In a group room, a ring tells; a plain "a call started" does not.
	g := s.roomOf("!grp")
	g.name = "Plans"
	s.onCallNotification(nil, callNote("!grp", "$5", "@kai:x", "notification", 0))
	if len(got) != 1 {
		t.Fatalf("a plain call in a group notified: %+v", got)
	}
	s.onCallNotification(nil, callNote("!grp", "$6", "@kai:x", "ring", 0))
	if len(got) != 2 || got[1].Title != "Plans" || got[1].Body != "Kai is calling" {
		t.Fatalf("a ring in a group: %+v", got)
	}
}

// When the last person leaves a call that was going, the chat says it ended.
func TestCallEndedLine(t *testing.T) {
	s, _ := testSynced()
	r := s.roomOf("!r")
	at := func(typ, key, sender, content string) *event.Event {
		e := memberEvent(typ, key, sender, content)
		e.Mautrix.EventSource = event.SourceTimeline
		e.Timestamp = time.Now().UnixMilli()
		return e
	}
	count := func() (n int) {
		for _, e := range r.events {
			if e.Kind == model.EventSystem && e.Body == "The call ended" {
				n++
			}
		}
		return n
	}
	s.onCallEvent(nil, at("m.rtc.member", "_@a:x_1", "@a:x", `{"slot_id":"s"}`))
	s.onCallEvent(nil, at("m.rtc.member", "_@b:x_1", "@b:x", `{"slot_id":"s"}`))
	s.onCallEvent(nil, at("m.rtc.member", "_@a:x_1", "@a:x", `{}`))
	if count() != 0 {
		t.Fatal("it ended with someone still in it")
	}
	s.onCallEvent(nil, at("m.rtc.member", "_@b:x_1", "@b:x", `{}`))
	if count() != 1 {
		t.Fatalf("%d \"call ended\" lines, want 1", count())
	}
	// Emptying something never seen is not an end.
	s.onCallEvent(nil, at("m.rtc.member", "_@c:x_1", "@c:x", `{}`))
	if count() != 1 {
		t.Fatalf("a second end appeared: %d", count())
	}
}

// The ended line carries how long the call lasted: from the first person in to
// the last out.
func TestCallEndedLineHasItsLength(t *testing.T) {
	s, _ := testSynced()
	r := s.roomOf("!r")
	start := time.Now().Add(-12 * time.Minute)
	at := func(key, sender, content string, when time.Time) *event.Event {
		e := memberEvent("m.rtc.member", key, sender, content)
		e.Mautrix.EventSource = event.SourceTimeline
		e.Timestamp = when.UnixMilli()
		return e
	}
	s.onCallEvent(nil, at("_@a:x_1", "@a:x", `{"slot_id":"s"}`, start))
	s.onCallEvent(nil, at("_@b:x_1", "@b:x", `{"slot_id":"s"}`, start.Add(2*time.Minute)))
	s.onCallEvent(nil, at("_@a:x_1", "@a:x", `{}`, start.Add(10*time.Minute)))
	s.onCallEvent(nil, at("_@b:x_1", "@b:x", `{}`, start.Add(12*time.Minute)))
	var end *model.Event
	for i := range r.events {
		if r.events[i].Call != nil && r.events[i].Call.Ended {
			end = &r.events[i]
		}
	}
	if end == nil {
		t.Fatal("no ended line")
	}
	if got := end.Call.Duration; got < 11*time.Minute+55*time.Second || got > 12*time.Minute+5*time.Second {
		t.Fatalf("the length is %v, want about 12 minutes", got)
	}
	// A second call is timed from its own start.
	s.onCallEvent(nil, at("_@a:x_1", "@a:x", `{"slot_id":"s"}`, start.Add(30*time.Minute)))
	s.onCallEvent(nil, at("_@a:x_1", "@a:x", `{}`, start.Add(33*time.Minute)))
	last := r.events[len(r.events)-1]
	if last.Call == nil || last.Call.Duration < 2*time.Minute+55*time.Second || last.Call.Duration > 3*time.Minute+5*time.Second {
		t.Fatalf("the second call: %+v", last.Call)
	}
}

// Older history is paged from the oldest event the server gave the room, never
// from a line the app made itself ("The call ended"): with such a line at the
// top, paging asked the server about an event that does not exist and stalled.
func TestHistoryPagesFromARealEventNotACallLine(t *testing.T) {
	s, _ := testSynced()
	r := s.roomOf("!r")
	now := time.Now()
	r.events = []model.Event{
		{ID: linePrefix + "call-ended:!r:1", Room: "!r", Time: now.Add(-3 * time.Hour), Kind: model.EventSystem},
		{ID: "$real1", Room: "!r", Time: now.Add(-2 * time.Hour)},
		{ID: "$real2", Room: "!r", Time: now.Add(-1 * time.Hour)},
	}
	if got := r.oldestServerID(); got != "$real1" {
		t.Fatalf("history would be paged from %q, want $real1", got)
	}
	// Only lines of the app's own: nothing to page from.
	r.events = r.events[:1]
	if got := r.oldestServerID(); got != "" {
		t.Fatalf("paged from %q with only an app line held", got)
	}
}

// The call line stays in the cache, an unsent message does not; and a line an
// earlier build kept under its old ID becomes the app's own on load.
func TestCallLinesAreCachedAndOldOnesFixed(t *testing.T) {
	s, _ := testSynced()
	r := s.roomOf("!r")
	now := time.Now()
	r.events = []model.Event{
		{ID: linePrefix + "call-ended:!r:1", Room: "!r", Time: now, Kind: model.EventSystem, Body: "The call ended"},
		{ID: localPrefix + "txn", Room: "!r", Time: now, Body: "not sent"},
	}
	rec := r.record()
	if len(rec.Events) != 1 || rec.Events[0].Body != "The call ended" {
		t.Fatalf("cached events: %+v", rec.Events)
	}
	rec.Events = append(rec.Events, model.Event{ID: "call-ended:!r:0", Room: "!r", Time: now.Add(-time.Hour), Kind: model.EventSystem})
	back := rec.room()
	for _, e := range back.events {
		if e.ID == "call-ended:!r:0" {
			t.Fatal("an old-style call line kept its server-looking ID")
		}
	}
	if got := back.oldestServerID(); got != "" {
		t.Fatalf("paging would start from %q", got)
	}
}

// A call notification that waited for its key shows as "waiting for this
// message"; when it decrypts it becomes "<who> started a call", and the lock
// tile and its raw copy are gone.
func TestCallNotificationReplacesItsLockTile(t *testing.T) {
	s, _ := testSynced()
	s.me.ID = "@me:x"
	s.people["@kai:x"] = &model.Person{ID: "@kai:x", Name: "Kai"}
	r := s.roomOf("!r")
	locked := model.Event{ID: "$n", Room: "!r", Sender: "@kai:x", Time: time.Now(), Kind: model.EventUndecryptable}
	r.events = []model.Event{locked}
	r.raw = map[string]*rawEvent{"$n": {evt: &event.Event{ID: "$n"}, tile: locked}}
	s.encrypted = 1

	s.onCallNotification(nil, callNote("!r", "$n", "@kai:x", "ring", 0))

	if len(r.events) != 1 || r.events[0].Kind != model.EventSystem || r.events[0].Body != "Kai started a call" {
		t.Fatalf("events: %+v", r.events)
	}
	if len(r.raw) != 0 || s.encrypted != 0 {
		t.Fatalf("the waiting copy stayed: raw %d, encrypted %d", len(r.raw), s.encrypted)
	}
	// One's own is "You".
	s.onCallNotification(nil, callNote("!r", "$m", "@me:x", "ring", 0))
	if got := r.events[len(r.events)-1].Body; got != "You started a call" {
		t.Fatalf("own line: %q", got)
	}
}
