package matrix

import (
	"testing"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// msgEvent is a message event with its content parsed, as the syncer
// hands them to the handlers.
func msgEvent(evtID, room, sender string, ts int64, c *event.MessageEventContent) *event.Event {
	return &event.Event{
		ID: id.EventID(evtID), RoomID: id.RoomID(room), Sender: id.UserID(sender),
		Timestamp: ts, Type: event.EventMessage,
		Content: event.Content{Parsed: c},
	}
}

func TestTextEvent(t *testing.T) {
	c := &event.MessageEventContent{MsgType: event.MsgText, Body: "hi there"}
	c.Mentions = &event.Mentions{UserIDs: []id.UserID{"@me:example.org"}}
	c.RelatesTo = &event.RelatesTo{InReplyTo: &event.InReplyTo{EventID: id.EventID("$before")}}
	e := textEvent("@me:example.org", msgEvent("$1", "!r", "@ada:example.org", 1000, c))

	if e.Kind != model.EventText || e.Body != "hi there" || e.Sender != "@ada:example.org" {
		t.Fatalf("event = %+v", e)
	}
	if !e.Mentions || e.ReplyTo != "$before" {
		t.Fatalf("mention = %v, reply = %q", e.Mentions, e.ReplyTo)
	}
	if e.Time.UnixMilli() != 1000 {
		t.Fatalf("time = %v", e.Time)
	}
}

func TestTextEventMedia(t *testing.T) {
	c := &event.MessageEventContent{
		MsgType: event.MsgImage, Body: "cat.png", FileName: "cat.png",
		URL:  "mxc://example.org/abc",
		Info: &event.FileInfo{Width: 640, Height: 480, Size: 12345},
	}
	e := textEvent("@me:example.org", msgEvent("$2", "!r", "@ada:example.org", 1, c))
	if e.Kind != model.EventMedia || len(e.Media) != 1 {
		t.Fatalf("event = %+v", e)
	}
	m := e.Media[0]
	if m.URL != "mxc://example.org/abc" || m.W != 640 || m.H != 480 || m.Size != 12345 {
		t.Fatalf("media = %+v", m)
	}
}

func TestMemberLine(t *testing.T) {
	join := &event.Event{
		ID: "$j", RoomID: id.RoomID("!r"), Sender: id.UserID("@ada:example.org"),
		StateKey: ptr("@ada:example.org"), Timestamp: 5, Type: event.StateMember,
		Content: event.Content{Parsed: &event.MemberEventContent{
			Membership: event.MembershipJoin, Displayname: "Ada",
		}},
	}
	e, ok := memberLine(join)
	if !ok || e.Kind != model.EventSystem || e.Body != "Ada joined" || e.Membership != "join" {
		t.Fatalf("line = %+v, %v", e, ok)
	}

	leave := &event.Event{
		ID: "$l", RoomID: id.RoomID("!r"), Sender: id.UserID("@sam:example.org"),
		StateKey: ptr("@sam:example.org"), Timestamp: 6, Type: event.StateMember,
		Content: event.Content{Parsed: &event.MemberEventContent{Membership: event.MembershipLeave}},
	}
	e, ok = memberLine(leave)
	if !ok || e.Body != "sam left" || e.Membership != "leave" { // no displayname: the MXID's local part
		t.Fatalf("line = %+v, %v", e, ok)
	}
}

func TestEditAndReactionOf(t *testing.T) {
	edit := &event.MessageEventContent{
		Body: "* fixed", NewContent: &event.MessageEventContent{Body: "fixed"},
		RelatesTo: &event.RelatesTo{Type: event.RelReplace, EventID: id.EventID("$orig")},
	}
	if target, body, formatted, ok := editOf(edit); !ok || target != "$orig" || body != "fixed" || formatted != "" {
		t.Fatalf("edit = %q %q %q %v", target, body, formatted, ok)
	}
	// An edit that carries HTML keeps it.
	edit.NewContent.Format, edit.NewContent.FormattedBody = event.FormatHTML, "<strong>fixed</strong>"
	if _, _, formatted, ok := editOf(edit); !ok || formatted != "<strong>fixed</strong>" {
		t.Fatalf("edit html = %q %v", formatted, ok)
	}
	plain := &event.MessageEventContent{Body: "hi"}
	if _, _, _, ok := editOf(plain); ok {
		t.Fatal("a plain message parsed as an edit")
	}

	react := &event.MessageEventContent{
		RelatesTo: &event.RelatesTo{Type: event.RelAnnotation, EventID: id.EventID("$orig"), Key: "🔥"},
	}
	if target, key, ok := reactionOf(react); !ok || target != "$orig" || key != "🔥" {
		t.Fatalf("reaction = %q %q %v", target, key, ok)
	}
}

func ptr[T any](v T) *T { return &v }
