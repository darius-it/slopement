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

// A fake homeserver that serves two pages of a room's history.
func historyServer(t *testing.T, pages *int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_matrix/client/v3/rooms/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/context/"):
			_, _ = w.Write([]byte(`{"start":"t1","end":"t2","events_before":[],"events_after":[],"state":[]}`))
		case strings.HasSuffix(r.URL.Path, "/messages"):
			*pages++
			if r.URL.Query().Get("from") == "t0" {
				_, _ = w.Write([]byte(`{"start":"t0","chunk":[]}`)) // nothing further back
				return
			}
			_, _ = w.Write([]byte(`{"start":"t1","end":"t0","chunk":[
			 {"event_id":"$new","type":"m.room.message","sender":"@kai:example.org","origin_server_ts":3000,
			  "content":{"msgtype":"m.text","body":"* fixed","m.new_content":{"msgtype":"m.text","body":"fixed"},
			             "m.relates_to":{"rel_type":"m.replace","event_id":"$b"}}},
			 {"event_id":"$b","type":"m.room.message","sender":"@kai:example.org","origin_server_ts":2000,
			  "content":{"msgtype":"m.text","body":"fxied"}},
			 {"event_id":"$gone","type":"m.room.message","sender":"@kai:example.org","origin_server_ts":1500,"content":{}},
			 {"event_id":"$mem","type":"m.room.member","state_key":"@kai:example.org","sender":"@kai:example.org","origin_server_ts":1200,
			  "content":{"membership":"join","displayname":"OLD NAME"}},
			 {"event_id":"$a","type":"m.room.message","sender":"@ada:example.org","origin_server_ts":1000,
			  "content":{"msgtype":"m.text","body":"first"}}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Paging older history in puts the events above what the room holds, by
// time; it changes no unread count, preview or last activity; it takes
// edits as edits, leaves redacted events and old state alone, and says when
// there is nothing further back.
func TestLoadOlder(t *testing.T) {
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
	r.appendEvent(model.Event{ID: "$c", Sender: "@ada:example.org", Kind: model.EventText, Body: "newest", Time: time.UnixMilli(5000)}, "Ada", false)
	unread, preview, last := r.unread, r.preview, r.last

	if !s.HasOlder("!a") {
		t.Fatal("a room with events says it has no older ones")
	}
	s.LoadOlder("!a")
	s.LoadOlder("!a") // a page is already on its way: one at a time
	waitFor(t, "the first page", func() bool { return !r.backBusy && len(r.events) > 1 })

	var ids []string
	for _, e := range s.Timeline("!a") {
		ids = append(ids, e.ID+":"+e.Body)
	}
	want := "$a:first,$b:fixed,$c:newest"
	if strings.Join(ids, ",") != want {
		t.Fatalf("timeline = %v, want %s", ids, want)
	}
	if !s.Timeline("!a")[1].Edited {
		t.Fatal("the edit did not mark its message edited")
	}
	if r.unread != unread || r.preview != preview || !r.last.Equal(last) {
		t.Fatalf("history counted as news: unread %d->%d preview %q->%q", unread, r.unread, preview, r.preview)
	}
	if _, known := s.people["@kai:example.org"]; known {
		t.Fatal("an old member event from history changed the people")
	}
	if pages != 1 {
		t.Fatalf("pages fetched = %d, want 1 (the second call must not ask again)", pages)
	}

	// The next page is empty: nothing further back.
	s.LoadOlder("!a")
	waitFor(t, "the end of history", func() bool { return r.backDone })
	if s.HasOlder("!a") {
		t.Fatal("still says there are older messages")
	}
}
