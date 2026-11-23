package matrix

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"maunium.net/go/mautrix"
)

// The call page's requests become the right Matrix requests: a delayed state
// event (the call membership), a state read, a to-device message, a restart of
// the delay — and the page cannot reach another room.
func TestCallBackendDoesTheMatrixSide(t *testing.T) {
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/state/m.rtc.member/"):
			_, _ = w.Write([]byte(`{"event_id":"$s","delay_id":"d1"}`))
		case strings.HasSuffix(r.URL.Path, "/state") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"type":"m.rtc.member","state_key":"_@a:x_D","sender":"@a:x","content":{"x":1},"event_id":"$1","origin_server_ts":1},
			  {"type":"m.rtc.member","state_key":"_@b:x_E","sender":"@b:x","content":{"x":2},"event_id":"$2","origin_server_ts":2},
			  {"type":"m.room.topic","state_key":"","sender":"@a:x","content":{"topic":"t"},"event_id":"$3","origin_server_ts":3}]`))
		case strings.Contains(r.URL.Path, "/delayed_events/"):
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(r.URL.Path, "/sendToDevice/"):
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(r.URL.Path, "/openid/request_token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600,"matrix_server_name":"x","token_type":"Bearer"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s, _ := testSynced()
	cli, err := mautrix.NewClient(srv.URL, "@me:x", "token")
	if err != nil {
		t.Fatal(err)
	}
	s.cli = cli
	be := s.CallBackend("!r:x")
	ctx := context.Background()

	out, err := be.SendEvent(ctx, json.RawMessage(`{"type":"m.rtc.member","state_key":"_@me:x_D","content":{"a":1},"delay":8000}`))
	if err != nil {
		t.Fatal(err)
	}
	if m := out.(map[string]any); m["delay_id"] != "d1" || m["room_id"] != "!r:x" {
		t.Fatalf("send response: %v", out)
	}
	if !anyHas(&mu, &got, "org.matrix.msc4140.delay=8000") {
		t.Fatalf("the delay was not sent: %v", got)
	}
	if _, err := be.SendEvent(ctx, json.RawMessage(`{"type":"m.rtc.member","state_key":"k","content":{},"room_id":"!other:x"}`)); err == nil {
		t.Fatal("the page sent to another room")
	}

	res, err := be.ReadEvents(ctx, json.RawMessage(`{"type":"m.rtc.member","state_key":true}`))
	if err != nil || len(res.(map[string]any)["events"].([]json.RawMessage)) != 2 {
		t.Fatalf("read all: %v %v", res, err)
	}
	res, _ = be.ReadEvents(ctx, json.RawMessage(`{"type":"m.rtc.member","state_key":"_@a:x_D"}`))
	if n := len(res.(map[string]any)["events"].([]json.RawMessage)); n != 1 {
		t.Fatalf("read one: %d", n)
	}

	if _, err := be.SendToDevice(ctx, json.RawMessage(`{"type":"io.element.call.encryption_keys","encrypted":false,"messages":{"@b:x":{"E":{"keys":{}}}}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := be.SendToDevice(ctx, json.RawMessage(`{"type":"t","messages":{"@b:x":{"*":{}}}}`)); err == nil {
		t.Fatal("to every device should be refused for now")
	}
	if _, err := be.UpdateDelayedEvent(ctx, json.RawMessage(`{"delay_id":"d1","action":"restart"}`)); err != nil {
		t.Fatal(err)
	}
	if o, _ := be.OpenID(ctx); o.(map[string]any)["state"] != "allowed" {
		t.Fatalf("openid: %v", o)
	}
}

func anyHas(mu *sync.Mutex, lines *[]string, sub string) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, l := range *lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
