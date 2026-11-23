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
	"strconv"
	"strings"
	"sync"
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

// callTrace: the call's handshake and clicks are written to the app's log (the
// one Settings → Advanced → Export logs collects), as action and event names
// only — what the page asked, what it was told, where clicks went — never what
// anyone said. A call that does not behave is then a log to send, not a guess.
var callTrace = true

// callAreaZIndex orders the call area above its siblings in the room (the
// timeline and the composer), so that none of their boxes counts as covering
// the web view in it.
const callAreaZIndex = 1

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
	if callTrace { // what the call page asked, and whether we answered with an error
		var m struct {
			Action string
			Data   struct{ Type string }
		}
		_ = json.Unmarshal([]byte(msg), &m)
		what := m.Action
		if m.Data.Type != "" {
			what += " " + m.Data.Type // which event it sends or reads
		}
		log.Printf("call: widget → %s (%d replies%s)", what, len(out), errNote(out))
	}
	return out
}

// Log is a line from the host page's click trace.
func (CallBridge) Log(msg string) {
	if callTrace {
		log.Printf("call page: %s", msg)
	}
}

// BindCalls makes the bridge callable by the host page; call it before the
// app runs.
func BindCalls() { mygo.Bind(&CallBridge{}) }

// callsNote says, in the words used in Settings and in the question before the
// first call, what loading the call page from Element means.
const callsNote = "Calls run in Element Call, whose page loads from call.element.io: a server run by Element, not by your homeserver. " +
	"That server learns your IP address when the page loads. While you are in a call the page sees your user and device ID, the room, " +
	"who is in the call and a short-lived sign-in token for the call service; your messages and your account stay in this app. " +
	"The call's audio and video go to your own homeserver's call service. Nothing is loaded until you press Call."

// askCallsConsent asks, once, before the page is loaded from Element's server;
// a yes starts the call, and is kept (Settings → Calls takes it back).
func (a *App) askCallsConsent(r model.Room) {
	win := a.win
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Title: "Calls load a page from Element",
			Message: callsNote, Buttons: []string{"Continue", "Not now"}, DefaultButton: 1, CancelButton: 1})
		if err != nil || res.Button != 0 {
			return
		}
		win.Update(func() {
			a.prefs.callsAccepted = true
			a.joinCall(r)
		})
	}()
}

// startCall begins the room's call: it asks the server what it offers, then
// opens Element Call. A source that cannot run calls (the mock) shows the
// scaffold as before.
func (a *App) startCall(r model.Room) bool {
	hoster, ok := a.src.(model.CallHoster)
	if !ok {
		return false
	}
	if a.win != nil && !a.prefs.callsAccepted { // the real app: ask before loading the page from Element
		a.askCallsConsent(r)
		return true
	}
	sess := &callSession{room: r.ID, status: "Checking what your server offers…"}
	a.callSess, a.inCall, a.call = sess, r.ID, kit.NewCall()
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
	if callTrace {
		host.Logf = func(f string, args ...any) { log.Printf("call: "+f, args...) }
	}
	host.OnHangup = func() { a.runUI(func() { a.endCall(sess) }) } // the page has left: close it
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
		if callTrace {
			log.Printf("call: permission %v asked by %s → %v", req.Permissions, req.Origin, ok)
		}
		return ok
	})
	sess.wv = wv
	sess.status = "Joining…"

	// What the room's call events say reaches the widget, as far as it may hear.
	sess.stop = hoster.WatchCall(r.ID, func(toDevice bool, raw json.RawMessage) {
		ready := host.Ready()
		heard := host.Receives(raw)
		kind := "room event"
		if toDevice {
			heard, kind = host.ReceivesToDevice(raw), "to-device"
		}
		if callTrace { // what arrived, and whether the page was told
			var e struct{ Type string }
			_ = json.Unmarshal(raw, &e)
			log.Printf("call: %s %s → page: ready %v, allowed %v", kind, e.Type, ready, heard)
		}
		if !ready || !heard {
			return
		}
		_ = callToWidget.EmitPage(wv.Page(), string(host.Deliver(toDevice, json.RawMessage(raw))))
	})
	// Joining at once (no lobby): the microphone and camera the user starts calls
	// with are set the moment the page says it joined.
	// Only what differs from how the page itself started is asked of it: with
	// the page already as wanted, it is not touched (its own buttons keep the
	// say), and a request that changes nothing is not sent at all.
	if !a.prefs.callLobby {
		wantMic, wantCam := !a.prefs.callMicOff, a.prefs.callCamOn
		var mu sync.Mutex
		var micNow, camNow *bool // how the page says its devices are
		host.OnMute = func(audio, video *bool) {
			mu.Lock()
			defer mu.Unlock()
			if audio != nil {
				micNow = audio
			}
			if video != nil {
				camNow = video
			}
		}
		host.OnJoin = func() {
			mu.Lock()
			var audio, video *bool
			if micNow == nil || *micNow != wantMic {
				audio = &wantMic
			}
			if camNow == nil || *camNow != wantCam {
				video = &wantCam
			}
			mu.Unlock()
			if audio == nil && video == nil {
				return
			}
			_ = callToWidget.EmitPage(wv.Page(), string(host.SetMute(audio, video)))
		}
	}
	debug := callTrace
	if os.Getenv("SLOPEMENT_CALL_DEVTOOLS") != "" {
		wv.Page().OpenDevTools() // the web inspector on the call page: its console and what is under a click
	}
	wv.Page().LoadHTML(callHostPage(callWidgetURL(id, r.ID, widgetID, a.callIntent(r), !a.prefs.callLobby), debug), callHostURL)
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
func callWidgetURL(id model.CallIdentity, room, widgetID, intent string, skipLobby bool) string {
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
	q.Set("skipLobby", strconv.FormatBool(skipLobby))
	q.Set("header", "none")
	q.Set("confineToRoom", "true")
	q.Set("appPrompt", "false")
	q.Set("perParticipantE2EE", "true")
	return callOrigin + "/room#?" + q.Encode()
}

// callHostPage is the page the web view shows: the call in an iframe, and the
// bridge between the iframe's messages and Go.
func callHostPage(src string, debug bool) string {
	srcJSON, _ := json.Marshal(src)
	originJSON, _ := json.Marshal(callOrigin)
	trace := ""
	if debug {
		// Where clicks go, for a call whose buttons do not answer: the window
		// loses focus to the call page when a click lands in it, and a click on
		// this page itself (not the call) is seen here.
		trace = `
const dbg = (m) => window.mygo.call("CallBridge.Log", m).catch(() => {});
window.addEventListener("blur", () => dbg("the host page lost focus: a click or focus went into the call page"));
window.addEventListener("focus", () => dbg("the host page has focus again"));
document.addEventListener("pointerdown", (e) => dbg("a press on the host page itself, not the call, at " + e.clientX + "," + e.clientY), true);
window.addEventListener("error", (e) => dbg("host page error: " + e.message));
f.addEventListener("load", () => dbg("the call page loaded"));
`
	}
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
});` + trace + `
window.mygo.on("call-to-widget", (m) => f.contentWindow && f.contentWindow.postMessage(JSON.parse(m), ORIGIN));
f.src = ` + string(srcJSON) + `;
</script>`
}

// realCallArea is the call over the timeline: the call page, all of it (its
// own buttons mute, share and hang up), or what is going on, or why it could
// not start. fixed is its height (0: the room the split around it gives, which
// the user drags).
func (a *App) realCallArea(c *ui.Context, s *callSession, fixed float32) {
	p := kit.Of(c)
	// Above the timeline (callAreaZIndex): a web view takes the pointer except
	// where an element built after it overlaps it, by that element's whole box,
	// clipped away or not — and the timeline is built after, with rows (those a list
	// builds just past its edge) whose boxes reach up over the lower part of the
	// page. Those spots stopped taking the page's clicks: an invisible strip across
	// its bottom, where its buttons are. Ordered above, the timeline is no longer
	// "after" the page.
	col := ui.Column(c.Key("call")).Background(p.BgInset).Padding(kit.S2).Gap(kit.S2).ZIndex(callAreaZIndex)
	if fixed > 0 {
		col.Height(fixed).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(p.Border)
	} else {
		col.Fill()
	}
	col.Children(func() {
		switch {
		case s.err != "":
			ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(kit.S2).Children(func() {
				ui.Text(c, s.err).FontSize(kit.SizeSmall).TextColor(p.TextMuted).TextAlign(ui.Center)
				if kit.Button(c, kit.Secondary, "Close", nil).Clicked() { // no page to end it from
					a.endCall(s)
				}
			})
		case s.wv != nil:
			ui.WebView(c.Key("callpage"), s.wv).Grow(1).Radius(kit.RadiusLg)
		default:
			ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(kit.S2).Children(func() {
				ui.Text(c, s.status).FontSize(kit.SizeSmall).TextColor(p.TextMuted)
				if kit.Button(c, kit.Ghost, "Cancel", nil).Clicked() {
					a.endCall(s)
				}
			})
		}
	})
}

// callReturnBar is the slim line shown in any other room while a call goes on:
// the call carries on in its page (kept, though not shown), and this is the way
// back to it, or out.
func (a *App) callReturnBar(c *ui.Context, shown model.Room) {
	s := a.callSess
	if s == nil || a.inCall == "" || a.inCall == shown.ID || s.wv == nil {
		return
	}
	p := kit.Of(c)
	name := "a call"
	if r, ok := a.roomByID(a.inCall); ok {
		name = r.Name
	}
	ui.Row(c.Key("callreturn")).Shrink(0).Padding(kit.S2, kit.S3).Gap(kit.S2).AlignItems(ui.Center).
		Background(p.AccentSubtle).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
		ui.Box(c).Size(8, 8).Radius(kit.RadiusFull).Background(p.Success)
		ui.Text(c, "In a call · "+name).FontSize(kit.SizeSmall).FontWeight(kit.WeightMedium).Grow(1).SingleLine()
		if kit.Button(c, kit.Secondary, "Return", kit.IconPhone).Clicked() {
			a.open(a.inCall)
		}
		if kit.Button(c, kit.Ghost, "Leave", nil).Clicked() {
			a.leaveCall(s)
		}
	})
}

// callGrip is the thin bar under the call page: dragged up or down it makes the
// page smaller or bigger (a double click puts it back). It is a sibling of the
// page, not a wrapper, so the page is laid out and hit as a plain box.
func (a *App) callGrip(c *ui.Context) {
	p := kit.Of(c)
	_, h := c.Size()
	maxH := h - 260 // the header and a few messages stay
	if maxH < 160 {
		maxH = 160
	}
	grip := ui.ButtonBase(c.Key("callgrip")).Height(10).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorResizeNS).Label("Resize the call")
	if _, dy, ok := grip.Dragged(); ok {
		a.callHeight = min(max(a.callHeight+dy, 120), maxH)
	}
	if grip.DoubleClicked() {
		a.callHeight = min(440, maxH)
	}
	grip.Children(func() {
		col := p.Border
		if grip.Hovered() || grip.Pressed() {
			col = p.TextFaint
		}
		ui.Box(c).Size(40, 3).Radius(kit.RadiusFull).Background(col)
	})
	a.callHeight = min(max(a.callHeight, 120), maxH) // a smaller window takes it down too
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
