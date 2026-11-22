package matrix

import (
	"testing"
	"time"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// A message older than what the app holds is found once it is paged in: how
// an encrypted room's old messages become searchable. PageBack pages until
// the start, reporting each page and the end.
func TestPageBackMakesOldMessagesSearchable(t *testing.T) {
	var pages int
	srv := historyServer(t, &pages)
	defer srv.Close()
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	r.encrypted = true
	r.appendEvent(model.Event{ID: "$c", Sender: "@ada:example.org", Kind: model.EventText, Body: "newest", Time: time.UnixMilli(5000)}, "Ada", false)

	q := model.ParseSearch("first")
	if len(s.localHits(q)) != 0 {
		t.Fatal("the old message was found before it was loaded")
	}

	type step struct{ done, more bool }
	steps := make(chan step, 20)
	s.PageBack("!a", 5, func(done, more bool) { steps <- step{done, more} })
	var last step
	for !last.done {
		select {
		case last = <-steps:
		case <-time.After(5 * time.Second):
			t.Fatal("PageBack never finished")
		}
	}
	hits := s.localHits(q)
	if len(hits) != 1 || hits[0].Event != "$a" {
		t.Fatalf("after paging back: %+v", hits)
	}
	// And only that room, when asked for one.
	other := s.roomOf("!b")
	other.appendEvent(model.Event{ID: "$o", Sender: "@ada:example.org", Kind: model.EventText, Body: "first thing elsewhere", Time: time.UnixMilli(6000)}, "Ada", false)
	q.Room = "!a"
	if h := s.localHits(q); len(h) != 1 || h[0].Room != "!a" {
		t.Fatalf("a room-limited search: %+v", h)
	}
	q.Room = ""
	if h := s.localHits(q); len(h) != 2 {
		t.Fatalf("an unlimited search: %+v", h)
	}
}
