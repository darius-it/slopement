package app

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"slopement/calls"
	"slopement/kit"
	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// hoster is a source that can run calls, with the server's answer as given.
type hoster struct {
	model.Source
	sup model.CallSupport
	err error
}

func (h *hoster) CallSupport(context.Context) (model.CallSupport, error) { return h.sup, h.err }
func (h *hoster) CallBackend(string) calls.Backend                       { return nil }
func (h *hoster) CallIdentity() model.CallIdentity {
	return model.CallIdentity{UserID: "@me:x", DeviceID: "D", BaseURL: "https://x"}
}
func (h *hoster) WatchCall(string, func(bool, json.RawMessage)) func() { return func() {} }

func callApp(t *testing.T, h *hoster) (*App, *ui.Tester) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	h.Source = m
	a := New(h, picker.New(nil))
	a.nav = kit.NavRail
	a.runUI = func(f func()) { f() }
	tt := ui.NewTesterWith(a.View, 1300, 900, ui.TesterOptions{Dark: true})
	settle(tt)
	return a, tt
}

// A server that does not offer calls says so in the call area, instead of a
// dead page.
func TestCallOnAServerWithoutCalls(t *testing.T) {
	for name, h := range map[string]*hoster{
		"no livekit":     {sup: model.CallSupport{DelayedEvents: true}},
		"no delayed":     {sup: model.CallSupport{LiveKit: true, Service: "https://x/jwt"}},
		"server errored": {err: context.DeadlineExceeded},
	} {
		t.Run(name, func(t *testing.T) {
			a, tt := callApp(t, h)
			a.open("!crit")
			settle(tt)
			r, _ := a.src.Room("!crit")
			a.joinCall(r)
			deadline := time.Now().Add(3 * time.Second)
			for a.callSess != nil && a.callSess.err == "" && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			settle(tt)
			if a.callSess == nil || a.callSess.err == "" {
				t.Fatal("no reason was given")
			}
			if !tt.HasText(a.callSess.err) {
				t.Fatalf("the reason %q is not shown", a.callSess.err)
			}
			a.leaveCall(a.callSess)
			if a.inCall != "" || a.callSess != nil {
				t.Fatal("leaving did not end the session")
			}
		})
	}
}

// With no window to show it in (as here), a supported call says so too.
func TestCallNeedsAWindow(t *testing.T) {
	a, tt := callApp(t, &hoster{sup: model.CallSupport{LiveKit: true, DelayedEvents: true}})
	a.open("!crit")
	settle(tt)
	r, _ := a.src.Room("!crit")
	a.joinCall(r)
	deadline := time.Now().Add(3 * time.Second)
	for a.callSess != nil && a.callSess.err == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if a.callSess == nil || !strings.Contains(a.callSess.err, "window") {
		t.Fatalf("session: %+v", a.callSess)
	}
}

// The page's address carries what Element Call needs as a widget, and the
// host page forwards only the call origin's messages.
func TestCallWidgetURL(t *testing.T) {
	u := callWidgetURL(model.CallIdentity{UserID: "@me:x", DeviceID: "D", BaseURL: "https://x"}, "!r:x", "w1", "start_call", true)
	if !strings.HasPrefix(u, callOrigin+"/room#?") {
		t.Fatalf("url %s", u)
	}
	q, err := url.ParseQuery(strings.TrimPrefix(u, callOrigin+"/room#?"))
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"embed": "true", "widgetId": "w1", "roomId": "!r:x", "userId": "@me:x", "deviceId": "D",
		"baseUrl": "https://x", "parentUrl": callHostURL, "skipLobby": "true", "confineToRoom": "true", "intent": "start_call"} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if lobby := callWidgetURL(model.CallIdentity{}, "!r", "w", "start_call", false); !strings.Contains(lobby, "skipLobby=false") {
		t.Fatalf("with the lobby on the page is not told to skip it: %s", lobby)
	}
	page := callHostPage(u, false)
	if !strings.Contains(page, "e.origin !== ORIGIN") || !strings.Contains(page, "e.source !== f.contentWindow") {
		t.Fatal("the host page does not check where messages come from")
	}
	if strings.Contains(page, "CallBridge.Log") {
		t.Fatal("the click trace is in a page made without debug")
	}
	// With debug the page traces where clicks go.
	traced := callHostPage(u, true)
	for _, want := range []string{"CallBridge.Log", `addEventListener("blur"`, "pointerdown", "the call page loaded"} {
		if !strings.Contains(traced, want) {
			t.Errorf("the debug page has no %q", want)
		}
	}
}

// The bridge answers for the active session only.
func TestCallBridgeRoutesToTheActiveSession(t *testing.T) {
	var b CallBridge
	activeCall.Store(nil)
	if got := b.Post(context.Background(), `{}`); got != nil {
		t.Fatal("answered with no call")
	}
	s := &callSession{host: calls.NewHost("w", nil)}
	activeCall.Store(s)
	defer activeCall.Store(nil)
	got := b.Post(context.Background(), `{"api":"fromWidget","requestId":"1","action":"supported_api_versions","widgetId":"w","data":{}}`)
	if len(got) != 1 || !strings.Contains(got[0], "supported_versions") {
		t.Fatalf("reply: %v", got)
	}
}
