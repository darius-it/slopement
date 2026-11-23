package calls

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type fakeBackend struct{ sent []string }

func (f *fakeBackend) SendEvent(_ context.Context, d json.RawMessage) (any, error) {
	f.sent = append(f.sent, string(d))
	return map[string]any{"room_id": "!r", "event_id": "$e"}, nil
}
func (f *fakeBackend) ReadEvents(context.Context, json.RawMessage) (any, error) {
	return map[string]any{"events": []any{}}, nil
}
func (f *fakeBackend) SendToDevice(context.Context, json.RawMessage) (any, error) {
	return map[string]any{}, nil
}
func (f *fakeBackend) UpdateDelayedEvent(context.Context, json.RawMessage) (any, error) {
	return map[string]any{}, nil
}
func (f *fakeBackend) OpenID(context.Context) (any, error) {
	return map[string]any{"state": "allowed", "access_token": "t"}, nil
}
func (f *fakeBackend) TURNServers(context.Context) (any, error) { return map[string]any{}, nil }
func (f *fakeBackend) RTCTransports(context.Context) (any, error) {
	return map[string]any{"rtc_transports": []any{map[string]any{"type": "livekit"}}}, nil
}
func (f *fakeBackend) LiveKitToken(context.Context, json.RawMessage) (any, error) {
	return map[string]any{"jwt": "j"}, nil
}
func (f *fakeBackend) LiveKitDelayedLeave(context.Context, json.RawMessage) (any, error) {
	return map[string]any{}, nil
}

// Hanging up asks the widget to leave (so it removes the user's membership),
// and the widget saying it has left reaches OnHangup.
func TestHangupAsksTheWidgetToLeave(t *testing.T) {
	h := NewHost("w", &fakeBackend{})
	m := msg(t, h.Hangup())
	if m.API != "toWidget" || m.Action != "im.vector.hangup" || m.WidgetID != "w" || m.RequestID == "" {
		t.Fatalf("hangup message %+v", m)
	}
	var left bool
	h.OnHangup = func() { left = true }
	h.Handle(context.Background(), from("im.vector.hangup", "9", `{}`))
	if !left {
		t.Fatal("the widget's hangup did not reach the host")
	}
}

// The MatrixRTC requests (transports, token, delayed leave) answer only once
// the page was granted their capabilities, and the host says it speaks them.
func TestMatrixRTCRequestsNeedTheirCapabilities(t *testing.T) {
	h := NewHost("w", &fakeBackend{})
	ctx := context.Background()
	if out := h.Handle(ctx, from("supported_api_versions", "v", `{}`)); !strings.Contains(string(out[0]), "org.matrix.msc4515") ||
		!strings.Contains(string(out[0]), "org.matrix.msc4533") {
		t.Fatalf("the host does not advertise the RTC API versions: %s", out[0])
	}
	req := "org.matrix.msc4515.get_rtc_transports"
	if got := h.Handle(ctx, from(req, "1", `{}`)); !containsError(msg(t, got[0]).Response) {
		t.Fatal("transports answered without the capability")
	}
	h.granted = Grant([]string{"org.matrix.msc4515.rtc_transports", "org.matrix.msc4533.rtc_livekit_get_token",
		"org.matrix.msc4533.rtc_livekit_delegate_delayed_leave", "org.matrix.msc4407.send.sticky_event:m.rtc.member"})
	if len(h.granted) != 4 {
		t.Fatalf("granted %v", h.granted)
	}
	for action, want := range map[string]string{req: "rtc_transports",
		"org.matrix.msc4533.rtc_livekit_get_token":              "jwt",
		"org.matrix.msc4533.rtc_livekit_delegate_delayed_leave": "{}"} {
		got := h.Handle(ctx, from(action, "2", `{"url":"https://x","room_id":"!r","slot_id":"s","member_id":"m","delay_id":"d"}`))
		if r := string(msg(t, got[0]).Response); containsError(msg(t, got[0]).Response) || !strings.Contains(r, want) {
			t.Fatalf("%s: %s", action, r)
		}
	}
	var logged []string
	h2 := NewHost("w", &fakeBackend{})
	h2.Logf = func(f string, a ...any) { logged = append(logged, f) }
	h2.pending["x"] = "capabilities"
	h2.Handle(ctx, []byte(`{"api":"toWidget","requestId":"x","action":"capabilities","widgetId":"w","response":{"capabilities":["org.matrix.msc2762.send.event:m.room.message"]}}`))
	if len(logged) != 1 {
		t.Fatal("a refused capability was not logged")
	}
}

func msg(t *testing.T, b []byte) Message {
	t.Helper()
	var m Message
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func from(action, id string, data string) []byte {
	return []byte(`{"api":"fromWidget","requestId":"` + id + `","action":"` + action + `","widgetId":"w","data":` + data + `}`)
}

// The handshake: the widget asks the versions, says it has loaded, the host
// asks for capabilities, and approves only what a call needs.
func TestHandshakeGrantsOnlyWhatACallNeeds(t *testing.T) {
	h := NewHost("w", &fakeBackend{})
	ctx := context.Background()

	out := h.Handle(ctx, from("supported_api_versions", "1", `{}`))
	if len(out) != 1 || msg(t, out[0]).Action != "supported_api_versions" || len(msg(t, out[0]).Response) == 0 {
		t.Fatalf("versions: %s", out)
	}

	out = h.Handle(ctx, from("content_loaded", "2", `{}`))
	if len(out) != 2 || !h.Ready() {
		t.Fatalf("content_loaded: %d messages, ready %v", len(out), h.Ready())
	}
	ask := msg(t, out[1])
	if ask.API != "toWidget" || ask.Action != "capabilities" {
		t.Fatalf("the host did not ask for capabilities: %+v", ask)
	}

	requested := []string{
		"org.matrix.msc2762.send.state_event:org.matrix.msc3401.call.member#@me:x_DEV",
		"org.matrix.msc2762.receive.state_event:m.room.member",
		"org.matrix.msc3819.send.to_device:io.element.call.encryption_keys",
		"org.matrix.msc2762.send.event:m.room.message", // a chat message: refused
		"org.matrix.msc2762.send.state_event:m.room.power_levels#",
		"org.matrix.msc2762.receive.state_event:m.room.topic", // not needed
		"town.robin.msc3846.turn_servers",
		"something.unknown",
	}
	reqJSON, _ := json.Marshal(map[string]any{"capabilities": requested})
	out = h.Handle(ctx, []byte(`{"api":"toWidget","requestId":"`+ask.RequestID+`","action":"capabilities","widgetId":"w","response":`+string(reqJSON)+`}`))
	if len(out) != 1 || msg(t, out[0]).Action != "notify_capabilities" {
		t.Fatalf("no notify_capabilities: %s", out)
	}
	want := []string{requested[0], requested[1], requested[2], requested[4], requested[6]}
	if got := h.Granted(); !reflect.DeepEqual(got, want) {
		t.Fatalf("granted %v\nwant    %v", got, want)
	}
}

// Sending and reading go through only for the types granted.
func TestRequestsFollowTheGrant(t *testing.T) {
	be := &fakeBackend{}
	h := NewHost("w", be)
	ctx := context.Background()
	h.granted = []string{"org.matrix.msc2762.send.state_event:m.rtc.member#@me:x_D", "org.matrix.msc2762.receive.state_event:m.rtc.member"}

	ok := h.Handle(ctx, from("send_event", "a", `{"type":"m.rtc.member","state_key":"@me:x_D","content":{}}`))
	if r := msg(t, ok[0]).Response; len(r) == 0 || string(r) == "" || len(be.sent) != 1 {
		t.Fatalf("a granted send did not go through: %s, sent %v", ok[0], be.sent)
	}
	denied := h.Handle(ctx, from("send_event", "b", `{"type":"m.room.message","content":{"body":"hi"}}`))
	if len(be.sent) != 1 || !containsError(msg(t, denied[0]).Response) {
		t.Fatalf("a message was sent without a grant: %s", denied[0])
	}
	if got := h.Handle(ctx, from("read_events", "c", `{"type":"m.rtc.member","state_key":true}`)); containsError(msg(t, got[0]).Response) {
		t.Fatalf("a granted read was refused: %s", got[0])
	}
	if got := h.Handle(ctx, from("read_events", "d", `{"type":"m.room.topic"}`)); !containsError(msg(t, got[0]).Response) {
		t.Fatalf("an ungranted read went through: %s", got[0])
	}
	if got := h.Handle(ctx, from("no_such_thing", "e", `{}`)); !containsError(msg(t, got[0]).Response) {
		t.Fatalf("an unknown action was not refused: %s", got[0])
	}
	if got := h.Handle(ctx, []byte(`{"api":"fromWidget","requestId":"z","action":"send_event","widgetId":"other","data":{}}`)); got != nil {
		t.Fatal("another widget's message was answered")
	}
	if got := h.Handle(ctx, []byte(`not json`)); got != nil {
		t.Fatal("garbage was answered")
	}
}

func containsError(resp json.RawMessage) bool {
	var r struct {
		Error *struct{ Message string } `json:"error"`
	}
	_ = json.Unmarshal(resp, &r)
	return r.Error != nil
}
