package matrix

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

func gapEvent(id string, ts int64) string {
	return fmt.Sprintf(`{"event_id":%q,"type":"m.room.message","sender":"@ada:example.org","origin_server_ts":%d,
	  "content":{"msgtype":"m.text","body":%q}}`, id, ts, id)
}

func gapSource(t *testing.T, h http.HandlerFunc) (*Source, *room) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	for i := 1; i <= 3; i++ { // what the cache held: e1..e3
		r.appendEvent(model.Event{ID: "$e" + strconv.Itoa(i), Sender: "@ada:example.org", Kind: model.EventText,
			Body: "$e" + strconv.Itoa(i), Time: time.UnixMilli(int64(i) * 1000)}, "Ada", false)
	}
	return s, r
}

func limitedSync(s *Source, prev string, ids ...string) {
	room := &mautrix.SyncJoinedRoom{Timeline: mautrix.SyncTimeline{Limited: true, PrevBatch: prev}}
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{"!a": room}}})
	for i, evID := range ids { // the sync's own events, after its listener ran
		c := &event.MessageEventContent{MsgType: event.MsgText, Body: evID}
		s.onMessage(nil, msgEvent(evID, "!a", "@ada:example.org", int64(8000+i*1000), c))
	}
}

func bodies(s *Source) string {
	var out []string
	for _, e := range s.Timeline("!a") {
		out = append(out, e.Body)
	}
	return strings.Join(out, ",")
}

// A limited sync leaves a hole between what the room held and what the sync
// gave: the messages in between are fetched, down to what was held.
func TestLimitedSyncGapIsFilled(t *testing.T) {
	s, r := gapSource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(req.URL.Path, "/messages") || req.URL.Query().Get("from") != "P" {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write([]byte(`{"start":"P","end":"Q","chunk":[` + strings.Join([]string{
			gapEvent("$e7", 7000), gapEvent("$e6", 6000), gapEvent("$e5", 5000), gapEvent("$e4", 4000), gapEvent("$e3", 3000), gapEvent("$e2", 2000),
		}, ",") + `]}`))
	})
	limitedSync(s, "P", "$e8", "$e9")
	waitFor(t, "the gap to fill", func() bool { return len(r.events) == 9 && !r.backBusy })
	if got, want := bodies(s), "$e1,$e2,$e3,$e4,$e5,$e6,$e7,$e8,$e9"; got != want {
		t.Fatalf("timeline = %s, want %s", got, want)
	}
}

// A hole too big to fill drops what was held from before it, so the timeline
// has no hole, and history continues from where the fill stopped.
func TestLimitedSyncBigGapDropsOldEvents(t *testing.T) {
	s, r := gapSource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(req.URL.Path, "/messages") {
			http.NotFound(w, req)
			return
		}
		from := req.URL.Query().Get("from")
		n, err := strconv.Atoi(strings.TrimPrefix(from, "P"))
		if err != nil {
			http.NotFound(w, req)
			return
		}
		ts := int64(7000 - n*100)
		_, _ = fmt.Fprintf(w, `{"start":%q,"end":"P%d","chunk":[%s,%s]}`, from, n+1,
			gapEvent(fmt.Sprintf("$g%da", n), ts), gapEvent(fmt.Sprintf("$g%db", n), ts-50))
	})
	limitedSync(s, "P0", "$e8")
	waitFor(t, "the fill to stop", func() bool { return !r.backBusy && r.back != "" })
	if r.back != "P5" || r.backDone {
		t.Fatalf("history continues from %q (done %v), want P5", r.back, r.backDone)
	}
	got := bodies(s)
	if strings.Contains(got, "$e1") || strings.Contains(got, "$e2") || strings.Contains(got, "$e3") {
		t.Fatalf("what was held from before the hole is still there: %s", got)
	}
	if !strings.Contains(got, "$g0a") || !strings.HasSuffix(got, "$e8") {
		t.Fatalf("timeline = %s", got)
	}
}

// A room with nothing held has no hole: its first sync asks nothing.
func TestLimitedSyncOfANewRoomAsksNothing(t *testing.T) {
	asked := false
	s, _ := testSynced()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { asked = true }))
	defer srv.Close()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	s.cli = cli
	s.onSync(&mautrix.RespSync{Rooms: mautrix.RespSyncRooms{Join: map[id.RoomID]*mautrix.SyncJoinedRoom{
		"!new": {Timeline: mautrix.SyncTimeline{Limited: true, PrevBatch: "P"}}}}})
	time.Sleep(100 * time.Millisecond)
	if asked {
		t.Fatal("asked the server for a hole that is not there")
	}
}
