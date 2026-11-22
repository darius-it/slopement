package matrix

import (
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// A thread reply belongs to its root; the reply it points to for clients
// without threads is a fallback, not a reply — unless the sender did reply
// to something in the thread.
func TestThreadRepliesAreRead(t *testing.T) {
	fallback := &event.MessageEventContent{MsgType: event.MsgText, Body: "in the thread",
		RelatesTo: (&event.RelatesTo{}).SetThread("$root", "$latest")}
	e := textEvent("@me:x", msgEvent("$r1", "!a", "@kai:x", 1, fallback))
	if e.ThreadRoot != "$root" || e.ReplyTo != "" {
		t.Fatalf("a thread reply: root %q replyTo %q", e.ThreadRoot, e.ReplyTo)
	}
	real := &event.MessageEventContent{MsgType: event.MsgText, Body: "to you",
		RelatesTo: (&event.RelatesTo{}).SetThread("$root", "").SetReplyTo("$someone")}
	e = textEvent("@me:x", msgEvent("$r2", "!a", "@kai:x", 1, real))
	if e.ThreadRoot != "$root" || e.ReplyTo != "$someone" {
		t.Fatalf("a reply within a thread: root %q replyTo %q", e.ThreadRoot, e.ReplyTo)
	}
	plain := &event.MessageEventContent{MsgType: event.MsgText, Body: "ordinary"}
	if e := textEvent("@me:x", msgEvent("$p", "!a", "@kai:x", 1, plain)); e.ThreadRoot != "" {
		t.Fatal("a plain message is in a thread")
	}
}

// A root says how many replies its thread has, the latest and who wrote.
func TestThreadSummary(t *testing.T) {
	r := &room{id: "!a"}
	r.appendEvent(model.Event{ID: "$root", Sender: "@ada:x", Kind: model.EventText, Body: "root", Time: at(1)}, "ada", false)
	r.appendEvent(model.Event{ID: "$1", Sender: "@kai:x", Kind: model.EventText, Body: "one", Time: at(2), ThreadRoot: "$root"}, "kai", false)
	r.appendEvent(model.Event{ID: "$2", Sender: "@ada:x", Kind: model.EventText, Body: "two", Time: at(4), ThreadRoot: "$root"}, "ada", false)
	r.appendEvent(model.Event{ID: "$3", Sender: "@kai:x", Kind: model.EventText, Body: "three", Time: at(3), ThreadRoot: "$root"}, "kai", false)
	r.appendEvent(model.Event{ID: "$other", Sender: "@kai:x", Kind: model.EventText, Body: "no thread", Time: at(5)}, "kai", false)
	tl := r.timeline("@me:x")
	var root model.Event
	for _, e := range tl {
		if e.ID == "$root" {
			root = e
		}
		if e.ID != "$root" && e.Thread != nil {
			t.Fatalf("%s has a thread summary", e.ID)
		}
	}
	th := root.Thread
	if th == nil || th.Replies != 3 || th.LastSender != "@ada:x" || !th.Last.Equal(at(4)) {
		t.Fatalf("summary %+v", th)
	}
	if len(th.Participants) != 2 || th.Participants[0] != "@kai:x" {
		t.Fatalf("participants %v (the first to reply first)", th.Participants)
	}
}

// A reply in a thread goes out related to the root as a thread, with the
// thread's latest message as the fallback reply, and shows at once in the
// thread.
func TestSendInThread(t *testing.T) {
	fake := &actionServer{}
	s, r := actionSetup(t, fake)
	r.appendEvent(model.Event{ID: "$root", Sender: "@kai:example.org", Kind: model.EventText, Body: "root", Time: at(2)}, "kai", false)
	r.appendEvent(model.Event{ID: "$t1", Sender: "@kai:example.org", Kind: model.EventText, Body: "first reply", Time: at(3), ThreadRoot: "$root"}, "kai", false)

	s.SendInThread("!a", "$root", "my **reply**")
	var pending model.Event
	for _, e := range r.events {
		if e.Pending {
			pending = e
		}
	}
	if pending.ThreadRoot != "$root" || !strings.Contains(pending.Body, "reply") {
		t.Fatalf("not shown at once in the thread: %+v", pending)
	}
	waitFor(t, "the reply to be sent", func() bool { _, ok := fake.has("PUT", "/send/m.room.message/"); return ok })
	body, _ := fake.has("PUT", "/send/m.room.message/")
	for _, want := range []string{`"rel_type":"m.thread"`, `"event_id":"$root"`, `"is_falling_back":true`, `"$t1"`, "\\u003cstrong\\u003ereply"} {
		if !strings.Contains(body, want) {
			t.Fatalf("thread reply lacks %q: %s", want, body)
		}
	}
	_ = id.EventID("")
	_ = time.Now
}
