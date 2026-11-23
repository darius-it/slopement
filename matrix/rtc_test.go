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

// The media server's token is asked of the call service with an OpenID token
// and who the member is; the delayed leave is handed over with the delay's
// length; the transports come from the homeserver, else its .well-known.
func TestCallBackendTalksToTheCallService(t *testing.T) {
	var mu sync.Mutex
	var bodies = map[string]map[string]any{}
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		bodies[r.URL.Path] = b
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/get_token" {
			_, _ = w.Write([]byte(`{"url":"wss://sfu.example","jwt":"JWT"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer svc.Close()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/rtc/transports"):
			_, _ = w.Write([]byte(`{"rtc_transports":[{"type":"livekit","livekit_service_url":"` + svc.URL + `"}]}`))
		case strings.Contains(r.URL.Path, "/openid/request_token"):
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600,"matrix_server_name":"x","token_type":"Bearer"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer hs.Close()
	s, _ := testSynced()
	s.me.ID = "@me:x"
	cli, _ := mautrix.NewClient(hs.URL, "@me:x", "token")
	cli.DeviceID = "DEV"
	s.cli = cli
	be := s.CallBackend("!r:x")
	ctx := context.Background()

	tr, err := be.RTCTransports(ctx)
	if err != nil || len(tr.(map[string]any)["rtc_transports"].([]map[string]any)) != 1 {
		t.Fatalf("transports: %v %v", tr, err)
	}
	req := `{"url":"` + svc.URL + `","room_id":"!r:x","slot_id":"s1","member_id":"m1","server_name":"x"}`
	out, err := be.LiveKitToken(ctx, json.RawMessage(req))
	if err != nil || out.(map[string]any)["jwt"] != "JWT" {
		t.Fatalf("token: %v %v", out, err)
	}
	b := bodies["/get_token"]
	mem, _ := b["member"].(map[string]any)
	oid, _ := b["openid_token"].(map[string]any)
	if b["room_id"] != "!r:x" || b["slot_id"] != "s1" || mem["id"] != "m1" || mem["claimed_user_id"] != "@me:x" ||
		mem["claimed_device_id"] != "DEV" || oid["access_token"] != "tok" {
		t.Fatalf("the token request is %v", b)
	}
	if _, err := be.LiveKitToken(ctx, json.RawMessage(`{"url":"`+svc.URL+`","room_id":"!other:x"}`)); err == nil {
		t.Fatal("a token for another room")
	}

	s.callDelays = map[string]int64{"d1": 8000}
	if _, err := be.LiveKitDelayedLeave(ctx, json.RawMessage(`{"url":"`+svc.URL+`","room_id":"!r:x","slot_id":"s1","member_id":"m1","delay_id":"d1"}`)); err != nil {
		t.Fatal(err)
	}
	if d := bodies["/delegate_delayed_leave"]; d["delay_id"] != "d1" || d["delay_timeout"] != float64(8000) {
		t.Fatalf("delegate body %v", d)
	}
}

// The homeserver's own MatrixRTC endpoints are asked first, with the page's
// request as it is; the call service is not contacted then.
func TestCallTokenComesFromTheHomeserverWhenItCan(t *testing.T) {
	var mu sync.Mutex
	got := map[string]string{}
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got[r.URL.Path] = string(body) + "|" + r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/rtc/livekit/get_token"):
			_, _ = w.Write([]byte(`{"jwt":"FROM-HS"}`))
		case strings.HasSuffix(r.URL.Path, "/rtc/livekit/delegate_delayed_leave"):
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer hs.Close()
	s, _ := testSynced()
	s.me.ID = "@me:x"
	cli, _ := mautrix.NewClient(hs.URL, "@me:x", "tok")
	s.cli = cli
	be := s.CallBackend("!r:x")
	req := `{"server_name":"x","url":"wss://sfu.example/livekit","room_id":"!r:x","slot_id":"s","member_id":"m","delay_id":"d"}`

	out, err := be.LiveKitToken(context.Background(), json.RawMessage(req))
	if err != nil || out.(map[string]any)["jwt"] != "FROM-HS" {
		t.Fatalf("token: %v %v", out, err)
	}
	if b := got["/_matrix/client/v1/rtc/livekit/get_token"]; !strings.Contains(b, `"slot_id":"s"`) || !strings.HasSuffix(b, "|Bearer tok") {
		t.Fatalf("the homeserver was sent %q", b)
	}
	if _, err := be.LiveKitDelayedLeave(context.Background(), json.RawMessage(req)); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["/_matrix/client/v1/rtc/livekit/delegate_delayed_leave"]; !ok {
		t.Fatal("the delayed leave did not go to the homeserver")
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
