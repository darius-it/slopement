package matrix

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func liveMessage(roomID, evID, sender, body string, age time.Duration, extra func(*event.MessageEventContent)) *event.Event {
	c := &event.MessageEventContent{MsgType: event.MsgText, Body: body}
	if extra != nil {
		extra(c)
	}
	raw, _ := json.Marshal(c)
	return &event.Event{
		ID: id.EventID(evID), RoomID: id.RoomID(roomID), Sender: id.UserID(sender),
		Type: event.EventMessage, Timestamp: time.Now().Add(-age).UnixMilli(),
		Content: event.Content{VeryRaw: raw, Parsed: c},
	}
}

// Only what is news and addressed to the user notifies: a person's message
// in a chat, or a mention elsewhere — never one's own, an old one, one in a
// muted room, one told of already, or an edit's.
func TestWhatNotifies(t *testing.T) {
	s, _ := testSynced()
	var got []model.Notice
	s.OnNotify(func(n model.Notice) { got = append(got, n) })
	dm := s.roomOf("!dm")
	dm.dm, dm.partner, dm.name = true, "@kai:example.org", "Kai"
	grp := s.roomOf("!grp")
	grp.name = "Lunch club"
	s.people["@kai:example.org"] = &model.Person{ID: "@kai:example.org", Name: "Kai"}
	ctx := context.Background()

	s.onMessage(ctx, liveMessage("!dm", "$1", "@kai:example.org", "hello there", 0, nil))
	if len(got) != 1 || got[0].Title != "Kai" || got[0].Body != "hello there" || !got[0].DM || got[0].Room != "!dm" {
		t.Fatalf("a DM: %+v", got)
	}
	// Not a mention, in a group: no.
	s.onMessage(ctx, liveMessage("!grp", "$2", "@kai:example.org", "anyone for lunch?", 0, nil))
	if len(got) != 1 {
		t.Fatalf("a group message with no mention notified: %+v", got)
	}
	// A mention: yes, with who said it.
	s.onMessage(ctx, liveMessage("!grp", "$3", "@kai:example.org", "hey @me", 0, func(c *event.MessageEventContent) {
		c.Mentions = &event.Mentions{UserIDs: []id.UserID{"@me:example.org"}}
	}))
	if len(got) != 2 || got[1].Title != "Lunch club" || got[1].Body != "Kai: hey @me" || !got[1].Mention {
		t.Fatalf("a mention: %+v", got)
	}
	// Told of once: the same event again (a late decrypt, an echo).
	s.onMessage(ctx, liveMessage("!grp", "$3", "@kai:example.org", "hey @me", 0, func(c *event.MessageEventContent) {
		c.Mentions = &event.Mentions{UserIDs: []id.UserID{"@me:example.org"}}
	}))
	if len(got) != 2 {
		t.Fatalf("told of twice: %d", len(got))
	}
	// Old, own, muted.
	s.onMessage(ctx, liveMessage("!dm", "$4", "@kai:example.org", "from last week", 3*time.Hour, nil))
	s.onMessage(ctx, liveMessage("!dm", "$5", "@me:example.org", "my own", 0, nil))
	dm.muted = true
	s.onMessage(ctx, liveMessage("!dm", "$6", "@kai:example.org", "in a muted room", 0, nil))
	if len(got) != 2 {
		t.Fatalf("what should have been quiet notified: %+v", got[2:])
	}
	// History pages are never news.
	dm.muted = false
	h := liveMessage("!dm", "$7", "@kai:example.org", "older", 0, nil)
	h.Mautrix.EventSource = sourceHistory
	s.onMessage(ctx, h)
	if len(got) != 2 {
		t.Fatal("a history page notified")
	}
}
