package matrix

import (
	"testing"

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
