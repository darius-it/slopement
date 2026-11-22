package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"maunium.net/go/mautrix"

	"slopement/model"
)

// A room whose pins this device never learned of (a resumed sync does not
// repeat state) asks the server: the list comes, and a pinned message older
// than the timeline is fetched, so the pin bar and panel can show it.
func TestRefreshPinsFetchesWhatIsNotHeld(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/state/m.room.pinned_events"):
			_, _ = w.Write([]byte(`{"pinned":["$old","$held"]}`))
		case strings.HasSuffix(r.URL.Path, "/event/$old"):
			_, _ = w.Write([]byte(`{"event_id":"$old","type":"m.room.message","sender":"@kai:example.org","origin_server_ts":1000,` +
				`"room_id":"!a","content":{"msgtype":"m.text","body":"the wifi password"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:example.org", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	r := s.roomOf("!a")
	r.appendEvent(model.Event{ID: "$held", Sender: "@kai:example.org", Kind: model.EventText, Body: "in the timeline", Time: at(2)}, "kai", false)

	s.RefreshPins("!a")
	waitFor(t, "both pins", func() bool { return len(s.PinnedEvents("!a")) == 2 })
	pins := s.PinnedEvents("!a")
	if pins[0].ID != "$old" || pins[0].Body != "the wifi password" || pins[1].ID != "$held" {
		t.Fatalf("pins = %+v", pins)
	}
	s.RefreshPins("!a") // once per run: no second ask
	if !r.pinsAsked {
		t.Fatal("not marked as asked")
	}
}
