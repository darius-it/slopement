package matrix

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// The rail follows the order the user dragged it into (the account's, on
// every session); communities not in it come after, by name.
func TestCommunityOrder(t *testing.T) {
	fake := &actionServer{}
	s, _ := actionSetup(t, fake)
	for id, name := range map[string]string{"!a": "Alpha", "!b": "Beta", "!c": "Gamma", "!d": "Delta"} {
		r := s.roomOf(id)
		r.name, r.space = name, true
	}
	names := func() string {
		var out []string
		for _, c := range s.Communities() {
			out = append(out, c.Name)
		}
		return strings.Join(out, ",")
	}
	if got := names(); got != "Alpha,Beta,Delta,Gamma" {
		t.Fatalf("by name: %s", got)
	}
	s.SetCommunityOrder([]string{"!c", "!a"})
	if got := names(); got != "Gamma,Alpha,Beta,Delta" {
		t.Fatalf("after dragging: %s (listed first, the rest by name)", got)
	}
	waitFor(t, "the order in account data", func() bool {
		b, ok := fake.has("PUT", "/account_data/com.slopement.rail_order")
		return ok && strings.Contains(b, `"order":["!c","!a"]`)
	})
	// Another session's order replaces it.
	evt := &event.Event{Content: event.Content{VeryRaw: json.RawMessage(`{"order":["!d","!b"]}`)}}
	s.onRailOrder(nil, evt)
	if got := names(); got != "Delta,Beta,Alpha,Gamma" {
		t.Fatalf("the account's order: %s", got)
	}
}

// A community's hierarchy lists the rooms it offers, those the user is not in
// among them, sub-spaces marked and the space itself left out.
func TestSpaceRooms(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rooms":[` +
			`{"room_id":"!space:x","name":"The space","room_type":"m.space","num_joined_members":50,"children_state":[]},` +
			`{"room_id":"!in:x","name":"Already in","num_joined_members":10,"children_state":[]},` +
			`{"room_id":"!out:x","name":"To join","canonical_alias":"#join:x","topic":"come in","num_joined_members":7,"children_state":[]},` +
			`{"room_id":"!sub:x","name":"Sub","room_type":"m.space","num_joined_members":3,"children_state":[]}]}`))
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, _ := mautrix.NewClient(srv.URL, "@me:example.org", "t")
	s.cli = cli
	s.roomOf("!in:x")

	got := make(chan []model.PublicRoom, 1)
	s.SpaceRooms("!space:x", func(r []model.PublicRoom, err string) {
		if err != "" {
			t.Error(err)
		}
		got <- r
	})
	rooms := <-got
	if len(rooms) != 3 {
		t.Fatalf("rooms = %+v (the space itself is left out)", rooms)
	}
	by := map[string]model.PublicRoom{}
	for _, r := range rooms {
		by[r.ID] = r
	}
	if !by["!in:x"].Joined || by["!out:x"].Joined || !by["!sub:x"].Space {
		t.Fatalf("joined/space flags: %+v", by)
	}
	if by["!out:x"].Alias != "#join:x" || by["!out:x"].Topic != "come in" || by["!out:x"].Members != 7 {
		t.Fatalf("details: %+v", by["!out:x"])
	}
	if !strings.Contains(query, "max_depth=1") {
		t.Fatalf("only one level asked for: %q", query)
	}
}
