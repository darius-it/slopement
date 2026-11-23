// Package calls is slopement's side of Matrix calls (MatrixRTC, with Element
// Call as the call page: docs/calls-research.md). Element Call runs as a widget
// in a web view and asks its host — us — to do the Matrix side through the
// widget API (postMessage). Host is that protocol, pure Go: the web view feeds
// it the page's messages and posts what it returns; Backend is the Matrix
// account behind it.
package calls

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

// Message is one widget API message, either way.
type Message struct {
	API       string          `json:"api"` // "fromWidget" or "toWidget"
	RequestID string          `json:"requestId"`
	Action    string          `json:"action"`
	WidgetID  string          `json:"widgetId"`
	Data      json.RawMessage `json:"data,omitempty"`
	Response  json.RawMessage `json:"response,omitempty"`
}

// Backend is what the widget's requests turn into on the Matrix side. Each
// takes the request's data as the widget sent it and returns the response's.
type Backend interface {
	SendEvent(ctx context.Context, data json.RawMessage) (any, error)
	ReadEvents(ctx context.Context, data json.RawMessage) (any, error)
	SendToDevice(ctx context.Context, data json.RawMessage) (any, error)
	UpdateDelayedEvent(ctx context.Context, data json.RawMessage) (any, error)
	OpenID(ctx context.Context) (any, error)
	TURNServers(ctx context.Context) (any, error)
	// MatrixRTC (MSC4515, MSC4533): where the call's media server is, a token
	// for it, and handing it the call's delayed leave.
	RTCTransports(ctx context.Context) (any, error)
	LiveKitToken(ctx context.Context, data json.RawMessage) (any, error)
	LiveKitDelayedLeave(ctx context.Context, data json.RawMessage) (any, error)
}

// Capabilities of the MatrixRTC requests, and the actions they allow.
const (
	capRTCTransports = "org.matrix.msc4515.rtc_transports"
	capLiveKitToken  = "org.matrix.msc4533.rtc_livekit_get_token"
	capLiveKitLeave  = "org.matrix.msc4533.rtc_livekit_delegate_delayed_leave"
)

// supportedVersions are the widget API versions the host speaks.
var supportedVersions = []string{"0.0.1", "0.0.2", "org.matrix.msc2762", "org.matrix.msc2871", "org.matrix.msc2931",
	"org.matrix.msc2974", "org.matrix.msc2876", "org.matrix.msc3819", "town.robin.msc3846", "org.matrix.msc3869",
	"org.matrix.msc3973", "org.matrix.msc4157", "org.matrix.msc4515", "org.matrix.msc4533"}

// allowedKinds are the capabilities a call page may be granted, before the
// colon: it sends and reads the call's state events and to-device messages,
// reads the room's, and asks for TURN servers. Nothing like sending a message.
var allowedKinds = map[string]bool{
	"org.matrix.msc2762.send.state_event":     true,
	"org.matrix.msc2762.receive.state_event":  true,
	"org.matrix.msc2762.send.event":           true,
	"org.matrix.msc2762.receive.event":        true,
	"org.matrix.msc3819.send.to_device":       true,
	"org.matrix.msc3819.receive.to_device":    true,
	"org.matrix.msc4157.send.delayed_event":   true,
	"org.matrix.msc4157.update_delayed_event": true,
	"town.robin.msc3846.turn_servers":         true,
	"m.always_on_screen":                      true,
	"org.matrix.msc2762.timeline":             true, // reads the room's recent events (the call's encryption keys)
	"org.matrix.msc4407.send.sticky_event":    true, // a call's membership may be a sticky event
	"org.matrix.msc4407.receive.sticky_event": true,
	capRTCTransports:                          true,
	capLiveKitToken:                           true,
	capLiveKitLeave:                           true,
}

// allowedTypes are the event types the page may send or read through those.
var allowedTypes = map[string]bool{
	"m.rtc.member": true, "org.matrix.msc3401.call.member": true, "org.matrix.msc3401.call": true,
	"m.call.member": true, "m.rtc.notification": true, "org.matrix.msc4075.rtc.notification": true,
	"m.room.member": true, "m.room.create": true, "m.room.encryption": true, "m.room.power_levels": true,
	"m.room.name": true, "m.room.avatar": true, "m.reaction": true, "io.element.call.reaction": true,
	"org.matrix.msc4310.rtc.decline": true, "m.rtc.decline": true,
	"io.element.call.encryption_keys": true, "io.element.call.encryption_keys.": true,
	"io.element.call.reaction.": true,
}

// Grant is the subset of the requested capabilities the host approves.
func Grant(requested []string) []string {
	var out []string
	for _, cap := range requested {
		kind, arg, _ := strings.Cut(cap, ":")
		if !allowedKinds[kind] {
			continue
		}
		switch kind {
		case "org.matrix.msc2762.send.state_event", "org.matrix.msc2762.receive.state_event",
			"org.matrix.msc2762.send.event", "org.matrix.msc2762.receive.event",
			"org.matrix.msc3819.send.to_device", "org.matrix.msc3819.receive.to_device",
			"org.matrix.msc4407.send.sticky_event", "org.matrix.msc4407.receive.sticky_event":
			// "…:<event type>", optionally with a state key after a second colon.
			typ, _, _ := strings.Cut(arg, "#") // a state key follows a #
			if !allowedTypes[typ] {
				continue
			}
		}
		out = append(out, cap)
	}
	return out
}

// Host speaks the widget API for one widget in one room.
type Host struct {
	WidgetID string
	// OnHangup is called when the widget says the user left the call.
	OnHangup func()
	// OnJoin is called when the widget says it joined the call: the moment to
	// set the microphone and camera the user wants to start with (SetMute).
	OnJoin func()
	// OnMute is told when the page's microphone or camera is turned on or off
	// (nil: not reported), by the user in the page or in answer to SetMute.
	OnMute func(audio, video *bool)
	// Logf, when set, is told of what was refused (a debug aid).
	Logf func(format string, args ...any)
	be   Backend

	mu      sync.Mutex
	next    int
	pending map[string]string // requests of ours awaiting the widget's answer: id → action
	granted []string
	ready   bool // the widget said its content loaded
}

// NewHost makes the host of the widget id, on the account behind be.
func NewHost(widgetID string, be Backend) *Host {
	return &Host{WidgetID: widgetID, be: be, pending: map[string]string{}}
}

// Granted are the capabilities approved so far.
func (h *Host) Granted() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.granted...)
}

// SetMute asks the widget to turn its microphone and/or camera on or off (nil:
// leave as it is). Post the result to it. The widget answers with how things
// are, which reaches OnMute.
func (h *Host) SetMute(audio, video *bool) []byte {
	data := map[string]any{}
	if audio != nil {
		data["audio_enabled"] = *audio
	}
	if video != nil {
		data["video_enabled"] = *video
	}
	return h.ask("io.element.device_mute", data)
}

// muteChanged reports the page's microphone and camera state ({audio_enabled,
// video_enabled}; a missing one is unchanged).
func (h *Host) muteChanged(data json.RawMessage) {
	if h.OnMute == nil {
		return
	}
	var d struct {
		Audio *bool `json:"audio_enabled"`
		Video *bool `json:"video_enabled"`
	}
	if json.Unmarshal(data, &d) == nil && (d.Audio != nil || d.Video != nil) {
		h.OnMute(d.Audio, d.Video)
	}
}

// Hangup is the message that asks the widget to leave the call: it removes the
// user's membership and tears down its media, then tells the host it is done
// (OnHangup). Post it to the widget.
func (h *Host) Hangup() []byte { return h.ask("im.vector.hangup", map[string]any{}) }

// Receives says the widget was granted hearing of this room event (JSON): the
// source offers it only those.
func (h *Host) Receives(event []byte) bool {
	return h.allows(event, "receive")
}

// ReceivesToDevice says the widget was granted hearing of this to-device
// message ({"type": …, …}).
func (h *Host) ReceivesToDevice(msg []byte) bool {
	var d struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(msg, &d) != nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, cap := range h.granted {
		kind, arg, _ := strings.Cut(cap, ":")
		if kind == "org.matrix.msc3819.receive.to_device" && arg == d.Type {
			return true
		}
	}
	return false
}

// Ready says the widget's content has loaded.
func (h *Host) Ready() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ready
}

// Handle takes one message the widget posted and returns the messages to post
// back, in order (JSON). A message that is not the widget API, or is for
// another widget, is ignored.
func (h *Host) Handle(ctx context.Context, raw []byte) [][]byte {
	var m Message
	if err := json.Unmarshal(raw, &m); err != nil || m.WidgetID != h.WidgetID {
		return nil
	}
	switch m.API {
	case "toWidget": // the widget answers a request of ours
		return h.answered(m)
	case "fromWidget":
		if len(m.Response) > 0 {
			return nil // an echo of our own answer
		}
		return h.request(ctx, m)
	}
	return nil
}

func (h *Host) request(ctx context.Context, m Message) [][]byte {
	var resp any
	var err error
	var after [][]byte
	switch m.Action {
	case "supported_api_versions":
		resp = map[string]any{"supported_versions": supportedVersions}
	case "content_loaded":
		resp = map[string]any{}
		h.mu.Lock()
		h.ready = true
		h.mu.Unlock()
		after = append(after, h.ask("capabilities", map[string]any{}))
	case "org.matrix.msc4515.get_rtc_transports":
		resp, err = h.guarded(capRTCTransports, func() (any, error) { return h.be.RTCTransports(ctx) })
	case "org.matrix.msc4533.rtc_livekit_get_token":
		resp, err = h.guarded(capLiveKitToken, func() (any, error) { return h.be.LiveKitToken(ctx, m.Data) })
	case "org.matrix.msc4533.rtc_livekit_delegate_delayed_leave":
		resp, err = h.guarded(capLiveKitLeave, func() (any, error) { return h.be.LiveKitDelayedLeave(ctx, m.Data) })
	case "get_openid":
		resp, err = h.be.OpenID(ctx)
	case "send_event":
		if !h.allows(m.Data, "send") {
			err = errString("not allowed")
			break
		}
		resp, err = h.be.SendEvent(ctx, m.Data)
	case "read_events", "org.matrix.msc2876.read_events":
		if !h.allows(m.Data, "receive") {
			err = errString("not allowed")
			break
		}
		resp, err = h.be.ReadEvents(ctx, m.Data)
	case "send_to_device", "org.matrix.msc3819.send_to_device":
		resp, err = h.be.SendToDevice(ctx, m.Data)
	case "update_delayed_event", "org.matrix.msc4157.update_delayed_event":
		resp, err = h.be.UpdateDelayedEvent(ctx, m.Data)
	case "watch_turn_servers", "town.robin.msc3846.watch_turn_servers":
		resp, err = h.be.TURNServers(ctx)
	case "unwatch_turn_servers", "town.robin.msc3846.unwatch_turn_servers":
		resp = map[string]any{}
	case "im.vector.hangup", "io.element.close":
		resp = map[string]any{}
		if h.OnHangup != nil {
			h.OnHangup()
		}
	case "io.element.device_mute": // the page tells whether its microphone and camera are on
		resp = map[string]any{}
		h.muteChanged(m.Data)
	case "io.element.join": // the page has joined the call (past its lobby, or none)
		resp = map[string]any{}
		if h.OnJoin != nil {
			h.OnJoin()
		}
	case "set_always_on_screen", "org.matrix.msc2931.navigate", "close_modal", "set_button_enabled",
		"io.element.tile_layout":
		resp = map[string]any{}
	default:
		err = errString("unsupported action " + m.Action)
	}
	if err != nil {
		resp = map[string]any{"error": map[string]any{"message": err.Error()}}
	}
	return append([][]byte{h.reply(m, resp)}, after...)
}

// guarded runs fn only when the capability was granted.
func (h *Host) guarded(capability string, fn func() (any, error)) (any, error) {
	h.mu.Lock()
	ok := false
	for _, g := range h.granted {
		if g == capability {
			ok = true
		}
	}
	h.mu.Unlock()
	if !ok {
		return nil, errString("not allowed")
	}
	return fn()
}

// allows says the request's event type is one that was granted for that
// direction ("send" or "receive").
func (h *Host) allows(data json.RawMessage, dir string) bool {
	var d struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(data, &d) != nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, cap := range h.granted {
		kind, arg, _ := strings.Cut(cap, ":")
		if kind != "org.matrix.msc2762."+dir+".state_event" && kind != "org.matrix.msc2762."+dir+".event" &&
			kind != "org.matrix.msc4407."+dir+".sticky_event" {
			continue
		}
		typ, _, _ := strings.Cut(arg, "#")
		if typ == d.Type {
			return true
		}
	}
	return false
}

func (h *Host) answered(m Message) [][]byte {
	h.mu.Lock()
	action, ok := h.pending[m.RequestID]
	delete(h.pending, m.RequestID)
	h.mu.Unlock()
	if ok && action == "io.element.device_mute" { // our request's answer: how things are now
		h.muteChanged(m.Response)
		return nil
	}
	if !ok || action != "capabilities" {
		return nil
	}
	var r struct {
		Capabilities []string `json:"capabilities"`
	}
	_ = json.Unmarshal(m.Response, &r)
	approved := Grant(r.Capabilities)
	h.mu.Lock()
	h.granted = approved
	h.mu.Unlock()
	if h.Logf != nil { // what the page wanted that it did not get, for the debug log
		got := map[string]bool{}
		for _, g := range approved {
			got[g] = true
		}
		for _, want := range r.Capabilities {
			if !got[want] {
				h.Logf("capability refused: %s", want)
			}
		}
	}
	return [][]byte{h.ask("notify_capabilities", map[string]any{"requested": r.Capabilities, "approved": approved})}
}

// ask is a request of ours to the widget; its answer comes back through Handle.
func (h *Host) ask(action string, data any) []byte {
	h.mu.Lock()
	h.next++
	id := "host-" + strconv.Itoa(h.next)
	h.pending[id] = action
	h.mu.Unlock()
	return h.encode(Message{API: "toWidget", RequestID: id, Action: action, WidgetID: h.WidgetID}, data, nil)
}

// Deliver is a room event, or a to-device message, for the widget: post the
// result to it. The widget only hears of what it asked to receive.
func (h *Host) Deliver(toDevice bool, event any) []byte {
	action := "send_event"
	if toDevice {
		action = "send_to_device"
	}
	return h.ask(action, event)
}

func (h *Host) reply(m Message, resp any) []byte {
	return h.encode(Message{API: m.API, RequestID: m.RequestID, Action: m.Action, WidgetID: h.WidgetID}, m.Data, resp)
}

func (h *Host) encode(m Message, data, response any) []byte {
	if response != nil {
		m.Response, _ = json.Marshal(response)
		if raw, ok := data.(json.RawMessage); ok {
			m.Data = raw
		}
	} else if data != nil {
		if raw, ok := data.(json.RawMessage); ok {
			m.Data = raw
		} else {
			m.Data, _ = json.Marshal(data)
		}
	}
	out, _ := json.Marshal(m)
	return out
}

type errString string

func (e errString) Error() string { return string(e) }
