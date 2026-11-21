package matrix

import (
	"testing"
	"time"

	"slopement/model"
)

func at(minutes int64) time.Time { return time.UnixMilli(minutes * 60_000) }

func TestRoomAppend(t *testing.T) {
	r := &room{id: "!r"}

	// Someone else's message counts unread, a mention highlighted; the
	// preview says who and what.
	r.appendEvent(model.Event{ID: "$1", Time: at(1), Kind: model.EventText, Body: "first"}, "Ada", false)
	r.events[0].Mentions = true // set after the fact: counting reads it
	_ = r.events
	r.unread, r.mentions = 0, 0
	r.events = r.events[:0]

	r.appendEvent(model.Event{ID: "$1", Time: at(1), Kind: model.EventText, Body: "first", Mentions: true}, "Ada", false)
	if r.unread != 1 || r.mentions != 1 {
		t.Fatalf("unread = %d, mentions = %d", r.unread, r.mentions)
	}
	if r.preview != "Ada: first" {
		t.Fatalf("preview = %q", r.preview)
	}

	// The user's own message: "You", and no unread.
	r.appendEvent(model.Event{ID: "$2", Time: at(2), Kind: model.EventText, Body: "done"}, "You", true)
	if r.unread != 1 || r.preview != "You: done" || !r.last.Equal(at(2)) {
		t.Fatalf("own message: unread %d, preview %q", r.unread, r.preview)
	}

	// A system line is quiet: no unread, no preview change.
	r.appendEvent(model.Event{ID: "$3", Time: at(3), Kind: model.EventSystem, Body: "Ada left"}, "Ada", false)
	if r.unread != 1 || r.preview != "You: done" {
		t.Fatalf("system line: unread %d, preview %q", r.unread, r.preview)
	}

	// The same event ID again (a late decrypt re-dispatched) replaces the
	// one the room has, in its place, without counting unread twice.
	r.appendEvent(model.Event{ID: "$2", Time: at(2), Kind: model.EventText, Body: "done, edited"}, "You", true)
	if len(r.events) != 3 || r.unread != 1 {
		t.Fatalf("replaced: %d events, unread %d", len(r.events), r.unread)
	}
	if r.events[1].Body != "done, edited" {
		t.Fatalf("the tile was not replaced in place: %+v", r.events[1])
	}

	// The timeline caps at timelineCap, keeping the newest.
	for i := 0; i < timelineCap+20; i++ {
		r.appendEvent(model.Event{ID: "$m" + time.Duration(i).String(), Time: at(int64(i)), Kind: model.EventText, Body: "m"}, "X", false)
	}
	if len(r.events) != timelineCap {
		t.Fatalf("timeline = %d events, want %d", len(r.events), timelineCap)
	}
	if r.events[0].Time.Before(at(19)) {
		t.Fatalf("the oldest kept is %v", r.events[0].Time)
	}
}

func TestRoomEditAndReactions(t *testing.T) {
	r := &room{id: "!r"}
	r.appendEvent(model.Event{ID: "$1", Time: at(1), Kind: model.EventText, Body: "tyop"}, "Ada", false)

	r.editEvent("$1", "typo", 10)
	if r.events[0].Body != "typo" || !r.events[0].Edited {
		t.Fatalf("edit = %+v", r.events[0])
	}
	r.editEvent("$nope", "nothing", 11)
	if r.events[0].Body != "typo" {
		t.Fatal("an edit of a missing event rewrote something")
	}

	// Two keys, one mine, one taken back.
	r.react("$1", "@ada:example.org", "🔥")
	r.react("$1", "@me:example.org", "🔥")
	r.react("$1", "@sam:example.org", "❤️")
	r.react("$1", "@sam:example.org", "")
	out := r.timeline("@me:example.org")
	if len(out[0].Reactions) != 1 { // the ❤️ was taken back; only 🔥 stays
		t.Fatalf("reactions = %+v", out[0].Reactions)
	}
	byKey := map[string]model.Reaction{}
	for _, react := range out[0].Reactions {
		byKey[react.Key] = react
	}
	fire := byKey["🔥"]
	if fire.Count != 2 || !fire.Mine {
		t.Fatalf("🔥 = %+v", fire)
	}
	if heart := byKey["❤️"]; heart.Count != 0 { // taken back, so gone
		t.Fatalf("❤️ = %+v", heart)
	}
}
