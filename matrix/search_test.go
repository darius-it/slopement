package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix"

	"slopement/model"
)

func searchFixture(t *testing.T) *Source {
	t.Helper()
	s, _ := testSynced()
	s.people["@kai:example.org"] = &model.Person{ID: "@kai:example.org", Name: "Kai Fireborn"}
	s.people["@ada:example.org"] = &model.Person{ID: "@ada:example.org", Name: "Ada"}
	design := s.roomOf("!design")
	design.name, design.alias, design.encrypted = "Design crit", "#design:example.org", true
	chat := s.roomOf("!chat")
	chat.name = "Weekend plans"
	day := func(d int) time.Time { return time.Date(2026, 3, d, 12, 0, 0, 0, time.Local) }
	add := func(r *room, e model.Event) {
		e.Kind = model.EventText
		r.events = append(r.events, e)
	}
	add(design, model.Event{ID: "$1", Sender: "@kai:example.org", Time: day(1), Body: "The lunch menu is ready"})
	add(design, model.Event{ID: "$2", Sender: "@ada:example.org", Time: day(5), Body: "see https://example.com/lunch for lunch plans", Mentions: true})
	add(design, model.Event{ID: "$3", Sender: "@kai:example.org", Time: day(9), Body: "photo", Kind: model.EventMedia,
		Media: []model.Media{{Kind: model.MediaImage, Name: "lunch.png"}}})
	add(chat, model.Event{ID: "$4", Sender: "@ada:example.org", Time: day(3), Body: "Lunch on Sunday?", Formatted: "<strong>Lunch</strong> on Sunday?"})
	add(chat, model.Event{ID: "$5", Sender: "@ada:example.org", Time: day(4), Body: "", Kind: model.EventUndecryptable})
	add(chat, model.Event{ID: localPrefix + "pending", Sender: "@me:example.org", Time: day(6), Body: "lunch pending"})
	return s
}

func ids(h []model.SearchHit) string {
	var out []string
	for _, x := range h {
		out = append(out, x.Event)
	}
	return strings.Join(out, ",")
}

// Local search covers every room the app holds — encrypted ones too — by
// every word, newest first; the filters narrow it; messages still being
// sent and ones that could not be read are never hits.
func TestLocalSearch(t *testing.T) {
	s := searchFixture(t)
	q := func(text string) string { return ids(s.localHits(model.ParseSearch(text))) }

	if got := q("lunch"); got != "$3,$2,$4,$1" {
		t.Fatalf("lunch = %s (every message that says it, newest first; the file's name counts)", got)
	}
	if got := q("LUNCH plans"); got != "$2" {
		t.Fatalf("two words = %s", got)
	}
	if got := q("lunch in:design"); got != "$3,$2,$1" {
		t.Fatalf("in: = %s", got)
	}
	if got := q("lunch in:#design:example"); got != "$3,$2,$1" {
		t.Fatalf("in: by address = %s", got)
	}
	if got := q("lunch from:fire"); got != "$3,$1" {
		t.Fatalf("from: a name = %s", got)
	}
	if got := q("lunch from:@ada"); got != "$2,$4" {
		t.Fatalf("from: an address = %s", got)
	}
	if got := q("has:image"); got != "$3" {
		t.Fatalf("has:image = %s", got)
	}
	if got := q("lunch has:link"); got != "$2" {
		t.Fatalf("has:link = %s", got)
	}
	if got := q("lunch after:2026-03-03 before:2026-03-09"); got != "$2,$4" {
		t.Fatalf("dates = %s (after is inclusive, before is not)", got)
	}
	if got := q("lunch mentions:me"); got != "$2" {
		t.Fatalf("mentions:me = %s", got)
	}
	if got := q("mentions:me"); got != "$2" {
		t.Fatalf("filters alone = %s", got)
	}
	if got := q("nothing like this"); got != "" {
		t.Fatalf("no match = %s", got)
	}
	// A formatted message is searched as it reads, not as its HTML.
	if got := q("strong"); got != "" {
		t.Fatalf("the HTML was searched: %s", got)
	}
}

// The server's hits for the rooms that are not encrypted join the local ones
// — the same message once — and a server that cannot be reached leaves the
// local hits standing, with a quiet line.
func TestSearchAsksTheServer(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		asked = string(b[:n])
		if strings.Contains(asked, "broken") {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"errcode":"M_UNKNOWN","error":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"search_categories":{"room_events":{"count":2,"results":[` +
			`{"rank":1,"result":{"event_id":"$old","room_id":"!chat","sender":"@ada:example.org","type":"m.room.message","origin_server_ts":1000,"content":{"msgtype":"m.text","body":"an old lunch from long ago"}}},` +
			`{"rank":1,"result":{"event_id":"$4","room_id":"!chat","sender":"@ada:example.org","type":"m.room.message","origin_server_ts":2000,"content":{"msgtype":"m.text","body":"Lunch on Sunday?"}}}]}}}`))
	}))
	defer srv.Close()
	s := searchFixture(t)
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "t")
	s.cli = cli

	type call struct {
		hits  []model.SearchHit
		final bool
		err   string
	}
	got := make(chan call, 4)
	s.Search(model.ParseSearch("lunch"), func(h []model.SearchHit, final bool, err string) { got <- call{h, final, err} })
	first := <-got
	if first.final || ids(first.hits) != "$3,$2,$4,$1" {
		t.Fatalf("the local answer first: final %v hits %s", first.final, ids(first.hits))
	}
	second := <-got
	if !second.final || second.err != "" {
		t.Fatalf("the merged answer: %+v", second)
	}
	if !strings.Contains(asked, "!chat") || strings.Contains(asked, "!design") {
		t.Fatalf("the server was asked about the wrong rooms (only the unencrypted): %s", asked)
	}
	// $4 is held and the server has it too: once. $old is new to the app.
	if ids(second.hits) != "$3,$2,$4,$1,$old" {
		t.Fatalf("merged = %s", ids(second.hits))
	}

	// Only filters, no words: nothing the server could look up.
	s.Search(model.ParseSearch("has:image"), func(h []model.SearchHit, final bool, err string) { got <- call{h, final, err} })
	if c := <-got; !c.final || ids(c.hits) != "$3" {
		t.Fatalf("filters alone: %+v", c)
	}

	// A server that fails: the local hits stand.
	s.Search(model.ParseSearch("broken lunch"), func(h []model.SearchHit, final bool, err string) { got <- call{h, final, err} })
	<-got
	c := <-got
	if !c.final || c.err == "" {
		t.Fatalf("no word on the server failing: %+v", c)
	}
}

func TestEmptySearchIsNothing(t *testing.T) {
	s := searchFixture(t)
	called := false
	s.Search(model.ParseSearch("  "), func(h []model.SearchHit, final bool, err string) {
		called = true
		if len(h) != 0 || !final {
			t.Fatalf("empty search: %v %v", h, final)
		}
	})
	if !called {
		t.Fatal("no answer")
	}
}
