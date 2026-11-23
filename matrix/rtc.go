package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/calls"
	"slopement/model"
)

// CallSupport asks what the account's homeserver offers for calls: a LiveKit
// service for MatrixRTC (advertised in .well-known as org.matrix.msc4143.rtc_foci)
// and delayed events (MSC4140), which a call's membership needs to expire when
// the app dies. Off the UI thread.
func (s *Source) CallSupport(ctx context.Context) (model.CallSupport, error) {
	var out model.CallSupport
	if s.cli == nil {
		return out, errors.New("not signed in")
	}
	_, server, _ := strings.Cut(s.me.ID, ":")
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, "https://"+server+"/.well-known/matrix/client", nil)
	if err != nil {
		return out, err
	}
	if resp, err := http.DefaultClient.Do(req); err == nil {
		defer resp.Body.Close()
		var wk struct {
			Foci []struct {
				Type string `json:"type"`
				URL  string `json:"livekit_service_url"`
			} `json:"org.matrix.msc4143.rtc_foci"`
		}
		if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&wk) == nil {
			for _, f := range wk.Foci {
				if f.Type == "livekit" && f.URL != "" {
					out.LiveKit, out.Service = true, f.URL
					break
				}
			}
		}
	}
	if v, err := s.cli.Versions(cctx); err == nil {
		out.DelayedEvents = v.UnstableFeatures["org.matrix.msc4140"]
	} else {
		return out, err
	}
	return out, nil
}

// CallBackend is the Matrix side of a call page in a room: what the widget
// API's requests do on this account.
func (s *Source) CallBackend(room string) calls.Backend {
	return &rtcBackend{s: s, room: id.RoomID(room)}
}

// CallIdentity is who and where the call page runs as.
func (s *Source) CallIdentity() model.CallIdentity {
	var dev, base string
	if s.cli != nil {
		dev, base = string(s.cli.DeviceID), s.cli.HomeserverURL.String()
	}
	return model.CallIdentity{UserID: s.me.ID, DeviceID: dev, BaseURL: base}
}

type rtcBackend struct {
	s    *Source
	room id.RoomID
}

func (b *rtcBackend) SendEvent(ctx context.Context, data json.RawMessage) (any, error) {
	var d struct {
		Type          string          `json:"type"`
		StateKey      *string         `json:"state_key"`
		Content       json.RawMessage `json:"content"`
		Delay         int64           `json:"delay"` // ms
		ParentDelayID string          `json:"parent_delay_id"`
		RoomID        string          `json:"room_id"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	room := b.room
	if d.RoomID != "" && id.RoomID(d.RoomID) != b.room {
		return nil, errors.New("the call page is limited to its own room")
	}
	cli := b.s.cli
	// A delayed event is the page's membership expiring on its own; a
	// restart of an earlier delay is UpdateDelayedEvent's.
	extra := mautrix.ReqSendEvent{UnstableDelay: time.Duration(d.Delay) * time.Millisecond}
	var content any = d.Content
	var resp *mautrix.RespSendEvent
	var err error
	if d.StateKey != nil {
		resp, err = cli.SendStateEvent(ctx, room, event.Type{Type: d.Type, Class: event.StateEventType}, *d.StateKey, content, extra)
	} else {
		resp, err = cli.SendMessageEvent(ctx, room, event.Type{Type: d.Type, Class: event.MessageEventType}, content, extra)
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{"room_id": string(room)}
	if resp.UnstableDelayID != "" {
		out["delay_id"] = string(resp.UnstableDelayID)
	} else {
		out["event_id"] = string(resp.EventID)
	}
	return out, nil
}

func (b *rtcBackend) ReadEvents(ctx context.Context, data json.RawMessage) (any, error) {
	var d struct {
		Type     string `json:"type"`
		StateKey any    `json:"state_key"` // a string, or true for every key
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	events := []json.RawMessage{}
	if d.StateKey == nil { // timeline events: the call page reads none of them
		return map[string]any{"events": events}, nil
	}
	state, err := b.s.cli.State(ctx, b.room)
	if err != nil {
		return nil, err
	}
	typ := event.Type{Type: d.Type, Class: event.StateEventType}
	for key, evt := range state[typ] {
		if k, ok := d.StateKey.(string); ok && k != key {
			continue
		}
		raw, err := json.Marshal(evt)
		if err == nil {
			events = append(events, raw)
		}
	}
	return map[string]any{"events": events}, nil
}

func (b *rtcBackend) SendToDevice(ctx context.Context, data json.RawMessage) (any, error) {
	var d struct {
		Type      string                                `json:"type"`
		Encrypted bool                                  `json:"encrypted"`
		Messages  map[string]map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	req := &mautrix.ReqSendToDevice{Messages: map[id.UserID]map[id.DeviceID]*event.Content{}}
	for user, devs := range d.Messages {
		req.Messages[id.UserID(user)] = map[id.DeviceID]*event.Content{}
		for dev, raw := range devs {
			if dev == "*" {
				return nil, errors.New("to every device of a user is not supported yet")
			}
			req.Messages[id.UserID(user)][id.DeviceID(dev)] = &event.Content{Raw: mustMap(raw)}
		}
	}
	typ := event.Type{Type: d.Type, Class: event.ToDeviceEventType}
	if d.Encrypted {
		if b.s.helper == nil || b.s.helper.Machine() == nil {
			return nil, errors.New("encryption is not ready")
		}
		enc, err := b.s.helper.Machine().EncryptToDevices(ctx, typ, req)
		if err != nil {
			return nil, fmt.Errorf("encrypting: %w", err)
		}
		req, typ = enc, event.ToDeviceEncrypted
	}
	if _, err := b.s.cli.SendToDevice(ctx, typ, req); err != nil {
		return nil, err
	}
	return map[string]any{}, nil
}

func (b *rtcBackend) UpdateDelayedEvent(ctx context.Context, data json.RawMessage) (any, error) {
	var d struct {
		DelayID string `json:"delay_id"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	_, err := b.s.cli.UpdateDelayedEvent(ctx, &mautrix.ReqUpdateDelayedEvent{DelayID: id.DelayID(d.DelayID), Action: event.DelayAction(d.Action)})
	if err != nil {
		return nil, err
	}
	return map[string]any{}, nil
}

func (b *rtcBackend) OpenID(ctx context.Context) (any, error) {
	t, err := b.s.cli.RequestOpenIDToken(ctx)
	if err != nil {
		return map[string]any{"state": "blocked"}, nil
	}
	return map[string]any{"state": "allowed", "access_token": t.AccessToken, "expires_in": t.ExpiresIn,
		"matrix_server_name": t.MatrixServerName, "token_type": t.TokenType}, nil
}

func (b *rtcBackend) TURNServers(ctx context.Context) (any, error) {
	return map[string]any{}, nil // media goes through LiveKit; TURN is the old Jitsi/1:1 way
}

// mustMap reads a JSON object into a map (an empty one when it is not one).
func mustMap(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m
}

var _ calls.Backend = (*rtcBackend)(nil)

// callTypes are the room events a call page is told of as they arrive, when it
// was granted them (calls.Grant): the call's membership and its signals.
var callTypes = map[string]bool{
	"m.rtc.member": true, "org.matrix.msc3401.call.member": true, "org.matrix.msc3401.call": true,
	"m.call.member": true, "m.rtc.notification": true, "org.matrix.msc4075.rtc.notification": true,
	"m.room.member": true, "m.room.encryption": true, "m.room.power_levels": true,
	"io.element.call.encryption_keys": true, "io.element.call.reaction": true, "m.reaction": true,
	"m.rtc.decline": true, "org.matrix.msc4310.rtc.decline": true,
}

// WatchCall tells fn of the call-related events of a room as they arrive (one
// watch per room; a new one replaces the old), until stop is called. fn runs
// on the sync goroutine and must not block.
func (s *Source) WatchCall(room string, fn func(raw json.RawMessage)) (stop func()) {
	s.callMu.Lock()
	if s.callWatch == nil {
		s.callWatch = map[string]func(json.RawMessage){}
	}
	s.callWatch[room] = fn
	s.callMu.Unlock()
	return func() {
		s.callMu.Lock()
		delete(s.callWatch, room)
		s.callMu.Unlock()
	}
}

func (s *Source) onCallEvent(_ context.Context, evt *event.Event) {
	if !callTypes[evt.Type.Type] || evt.RoomID == "" {
		return
	}
	s.callMu.Lock()
	fn := s.callWatch[evt.RoomID.String()]
	s.callMu.Unlock()
	if fn == nil {
		return
	}
	if raw, err := json.Marshal(evt); err == nil {
		fn(raw)
	}
}
