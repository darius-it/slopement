package matrix

import (
	"strings"
	"testing"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func reactionEvent(idv, room, sender, target, key, txn string) *event.Event {
	return &event.Event{
		ID: id.EventID(idv), RoomID: id.RoomID(room), Sender: id.UserID(sender), Type: event.EventReaction,
		Unsigned: event.Unsigned{TransactionID: txn},
		Content: event.Content{Parsed: &event.ReactionEventContent{
			RelatesTo: event.RelatesTo{Type: event.RelAnnotation, EventID: id.EventID(target), Key: key}}},
	}
}

func reactionsOf(s *Source, eventID string) map[string]model.Reaction {
	out := map[string]model.Reaction{}
	for _, e := range s.Timeline("!a") {
		if e.ID == eventID {
			for _, r := range e.Reactions {
				out[r.Key] = r
			}
		}
	}
	return out
}

// Reactions made by others show on their message; taking one back (the
// reaction event redacted) removes just that one.
func TestIncomingReactionsAndRedaction(t *testing.T) {
	s, _ := testSynced()
	r := s.roomOf("!a")
	r.appendEvent(model.Event{ID: "$m", Sender: "@ada:example.org", Kind: model.EventText, Body: "hi", Time: at(1)}, "Ada", false)

	s.onReaction(nil, reactionEvent("$r1", "!a", "@kai:example.org", "$m", "🔥", ""))
	s.onReaction(nil, reactionEvent("$r2", "!a", "@sam:example.org", "$m", "🔥", ""))
	s.onReaction(nil, reactionEvent("$r3", "!a", "@sam:example.org", "$m", "👍", ""))
	s.onReaction(nil, reactionEvent("$r3", "!a", "@sam:example.org", "$m", "👍", "")) // the same event again
	got := reactionsOf(s, "$m")
	if got["🔥"].Count != 2 || got["👍"].Count != 1 {
		t.Fatalf("reactions = %+v", got)
	}
	// In the order they were made, every time.
	for i := 0; i < 20; i++ {
		if rs := s.Timeline("!a")[0].Reactions; len(rs) != 2 || rs[0].Key != "🔥" || rs[1].Key != "👍" {
			t.Fatalf("order = %+v", rs)
		}
	}

	s.onRedaction(nil, &event.Event{RoomID: "!a", Type: event.EventRedaction, Redacts: "$r2"})
	if got := reactionsOf(s, "$m"); got["🔥"].Count != 1 || got["👍"].Count != 1 {
		t.Fatalf("after one was taken back: %+v", got)
	}
	// A reaction on a message that is deleted goes with it.
	s.onRedaction(nil, &event.Event{RoomID: "!a", Type: event.EventRedaction, Redacts: "$m"})
	if len(r.reactions["$m"]) != 0 {
		t.Fatal("reactions outlived their message")
	}
}

// The user's reaction shows at once, once; the server's echo (whichever of
// it and the send's answer comes first) does not double it; reacting again
// with the same key takes it back — the reaction event is redacted.
func TestMyReactionShowsAtOnceAndToggles(t *testing.T) {
	fake := &recordServer{}
	s, r, done := actionSource(t, fake)
	defer done()

	s.React("!a", "$other", "🔥")
	if got := reactionsOf(s, "$other"); got["🔥"].Count != 1 || !got["🔥"].Mine {
		t.Fatalf("not shown at once: %+v", got)
	}
	waitFor(t, "the reaction to be sent", func() bool { _, ok := fake.find("/send/m.reaction/"); return ok })
	body, _ := fake.find("/send/m.reaction/")
	if !strings.Contains(body, `"rel_type":"m.annotation"`) || !strings.Contains(body, `"key":"🔥"`) || !strings.Contains(body, `"$other"`) {
		t.Fatalf("reaction event = %s", body)
	}
	waitFor(t, "the answer to settle it", func() bool { return r.hasReaction("$ok") })
	if got := reactionsOf(s, "$other"); got["🔥"].Count != 1 {
		t.Fatalf("after the answer: %+v", got)
	}

	// The echo of it, as the sync brings it: still one.
	s.onReaction(nil, reactionEvent("$ok", "!a", "@me:example.org", "$other", "🔥", "whatever"))
	if got := reactionsOf(s, "$other"); got["🔥"].Count != 1 {
		t.Fatalf("the echo doubled it: %+v", got)
	}

	s.React("!a", "$other", "🔥") // again: take it back
	if got := reactionsOf(s, "$other"); got["🔥"].Count != 0 {
		t.Fatalf("not taken back at once: %+v", got)
	}
	waitFor(t, "the redaction", func() bool { _, ok := fake.find("/redact/$ok"); return ok })
}

// If the echo comes before the send's answer, there is still just one; and
// if the user took it back while it was being sent, it does not stay.
func TestReactionEchoBeforeAnswerAndTakenBackWhileSending(t *testing.T) {
	fake := &recordServer{}
	s, r, done := actionSource(t, fake)
	defer done()

	s.React("!a", "$other", "👍")
	var txn string
	for t2 := range r.reactTxn {
		txn = t2
	}
	if txn == "" {
		t.Fatal("no reaction in flight")
	}
	// The echo (the real event, carrying our transaction) comes first.
	s.onReaction(nil, reactionEvent("$ok", "!a", "@me:example.org", "$other", "👍", txn))
	waitFor(t, "the send to be answered", func() bool { return len(r.reactTxn) == 0 })
	if got := reactionsOf(s, "$other"); got["👍"].Count != 1 {
		t.Fatalf("echo first: %+v", got)
	}

	// Another: reacted, then taken back before the answer comes.
	fake2 := &recordServer{}
	s2, r2, done2 := actionSource(t, fake2)
	defer done2()
	s2.React("!a", "$other", "🎉")
	s2.React("!a", "$other", "🎉") // while the first is still being sent
	if got := reactionsOf(s2, "$other"); got["🎉"].Count != 0 {
		t.Fatalf("not taken back: %+v", got)
	}
	waitFor(t, "the real event to be redacted", func() bool { _, ok := fake2.find("/redact/"); return ok })
	if r2.hasReaction("$ok") {
		t.Fatal("a reaction taken back came back with the server's answer")
	}
}

// A refused reaction is undone, and the room says so.
func TestRefusedReactionIsUndone(t *testing.T) {
	fake := &recordServer{refuse: true}
	s, r, done := actionSource(t, fake)
	defer done()
	s.React("!a", "$other", "🔥")
	waitFor(t, "the notice", func() bool {
		for _, e := range r.events {
			if e.Kind == model.EventSystem && strings.Contains(e.Body, "Could not react") {
				return true
			}
		}
		return false
	})
	if got := reactionsOf(s, "$other"); got["🔥"].Count != 0 {
		t.Fatalf("a refused reaction stayed: %+v", got)
	}
}

// The echo of the user's own reaction, encrypted, is no lock tile.
func TestEncryptedReactionEchoMakesNoTile(t *testing.T) {
	fake := &recordServer{}
	s, r, done := actionSource(t, fake)
	defer done()
	s.React("!a", "$other", "🔥")
	var txn string
	for t2 := range r.reactTxn {
		txn = t2
	}
	before := len(r.events)
	s.onEncrypted(nil, &event.Event{ID: "$enc", RoomID: "!a", Sender: "@me:example.org", Type: event.EventEncrypted,
		Timestamp: 1, Unsigned: event.Unsigned{TransactionID: txn},
		Content: event.Content{Parsed: &event.EncryptedEventContent{SessionID: "s", Algorithm: id.AlgorithmMegolmV1}}})
	if len(r.events) != before || s.encrypted != 0 {
		t.Fatalf("an echo of the user's reaction became a tile: events %d -> %d", before, len(r.events))
	}
}
