package matrix

import (
	"encoding/json"
	"strings"
	"testing"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func pollEvt(typ event.Type, idv, sender string, ts int64, raw string) *event.Event {
	return &event.Event{
		ID: id.EventID(idv), RoomID: "!a", Sender: id.UserID(sender), Type: typ, Timestamp: ts,
		Content: event.Content{VeryRaw: json.RawMessage(raw)},
	}
}

func vote(target string, answers ...string) string {
	b, _ := json.Marshal(answers)
	return `{"org.matrix.msc3381.poll.response":{"answers":` + string(b) + `},"m.relates_to":{"rel_type":"m.reference","event_id":"` + target + `"}}`
}

const elementPoll = `{"org.matrix.msc3381.poll.start":{"question":{"org.matrix.msc1767.text":"Lunch?","body":"Lunch?","msgtype":"m.text"},` +
	`"kind":"org.matrix.msc3381.poll.disclosed","max_selections":1,` +
	`"answers":[{"id":"x","org.matrix.msc1767.text":"Pizza"},{"id":"y","org.matrix.msc1767.text":"Ramen"}]},"body":"Lunch?"}`

func votersOf(r *room, poll string) map[string][]string {
	out := map[string][]string{}
	for _, a := range r.events[r.indexOf(poll)].Poll.Answers {
		out[a.ID] = a.Voters
	}
	return out
}

// A poll as Element writes it shows as a poll; votes count each person's
// latest only, ones for answers it does not have are spoiled, the poll's
// starter ends it, and nothing after the end counts.
func TestPollsAsElementWritesThem(t *testing.T) {
	s, _ := testSynced()
	start, resp, end := pollStartTypes[0], pollResponseTypes[0], pollEndTypes[0]
	s.onPollStart(nil, pollEvt(start, "$p", "@kai:example.org", 1000, elementPoll))
	r := s.rooms["!a"]
	i := r.indexOf("$p")
	if i < 0 || r.events[i].Kind != model.EventPoll || r.events[i].Poll.Question != "Lunch?" || len(r.events[i].Poll.Answers) != 2 {
		t.Fatalf("the poll: %+v", r.events)
	}
	if got := r.events[i].Poll.Answers[0].Text; got != "Pizza" {
		t.Fatalf("answer text %q", got)
	}

	s.onPollResponse(nil, pollEvt(resp, "$v1", "@ada:example.org", 2000, vote("$p", "x")))
	s.onPollResponse(nil, pollEvt(resp, "$v2", "@bob:example.org", 2100, vote("$p", "y")))
	if v := votersOf(r, "$p"); len(v["x"]) != 1 || v["x"][0] != "@ada:example.org" || len(v["y"]) != 1 {
		t.Fatalf("tally %v", v)
	}
	// Ada changes her mind: her latest counts, not both.
	s.onPollResponse(nil, pollEvt(resp, "$v3", "@ada:example.org", 3000, vote("$p", "y")))
	if v := votersOf(r, "$p"); len(v["x"]) != 0 || len(v["y"]) != 2 {
		t.Fatalf("after a change %v", v)
	}
	// An older echo does not undo a newer vote.
	s.onPollResponse(nil, pollEvt(resp, "$v0", "@ada:example.org", 1500, vote("$p", "x")))
	if v := votersOf(r, "$p"); len(v["x"]) != 0 {
		t.Fatalf("an old vote replaced a newer: %v", v)
	}
	// An answer the poll does not have, and too many answers: spoiled.
	s.onPollResponse(nil, pollEvt(resp, "$v4", "@cy:example.org", 3100, vote("$p", "nope")))
	s.onPollResponse(nil, pollEvt(resp, "$v5", "@di:example.org", 3200, vote("$p", "x", "y")))
	if v := votersOf(r, "$p"); len(v["x"]) != 0 || len(v["y"]) != 2 {
		t.Fatalf("spoiled votes counted: %v", v)
	}

	// Someone else cannot end it.
	s.onPollEnd(nil, pollEvt(end, "$e0", "@bob:example.org", 3500, `{"m.relates_to":{"rel_type":"m.reference","event_id":"$p"}}`))
	if r.events[r.indexOf("$p")].Poll.Ended {
		t.Fatal("a poll was ended by someone who did not start it")
	}
	s.onPollEnd(nil, pollEvt(end, "$e1", "@kai:example.org", 4000, `{"m.relates_to":{"rel_type":"m.reference","event_id":"$p"}}`))
	if !r.events[r.indexOf("$p")].Poll.Ended {
		t.Fatal("the starter's end did not end the poll")
	}
	// A vote after the end does not count.
	s.onPollResponse(nil, pollEvt(resp, "$v6", "@ed:example.org", 5000, vote("$p", "x")))
	if v := votersOf(r, "$p"); len(v["x"]) != 0 {
		t.Fatalf("a vote after the end counted: %v", v)
	}
}

// A poll in the stable m.poll form, with votes that arrive before it (as a
// page of history delivers them, newest first).
func TestStablePollAndVotesBeforeIt(t *testing.T) {
	s, _ := testSynced()
	stable := `{"m.poll":{"question":{"m.text":[{"body":"Which?"}]},"kind":"m.disclosed","max_selections":2,` +
		`"answers":[{"m.id":"a","m.text":[{"body":"One"}]},{"m.id":"b","m.text":[{"body":"Two"}]}]}}`
	response := `{"m.selections":["a","b"],"m.relates_to":{"rel_type":"m.reference","event_id":"$s"}}`
	s.onPollResponse(nil, pollEvt(pollResponseTypes[1], "$v", "@ada:example.org", 2000, response))
	s.onPollStart(nil, pollEvt(pollStartTypes[1], "$s", "@kai:example.org", 1000, stable))
	r := s.rooms["!a"]
	p := r.events[r.indexOf("$s")].Poll
	if p == nil || p.Question != "Which?" || !p.Multiple {
		t.Fatalf("stable poll: %+v", p)
	}
	if v := votersOf(r, "$s"); len(v["a"]) != 1 || len(v["b"]) != 1 {
		t.Fatalf("a vote that came before the poll was lost: %v", v)
	}
}

// Making a poll shows it at once and sends the unstable form every client
// reads; a vote and the end go as responses that name it.
func TestSendingPolls(t *testing.T) {
	fake := &actionServer{}
	s, r := actionSetup(t, fake)
	s.CreatePoll("!a", model.Poll{Question: "Tea or coffee?", Answers: []model.PollAnswer{{Text: "Tea"}, {Text: "Coffee"}}})
	var pending model.Event
	for _, e := range r.events {
		if e.Kind == model.EventPoll {
			pending = e
		}
	}
	if pending.ID == "" || !pending.Pending || pending.Poll.Question != "Tea or coffee?" {
		t.Fatalf("the poll is not shown at once: %+v", pending)
	}
	waitFor(t, "the poll to be sent", func() bool {
		_, ok := fake.has("PUT", "org.matrix.msc3381.poll.start")
		return ok
	})
	body, _ := fake.has("PUT", "org.matrix.msc3381.poll.start")
	for _, want := range []string{`"org.matrix.msc3381.poll.start"`, `"Tea or coffee?"`, `"max_selections":1`, `"id":"a1"`, "poll.disclosed"} {
		if !strings.Contains(body, want) {
			t.Fatalf("poll start lacks %q: %s", want, body)
		}
	}
	waitFor(t, "the poll to settle", func() bool { return r.indexOf("$ok") >= 0 })

	s.Vote("!a", "$ok", 1)
	if v := votersOf(r, "$ok"); len(v["a2"]) != 1 || v["a2"][0] != s.me.ID {
		t.Fatalf("the vote is not shown at once: %v", v)
	}
	waitFor(t, "the vote to be sent", func() bool { _, ok := fake.has("PUT", "poll.response"); return ok })
	vb, _ := fake.has("PUT", "poll.response")
	if !strings.Contains(vb, `"answers":["a2"]`) || !strings.Contains(vb, `"event_id":"$ok"`) || !strings.Contains(vb, "m.reference") {
		t.Fatalf("vote body: %s", vb)
	}
	// A second press on the same answer takes the vote back.
	s.Vote("!a", "$ok", 1)
	if v := votersOf(r, "$ok"); len(v["a2"]) != 0 {
		t.Fatalf("the vote was not taken back: %v", v)
	}

	s.EndPoll("!a", "$ok")
	if !r.events[r.indexOf("$ok")].Poll.Ended {
		t.Fatal("not ended at once")
	}
	waitFor(t, "the end to be sent", func() bool { _, ok := fake.has("PUT", "poll.end"); return ok })
	// Voting on an ended poll does nothing.
	s.Vote("!a", "$ok", 0)
	if v := votersOf(r, "$ok"); len(v["a1"]) != 0 {
		t.Fatal("voted on an ended poll")
	}
}

// The votes are kept in the cache: a resumed sync never repeats them.
func TestPollStateSurvivesTheCache(t *testing.T) {
	s, _ := testSynced()
	s.onPollStart(nil, pollEvt(pollStartTypes[0], "$p", "@kai:example.org", 1000, elementPoll))
	s.onPollResponse(nil, pollEvt(pollResponseTypes[0], "$v", "@ada:example.org", 2000, vote("$p", "x")))
	rec := s.rooms["!a"].record()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var back roomRecord
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	r := back.room()
	r.refreshPoll("$p")
	if v := votersOf(r, "$p"); len(v["x"]) != 1 {
		t.Fatalf("the votes did not survive: %v", v)
	}
}
