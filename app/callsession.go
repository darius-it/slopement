package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html"
	"log"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"slopement/calls"
	"slopement/kit"
	"slopement/model"
)

// A call is Element Call (MatrixRTC) in a web view: it runs as a widget in an
// iframe of a small page of ours, and asks us — the host (calls.Host) — to do
// the Matrix side. Spike: the call page is loaded from callOrigin (hosted by
// Element); bundling it comes later.

const (
	callOrigin = "https://call.element.io"
	// callHostURL is the web address our host page claims, so the widget has an
	// origin to post to; trusted below, so the page may reach CallBridge.
	callHostOrigin = "https://slopement.invalid"
	callHostURL    = callHostOrigin + "/call"
)

// callSession is the call the user is in (or starting).
type callSession struct {
	room    string
	status  string // what is going on, while there is no page yet
	err     string // why it could not start
	leaving bool   // hung up: waiting for the page to leave the call
	wv      *mygo.WebView
	host    *calls.Host
	stop    func() // ends the room watch
}

// activeCall is the session CallBridge talks for: one call at a time.
var activeCall atomic.Pointer[callSession]

// callToWidget carries a message to the widget: the host page forwards it to
// the iframe.
var callToWidget = mygo.NewEvent[string]("call-to-widget")

// CallBridge is bound for the host page: the widget's messages come in
// through Post, and the replies go back to it.
type CallBridge struct{}

// Post takes one message from the widget and returns what to post back.
func (CallBridge) Post(ctx context.Context, msg string) []string {
	s := activeCall.Load()
	if s == nil || s.host == nil {
		return nil
	}
	var out []string
	for _, m := range s.host.Handle(ctx, []byte(msg)) {
		out = append(out, string(m))
	}
	if os.Getenv("SLOPEMENT_DEBUG") != "" { // what the call page asked, and whether we answered with an error
		var m struct{ Action string }
		_ = json.Unmarshal([]byte(msg), &m)
		log.Printf("call: widget → %s (%d replies%s)", m.Action, len(out), errNote(out))
	}
	return out
}

// BindCalls makes the bridge callable by the host page; call it before the
// app runs.
func BindCalls() { mygo.Bind(&CallBridge{}) }

// startCall begins the room's call: it asks the server what it offers, then
// opens Element Call. A source that cannot run calls (the mock) shows the
// scaffold as before.
func (a *App) startCall(r model.Room) bool {
	hoster, ok := a.src.(model.CallHoster)
	if !ok {
		return false
	}
	sess := &callSession{room: r.ID, status: "Checking what your server offers…"}
	a.callSess, a.inCall, a.call = sess, r.ID, kit.NewCall()
	a.call.NoShare = true // the page's own screen sharing is the one that works
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sup, err := hoster.CallSupport(ctx)
		a.runUI(func() {
			if a.callSess != sess {
				return // left meanwhile
			}
			switch {
			case err != nil:
				sess.err = "Could not reach your server: " + err.Error()
			case !sup.LiveKit:
				sess.err = "Your server does not offer calls (no LiveKit service is advertised in its .well-known)."
			case !sup.DelayedEvents:
				sess.err = "Your server cannot expire a call's membership by itself (delayed events, MSC4140), which calls need."
			default:
				a.launchCall(sess, hoster, r)
			}
		})
	}()
	return true
}

// launchCall opens the web view on Element Call for the room.
func (a *App) launchCall(sess *callSession, hoster model.CallHoster, r model.Room) {
	if a.win == nil {
		sess.err = "There is no window to show the call in."
		return
	}
	widgetID := "slopement-" + randomHex(6)
	id := hoster.CallIdentity()
	host := calls.NewHost(widgetID, hoster.CallBackend(r.ID))
	if os.Getenv("SLOPEMENT_DEBUG") != "" {
		host.Logf = func(f string, args ...any) { log.Printf("call: "+f, args...) }
	}
	host.OnHangup = func() { a.runUI(func() { a.endCall(sess) }) } // the page has left: close it
	host.OnMute = func(audio, video *bool) { a.runUI(func() { a.callMuteSeen(sess, audio, video) }) }
	sess.host = host
	activeCall.Store(sess)

	wv, err := a.win.NewWebView(mygo.WebViewOptions{
		Page:            mygo.PageOptions{TrustedOrigins: []string{callHostOrigin}},
		BackgroundColor: "light-dark(#FCFCFD, #121116)",
	})
	if err != nil {
		sess.err = "Could not open the call: " + err.Error()
		return
	}
	// The call page may use the camera, the microphone and the screen; nothing
	// else (no location, no notifications).
	wv.Page().SetPermissionHandler(func(req mygo.PermissionRequest) bool {
		// The call page (the iframe) asks, and so may our host page around it
		// when the system re-asks for what the frame holds (a microphone turned
		// back on): both are ours to answer.
		ok := req.Origin == callOrigin || req.Origin == callHostOrigin
		for _, p := range req.Permissions {
			if p == mygo.PermissionGeolocation || p == mygo.PermissionNotifications {
				ok = false
			}
		}
		if os.Getenv("SLOPEMENT_DEBUG") != "" {
			log.Printf("call: permission %v asked by %s → %v", req.Permissions, req.Origin, ok)
		}
		return ok
	})
	sess.wv = wv
	sess.status = "Joining…"

	// What the room's call events say reaches the widget, as far as it may hear.
	sess.stop = hoster.WatchCall(r.ID, func(raw json.RawMessage) {
		if !host.Ready() || !host.Receives(raw) {
			return
		}
		_ = callToWidget.EmitPage(wv.Page(), string(host.Deliver(false, json.RawMessage(raw))))
	})
	wv.Page().LoadHTML(callHostPage(callWidgetURL(id, r.ID, widgetID, a.callIntent(r))), callHostURL)
	a.win.Invalidate()
}

// callIntent: join the call going on in the room, or start one.
func (a *App) callIntent(r model.Room) string {
	if r.Kind == model.KindDM {
		return "start_call_dm"
	}
	return "start_call"
}

// callWidgetURL is Element Call's page for a room, as the widget it runs as:
// embedded, no lobby, no header, confined to the room.
func callWidgetURL(id model.CallIdentity, room, widgetID, intent string) string {
	q := url.Values{}
	q.Set("embed", "true")
	q.Set("widgetId", widgetID)
	q.Set("parentUrl", callHostURL)
	q.Set("userId", id.UserID)
	q.Set("deviceId", id.DeviceID)
	q.Set("roomId", room)
	q.Set("baseUrl", id.BaseURL)
	q.Set("lang", "en")
	q.Set("intent", intent)
	q.Set("skipLobby", "true")
	q.Set("header", "none")
	q.Set("confineToRoom", "true")
	q.Set("appPrompt", "false")
	q.Set("perParticipantE2EE", "true")
	return callOrigin + "/room#?" + q.Encode()
}

// callHostPage is the page the web view shows: the call in an iframe, and the
// bridge between the iframe's messages and Go.
func callHostPage(src string) string {
	srcJSON, _ := json.Marshal(src)
	originJSON, _ := json.Marshal(callOrigin)
	return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<style>html,body{margin:0;height:100%;background:#121116}iframe{border:0;width:100%;height:100%;display:block}</style>
<iframe id="f" title="` + html.EscapeString("Call") + `" allow="camera; microphone; display-capture; autoplay; clipboard-write; fullscreen"></iframe>
<script>
const f = document.getElementById("f"), ORIGIN = ` + string(originJSON) + `;
window.addEventListener("message", async (e) => {
  if (e.source !== f.contentWindow || e.origin !== ORIGIN) return;
  const msg = typeof e.data === "string" ? e.data : JSON.stringify(e.data);
  try {
    const replies = await window.mygo.call("CallBridge.Post", msg);
    for (const r of replies || []) f.contentWindow.postMessage(JSON.parse(r), ORIGIN);
  } catch (err) { console.error("call bridge", err); }
});
window.mygo.on("call-to-widget", (m) => f.contentWindow && f.contentWindow.postMessage(JSON.parse(m), ORIGIN));
f.src = ` + string(srcJSON) + `;
</script>`
}

// realCallArea is the call over the timeline: the page, and our bar under it
// (or what is going on, or why it could not start).
func (a *App) realCallArea(c *ui.Context, s *callSession) {
	p := kit.Of(c)
	ui.Column(c.Key("call")).Height(440).Shrink(0).Background(p.BgInset).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).
		Padding(kit.S2).Gap(kit.S2).Children(func() {
		switch {
		case s.err != "":
			ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(kit.S2).Children(func() {
				ui.Text(c, s.err).FontSize(kit.SizeSmall).TextColor(p.TextMuted).TextAlign(ui.Center)
			})
		case s.wv != nil:
			ui.WebView(c.Key("callpage"), s.wv).Grow(1).Radius(kit.RadiusLg)
		default:
			ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, s.status).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
			})
		}
		ui.Row(c).Justify(ui.Center).Children(func() {
			before := a.call
			act := kit.CallBar(c, &a.call)
			a.callMedia(before)
			if act.Leave {
				a.leaveCall(s)
			}
		})
	})
}

// leaveGrace is how long the page gets to leave the call by itself before it
// is closed anyway.
const leaveGrace = 4 * time.Second

// leaveCall hangs up: the page is asked to leave the call (so the others see
// the user go at once, not when a timer runs out), and closed when it says it
// has, or after leaveGrace. Without a page it ends at once.
func (a *App) leaveCall(sess *callSession) {
	if sess == nil || a.callSess != sess {
		return
	}
	if sess.wv == nil || sess.host == nil || !sess.host.Ready() {
		a.endCall(sess)
		return
	}
	if sess.leaving {
		return
	}
	sess.leaving = true
	sess.status = "Leaving…"
	_ = callToWidget.EmitPage(sess.wv.Page(), string(sess.host.Hangup()))
	run := a.runUI
	time.AfterFunc(leaveGrace, func() { run(func() { a.endCall(sess) }) })
}

// endCall ends the session: the page goes, the room watch stops.
func (a *App) endCall(sess *callSession) {
	if sess == nil || a.callSess != sess {
		return
	}
	if sess.stop != nil {
		sess.stop()
	}
	if activeCall.Load() == sess {
		activeCall.Store(nil)
	}
	if sess.wv != nil {
		sess.wv.Destroy()
		// Whatever the page did or did not do on its way out, this device's
		// membership in the call ends now: a call must not outlive its window.
		if cc, ok := a.src.(model.CallCleaner); ok {
			room := sess.room
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				_, _ = cc.ClearCallMembership(ctx, room, true)
			}()
		}
	}
	a.callSess, a.inCall = nil, ""
}

// callMedia applies the call bar's microphone and camera to the page: by the
// widget API's device-mute request, which the page honours as if its own
// buttons were pressed (muting the web view's capture from outside leaves the
// page believing it is still on).
func (a *App) callMedia(before kit.CallState) {
	s := a.callSess
	if s == nil || s.wv == nil || s.host == nil || !s.host.Ready() {
		return
	}
	var audio, video *bool
	if before.Mic != a.call.Mic {
		audio = &a.call.Mic
	}
	if before.Camera != a.call.Camera {
		video = &a.call.Camera
	}
	if audio == nil && video == nil {
		return
	}
	_ = callToWidget.EmitPage(s.wv.Page(), string(s.host.SetMute(audio, video)))
}

// callMuteSeen takes what the page says about its microphone and camera (its
// own buttons, or its answer to ours) into the bar, so the bar shows the truth.
func (a *App) callMuteSeen(sess *callSession, audio, video *bool) {
	if a.callSess != sess {
		return
	}
	if audio != nil {
		a.call.Mic = *audio
	}
	if video != nil {
		a.call.Camera = *video
	}
}

// errNote says, for the debug log, that a reply carried an error.
func errNote(replies []string) string {
	for _, r := range replies {
		if strings.Contains(r, `"error":{"message":`) {
			return ", ERROR: " + r
		}
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
