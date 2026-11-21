package matrix

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// feed is a Source that runs what its handlers post at once, as the
// window's update function does on its own thread.
type feed struct{ fns []func() }

func (f *feed) post(fn func()) { fn() }

func testSynced() (*Source, *feed) {
	s := NewSource(&Session{UserID: "@me:example.org", DeviceID: "DEV"})
	f := &feed{}
	s.update = f.post
	return s, f
}

func TestOnSyncRooms(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!a": {UnreadNotifications: &mautrix.UnreadNotificationCounts{NotificationCount: 3, HighlightCount: 1}},
	}}})

	rooms := s.Rooms()
	if len(rooms) != 1 || rooms[0].Unread != 3 || rooms[0].Mentions != 1 {
		t.Fatalf("rooms = %+v", rooms)
	}
}

func TestOnMessage(t *testing.T) {
	s, _ := testSynced()

	// The room arrives with its envelope; the message lands in the
	// timeline, counted unread.
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!a": {UnreadNotifications: nil},
	}}})
	c := &event.MessageEventContent{MsgType: event.MsgText, Body: "hello"}
	c.Mentions = &event.Mentions{UserIDs: []id.UserID{"@me:example.org"}}
	s.onMessage(nil, msgEvent("$1", "!a", "@ada:example.org", 1000, c))

	tl := s.Timeline("!a")
	if len(tl) != 1 || tl[0].Body != "hello" || !tl[0].Mentions {
		t.Fatalf("timeline = %+v", tl)
	}
	m, _ := s.Room("!a")
	if m.Unread != 1 || m.Mentions != 1 || m.Preview != "ada: hello" {
		t.Fatalf("room = %+v", m)
	}

	// The user's own message reads back: no unread.
	own := &event.MessageEventContent{MsgType: event.MsgText, Body: "hi"}
	s.onMessage(nil, msgEvent("$2", "!a", "@me:example.org", 2000, own))
	m, _ = s.Room("!a")
	if m.Unread != 1 || m.Preview != "You: hi" {
		t.Fatalf("own message: %+v", m)
	}
}

func TestOnMemberStateVsTimeline(t *testing.T) {
	s, _ := testSynced()

	// A member event from the initial sync's state dump: counted, but no
	// system line — a room's whole history must not flood its timeline.
	stateJoin := &event.Event{
		ID: "$j", RoomID: id.RoomID("!a"), Sender: id.UserID("@ada:example.org"),
		StateKey: ptr("@ada:example.org"), Timestamp: 1, Type: event.StateMember,
		Mautrix: event.MautrixInfo{EventSource: event.SourceJoin | event.SourceState},
		Content: event.Content{Parsed: &event.MemberEventContent{Membership: event.MembershipJoin}},
	}
	s.onMember(nil, stateJoin)
	if len(s.Timeline("!a")) != 0 {
		t.Fatal("the state dump wrote a timeline line")
	}
	if _, ok := s.Person("@ada:example.org"); !ok {
		t.Fatal("the member was not recorded")
	}

	// The same change live, from the timeline: one quiet line.
	liveJoin := &event.Event{
		ID: "$j2", RoomID: id.RoomID("!a"), Sender: id.UserID("@ada:example.org"),
		StateKey: ptr("@ada:example.org"), Timestamp: 2, Type: event.StateMember,
		Mautrix: event.MautrixInfo{EventSource: event.SourceJoin | event.SourceTimeline},
		Content: event.Content{Parsed: &event.MemberEventContent{Membership: event.MembershipJoin}},
	}
	s.onMember(nil, liveJoin)
	tl := s.Timeline("!a")
	if len(tl) != 1 || tl[0].Kind != model.EventSystem || tl[0].Body != "ada joined" {
		t.Fatalf("timeline = %+v", tl)
	}
}

func TestOnReceiptMine(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!a": {UnreadNotifications: &mautrix.UnreadNotificationCounts{NotificationCount: 7, HighlightCount: 2}},
	}}})

	receipts := event.ReceiptEventContent{}
	receipts.Set(id.EventID("$1"), event.ReceiptTypeRead, id.UserID("@me:example.org"), event.ReadReceipt{})
	evt := &event.Event{
		RoomID: id.RoomID("!a"), Type: event.EphemeralEventReceipt,
		Content: event.Content{Parsed: &receipts},
	}
	s.onReceipt(nil, evt)
	m, _ := s.Room("!a")
	if m.Unread != 0 || m.Mentions != 0 {
		t.Fatalf("after my receipt: %+v", m)
	}
}

// Others' public receipts say how far they have read: on the event they
// name, or — for one the room does not show — on the last shown event
// before the receipt's time. Their private ones never arrive; the user's
// own private one clears the room too.
func TestOnReceiptOthers(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!a": {UnreadNotifications: &mautrix.UnreadNotificationCounts{NotificationCount: 4}},
	}}})
	r := s.rooms["!a"]
	t0 := time.UnixMilli(1_000_000)
	for i, idv := range []string{"$1", "$2", "$3"} {
		r.appendEvent(model.Event{ID: idv, Sender: "@me:example.org", Body: idv, Kind: model.EventText,
			Time: t0.Add(time.Duration(i) * time.Minute)}, "You", true)
	}

	receipt := func(target id.EventID, typ event.ReceiptType, user string, at time.Time) *event.Event {
		c := event.ReceiptEventContent{}
		c.Set(target, typ, id.UserID(user), event.ReadReceipt{Timestamp: at})
		return &event.Event{RoomID: "!a", Type: event.EphemeralEventReceipt, Content: event.Content{Parsed: &c}}
	}
	s.onReceipt(nil, receipt("$2", event.ReceiptTypeRead, "@kai:example.org", t0.Add(time.Minute)))
	// A reaction's event ID the room has no tile for, read at the time of $3.
	s.onReceipt(nil, receipt("$unshown", event.ReceiptTypeRead, "@ada:example.org", t0.Add(2*time.Minute+time.Second)))
	// Another's private receipt, and an older one arriving late: no effect.
	s.onReceipt(nil, receipt("$3", event.ReceiptTypeReadPrivate, "@zed:example.org", t0.Add(2*time.Minute)))
	s.onReceipt(nil, receipt("$1", event.ReceiptTypeRead, "@kai:example.org", t0))
	// The user's own private receipt.
	s.onReceipt(nil, receipt("$3", event.ReceiptTypeReadPrivate, "@me:example.org", t0))

	tl := s.Timeline("!a")
	if got := tl[1].ReadBy; len(got) != 1 || got[0] != "@kai:example.org" {
		t.Fatalf("$2 read by %v", got)
	}
	if got := tl[2].ReadBy; len(got) != 1 || got[0] != "@ada:example.org" {
		t.Fatalf("$3 read by %v", got)
	}
	if len(tl[0].ReadBy) != 0 {
		t.Fatalf("$1 read by %v", tl[0].ReadBy)
	}
	if m, _ := s.Room("!a"); m.Unread != 0 {
		t.Fatalf("my private receipt left %d unread", m.Unread)
	}
}

// Sending is reading: a person is at least on their own latest message,
// and a receipt further on wins over it.
func TestImplicitReads(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{"!a": {}}}})
	r := s.rooms["!a"]
	t0 := time.UnixMilli(2_000_000)
	add := func(idv, sender string, min int) {
		r.appendEvent(model.Event{ID: idv, Sender: sender, Kind: model.EventText, Time: t0.Add(time.Duration(min) * time.Minute)}, sender, sender == "@me:example.org")
	}
	add("$1", "@me:example.org", 0)
	add("$2", "@kai:example.org", 1) // kai wrote: kai has read up to here
	add("$3", "@me:example.org", 2)
	add("$4", "@ada:example.org", 3)
	add("$5", "@me:example.org", 4)

	// ada's receipt is further on than her message.
	c := event.ReceiptEventContent{}
	c.Set("$5", event.ReceiptTypeRead, "@ada:example.org", event.ReadReceipt{Timestamp: t0.Add(4 * time.Minute)})
	s.onReceipt(nil, &event.Event{RoomID: "!a", Type: event.EphemeralEventReceipt, Content: event.Content{Parsed: &c}})

	tl := s.Timeline("!a")
	at := func(i int) string { return fmt.Sprint(tl[i].ReadBy) }
	if at(1) != "[@kai:example.org]" || at(3) != "[]" || at(4) != "[@ada:example.org]" {
		t.Fatalf("$2=%s $4=%s $5=%s", at(1), at(3), at(4))
	}
	if len(tl[0].ReadBy)+len(tl[2].ReadBy) != 0 {
		t.Fatal("the user's own messages carry no reader of their own")
	}
}

func TestOnDirectAndTags(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!dm": {UnreadNotifications: nil},
	}}})

	direct := event.DirectChatsEventContent{"@ada:example.org": []id.RoomID{"!dm"}}
	evt := &event.Event{Type: event.AccountDataDirectChats, Content: event.Content{Parsed: &direct}}
	s.onDirect(nil, evt)

	tags := event.TagEventContent{Tags: event.Tags{"m.favourite": event.TagMetadata{}}}
	tagEvt := &event.Event{
		RoomID: id.RoomID("!dm"), Type: event.AccountDataRoomTags,
		Content: event.Content{Parsed: &tags},
	}
	s.onTags(nil, tagEvt)

	m, _ := s.Room("!dm")
	if m.Kind != model.KindDM || m.Partner != "@ada:example.org" || !m.Favourite {
		t.Fatalf("room = %+v", m)
	}
}

// TestSpaces pins the communities: a room of type m.space with
// m.space.child state, its rooms listed in its order, and each child
// room knowing the space it belongs to (so Chats leaves them alone).
func TestSpaces(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!space": {}, "!one": {}, "!two": {},
	}}})

	// The space room, and its children: two added in the order Element
	// would show them, one without an order coming last.
	create := &event.Event{
		RoomID: id.RoomID("!space"), Type: event.StateCreate,
		Content: event.Content{Parsed: &event.CreateEventContent{Type: event.RoomTypeSpace}},
	}
	s.onCreate(nil, create)
	child := func(childID, order string) *event.Event {
		return &event.Event{
			RoomID: id.RoomID("!space"), Type: event.StateSpaceChild,
			StateKey: ptr(childID),
			Content:  event.Content{Parsed: &event.SpaceChildEventContent{Order: order}},
		}
	}
	s.onSpaceChild(nil, child("!two", "b"))
	s.onSpaceChild(nil, child("!one", "a"))
	s.onSpaceChild(nil, child("!three", ""))
	// A repeat of the full boot does not duplicate.
	s.onSpaceChild(nil, child("!two", "b"))

	comms := s.Communities()
	if len(comms) != 1 || comms[0].ID != "!space" {
		t.Fatalf("communities = %+v", comms)
	}
	if got := comms[0].Rooms; len(got) != 3 || got[0] != "!one" || got[1] != "!two" || got[2] != "!three" {
		t.Fatalf("children = %v", got)
	}

	// Each child room carries its space.
	for _, roomID := range []string{"!one", "!two"} {
		m, ok := s.Room(roomID)
		if !ok || m.Community != "!space" {
			t.Fatalf("%s community = %q, %v", roomID, m.Community, ok)
		}
	}
	// A plain room belongs to none.
	if m, _ := s.Room("!space"); m.Community != "" {
		t.Fatalf("the space itself claims a community: %q", m.Community)
	}
}

// TestEncryptedNeeded pins the prompt's signal: a message that does not
// decrypt sets Needed; its decrypted re-dispatch clears it.
// A space is a community on the rail; it must never show among the rooms
// of Chats.
func TestSpacesStayOutOfRooms(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!space": {}, "!chat": {},
	}}})
	s.rooms["!space"].space = true

	rooms := s.Rooms()
	if len(rooms) != 1 || rooms[0].ID != "!chat" {
		t.Fatalf("rooms = %+v", rooms)
	}
	if cs := s.Communities(); len(cs) != 1 || cs[0].ID != "!space" {
		t.Fatalf("communities = %+v", cs)
	}
}

// A sent message shows pending at once and ends up once, whichever of
// the send's answer and the sync's echo comes first.
func TestLocalEcho(t *testing.T) {
	for _, echoFirst := range []bool{false, true} {
		r := &room{id: "!a"}
		r.appendEvent(model.Event{ID: localPrefix + "1", Pending: true, Body: "hi"}, "You", true)
		if echoFirst {
			r.appendEvent(model.Event{ID: "$real", Body: "hi"}, "You", true)
			r.settleLocal(localPrefix+"1", "$real")
		} else {
			r.settleLocal(localPrefix+"1", "$real")
			r.appendEvent(model.Event{ID: "$real", Body: "hi"}, "You", true)
		}
		if len(r.events) != 1 || r.events[0].ID != "$real" || r.events[0].Pending {
			t.Fatalf("echoFirst=%v: events = %+v", echoFirst, r.events)
		}
	}
	r := &room{id: "!a"}
	r.appendEvent(model.Event{ID: localPrefix + "2", Pending: true}, "You", true)
	r.failLocal(localPrefix+"2", "Not sent: no")
	if e := r.events[0]; !e.Failed || e.Pending || e.Error != "Not sent: no" || e.Kind == model.EventSystem {
		t.Fatalf("failed = %+v", e)
	}
}

// What the user sends shows at once and never turns into a lock tile: the
// server's encrypted echo of it, whichever comes first, takes the local
// message's place without a flicker.
func TestEchoNeverFlickers(t *testing.T) {
	encrypted := func(idv string) *event.Event {
		return &event.Event{ID: id.EventID(idv), RoomID: "!a", Sender: "@me:example.org", Type: event.EventEncrypted,
			Timestamp: time.Now().UnixMilli(), Unsigned: event.Unsigned{TransactionID: "7"},
			Content: event.Content{Parsed: &event.EncryptedEventContent{SessionID: "s", Algorithm: id.AlgorithmMegolmV1}}}
	}
	for _, echoFirst := range []bool{false, true} {
		s, _ := testSynced()
		s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{"!a": {}}}})
		r := s.rooms["!a"]
		r.appendEvent(model.Event{ID: localPrefix + "7", Sender: "@me:example.org", Body: "hello", Kind: model.EventText, Pending: true, Time: time.Now()}, "You", true)
		if echoFirst {
			s.onEncrypted(nil, encrypted("$real"))
			r.settleLocal(localPrefix+"7", "$real")
		} else {
			r.settleLocal(localPrefix+"7", "$real")
			s.onEncrypted(nil, encrypted("$real"))
		}
		if len(r.events) != 1 || r.events[0].ID != "$real" || r.events[0].Body != "hello" || r.events[0].Kind != model.EventText {
			t.Fatalf("echoFirst=%v: events = %+v", echoFirst, r.events)
		}
		if s.encrypted != 0 || len(r.raw) != 0 {
			t.Fatalf("echoFirst=%v: the echo was kept as waiting for keys", echoFirst)
		}
	}
}

// A lock tile never replaces a message that is already readable.
func TestLockNeverReplacesText(t *testing.T) {
	r := &room{id: "!a"}
	r.appendEvent(model.Event{ID: "$1", Body: "text", Kind: model.EventText}, "kai", false)
	r.appendEvent(model.Event{ID: "$1", Kind: model.EventUndecryptable}, "kai", false)
	if len(r.events) != 1 || r.events[0].Body != "text" {
		t.Fatalf("events = %+v", r.events)
	}
}

// A message that arrived a moment ago and cannot be read yet is kept out
// of sight (and out of the previews and the verify count); after the grace
// period it shows as waiting.
func TestFreshLockIsHidden(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{"!a": {}}}})
	for _, tc := range []struct {
		idv  string
		age  time.Duration
		show bool
	}{{"$fresh", time.Second, false}, {"$old", time.Minute, true}} {
		evt := &event.Event{ID: id.EventID(tc.idv), RoomID: "!a", Sender: "@kai:example.org", Type: event.EventEncrypted,
			Timestamp: time.Now().Add(-tc.age).UnixMilli(),
			Content:   event.Content{Parsed: &event.EncryptedEventContent{SessionID: "s", Algorithm: id.AlgorithmMegolmV1}}}
		s.onEncrypted(nil, evt)
		shown := false
		for _, e := range s.Timeline("!a") {
			shown = shown || e.ID == tc.idv
		}
		if shown != tc.show {
			t.Fatalf("%s: shown = %v, want %v", tc.idv, shown, tc.show)
		}
	}
	if got := s.stuck(); got != 1 {
		t.Fatalf("stuck = %d: only the old one has waited long enough to count", got)
	}
	if p := s.rooms["!a"].preview; strings.Contains(p, "$fresh") || p == "" {
		t.Fatalf("preview = %q", p)
	}
}

// A sub-space knows its parent, and the parent its sub-spaces.
func TestSubSpaces(t *testing.T) {
	s, _ := testSynced()
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!top": {}, "!sub": {}, "!room": {},
	}}})
	s.rooms["!top"].space, s.rooms["!sub"].space = true, true
	s.rooms["!top"].children = []spaceChild{{room: "!sub"}}
	s.rooms["!sub"].children = []spaceChild{{room: "!room"}}

	byID := map[string]model.Community{}
	for _, cm := range s.Communities() {
		byID[cm.ID] = cm
	}
	if byID["!sub"].Parent != "!top" || byID["!top"].Parent != "" {
		t.Fatalf("parents: %+v", byID)
	}
	if len(byID["!top"].Spaces) != 1 || byID["!top"].Spaces[0] != "!sub" || len(byID["!top"].Rooms) != 0 {
		t.Fatalf("top: %+v", byID["!top"])
	}
	if got, _ := s.Room("!room"); got.Community != "!sub" {
		t.Fatalf("room community = %q", got.Community)
	}
}

// Posts made while a sync response is processed apply together, once.
func TestBatchAppliesOnce(t *testing.T) {
	s := NewSource(&Session{UserID: "@me:example.org"})
	updates := 0
	s.update = func(fn func()) { updates++; fn() }
	s.beginBatch()
	s.post(func() { s.roomOf("!a") })
	s.post(func() { s.roomOf("!b") })
	if updates != 0 || len(s.rooms) != 0 {
		t.Fatalf("applied early: updates=%d rooms=%d", updates, len(s.rooms))
	}
	s.endBatch("", nil)
	if updates != 1 || len(s.rooms) != 2 {
		t.Fatalf("updates=%d rooms=%d", updates, len(s.rooms))
	}
}

func TestEncryptedNeeded(t *testing.T) {
	s, _ := testSynced()
	if s.Verify() != nil {
		t.Fatal("the pre-sync source has a verifier")
	}
	// The verify driver is wired by NewSynced; the test sets one with the
	// Source's own post.
	s.verify = &VerifyDriver{src: s}

	enc := &event.Event{
		ID: "$e", RoomID: id.RoomID("!r"), Sender: id.UserID("@ada:example.org"),
		Timestamp: 1, Type: event.EventEncrypted,
		Content: event.Content{Parsed: &event.EncryptedEventContent{}},
	}
	s.onEncrypted(nil, enc)
	tl := s.Timeline("!r")
	if len(tl) != 1 || tl[0].Kind != model.EventUndecryptable {
		t.Fatalf("timeline = %+v", tl)
	}
	if !s.Verify().State().Needed {
		t.Fatal("an undecrypted message did not ask for verification")
	}

	// The keys arrived: the same event re-dispatched decrypted.
	s.onMessage(nil, msgEvent("$e", "!r", "@ada:example.org", 1,
		&event.MessageEventContent{MsgType: event.MsgText, Body: "now readable"}))
	tl = s.Timeline("!r")
	if len(tl) != 1 || tl[0].Kind != model.EventText || tl[0].Body != "now readable" {
		t.Fatalf("timeline = %+v", tl)
	}
	if s.Verify().State().Needed {
		t.Fatal("the decrypted re-dispatch left the prompt on")
	}
}
