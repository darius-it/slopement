package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
		StickyMS      int64           `json:"sticky_duration_ms"`
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
	extra := mautrix.ReqSendEvent{UnstableDelay: time.Duration(d.Delay) * time.Millisecond,
		UnstableStickyDuration: time.Duration(d.StickyMS) * time.Millisecond}
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
		b.s.callMu.Lock() // the call service wants the delay's length when handed the leave
		if b.s.callDelays == nil {
			b.s.callDelays = map[string]int64{}
		}
		b.s.callDelays[string(resp.UnstableDelayID)] = d.Delay
		b.s.callMu.Unlock()
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
	if d.StateKey == nil { // the room's recent events of a type: the call's encryption keys sent before the page joined
		return map[string]any{"events": b.recentEvents(ctx, d.Type, d.Limit)}, nil
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

// recentEvents are the room's latest events of a type, newest first, read from
// the server and decrypted when they were encrypted (the call's encryption keys
// are sent to the room, so the page joining later reads them here).
func (b *rtcBackend) recentEvents(ctx context.Context, typ string, limit int) []json.RawMessage {
	out := []json.RawMessage{}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	cli := b.s.cli
	resp, err := cli.Messages(ctx, b.room, "", "", mautrix.DirectionBackward, nil, 100)
	if err != nil {
		debugf("rtc: reading the timeline: %v", err)
		return out
	}
	for _, evt := range resp.Chunk {
		if evt.Type == event.EventEncrypted && b.s.helper != nil {
			if err := evt.Content.ParseRaw(evt.Type); err != nil && !errors.Is(err, event.ErrContentAlreadyParsed) {
				continue
			}
			dec, err := b.s.helper.Decrypt(ctx, evt)
			if err != nil {
				continue // locked: the page will not see it either
			}
			evt = dec
		}
		if evt.Type.Type != typ {
			continue
		}
		if raw, err := json.Marshal(evt); err == nil {
			out = append(out, raw)
			if len(out) >= limit {
				break
			}
		}
	}
	debugf("rtc: timeline read of %s: %d events", typ, len(out))
	return out
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

// RTCTransports answers the call page's "where is the media server": the
// homeserver's own list (MSC4143), else what its .well-known advertises.
func (b *rtcBackend) RTCTransports(ctx context.Context) (any, error) {
	var resp struct {
		Transports []map[string]any `json:"rtc_transports"`
	}
	cli := b.s.cli
	if err := cliGet(ctx, cli, cli.BuildClientURL("v1", "rtc", "transports"), &resp); err == nil && len(resp.Transports) > 0 {
		return map[string]any{"rtc_transports": resp.Transports}, nil
	}
	sup, err := b.s.CallSupport(ctx)
	if err != nil || !sup.LiveKit {
		return nil, errors.New("the server advertises no call transport")
	}
	return map[string]any{"rtc_transports": []map[string]any{{"type": "livekit", "livekit_service_url": sup.Service}}}, nil
}

func cliGet(ctx context.Context, cli *mautrix.Client, url string, out any) error {
	_, err := cli.MakeRequest(ctx, http.MethodGet, url, nil, out)
	return err
}

// rtcMember is who a call member claims to be, as the LiveKit service takes it.
func (b *rtcBackend) rtcMember(memberID string) map[string]any {
	return map[string]any{"id": memberID, "claimed_user_id": b.s.me.ID, "claimed_device_id": string(b.s.cli.DeviceID)}
}

// postService posts JSON to the LiveKit JWT service the server advertises (the
// widget names it in its request) and decodes its answer into out.
func postService(ctx context.Context, base, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(base, "/")+path, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// homeserverPost posts the page's request to the homeserver's MatrixRTC
// endpoint (/rtc/livekit/<name>, stable path then MSC4195's unstable one) and
// decodes the answer.
func (b *rtcBackend) homeserverPost(ctx context.Context, name string, body json.RawMessage, out any) error {
	cli := b.s.cli
	var err error
	for _, u := range []string{
		cli.BuildClientURL("v1", "rtc", "livekit", name),
		cli.BuildClientURL("unstable", "org.matrix.msc4195", "rtc", "livekit", name),
	} {
		if _, err = cli.MakeRequest(ctx, http.MethodPost, u, body, out); err == nil {
			return nil
		}
	}
	return err
}

// LiveKitToken asks the call service for the token that lets this member into
// the call's media room, proving who they are with an OpenID token.
func (b *rtcBackend) LiveKitToken(ctx context.Context, data json.RawMessage) (any, error) {
	var d struct {
		URL      string `json:"url"`
		RoomID   string `json:"room_id"`
		SlotID   string `json:"slot_id"`
		MemberID string `json:"member_id"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.URL == "" {
		return nil, errors.New("a token needs the service's address")
	}
	if id.RoomID(d.RoomID) != b.room {
		return nil, errors.New("the call page is limited to its own room")
	}
	// The homeserver's own endpoint (MSC4195) is the way: it knows its call
	// service, and the page's request is what it takes.
	var viaHS map[string]any
	if err := b.homeserverPost(ctx, "get_token", data, &viaHS); err == nil && viaHS["jwt"] != nil {
		debugf("rtc: token for %s from the homeserver", d.MemberID)
		return viaHS, nil
	} else if err != nil {
		debugf("rtc: the homeserver's get_token failed (%v): asking the service directly", err)
	}
	tok, err := b.s.cli.RequestOpenIDToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("openid: %w", err)
	}
	var out struct {
		URL string `json:"url"`
		JWT string `json:"jwt"`
	}
	err = postService(ctx, d.URL, "/get_token", map[string]any{
		"room_id": d.RoomID, "slot_id": d.SlotID, "openid_token": tok, "member": b.rtcMember(d.MemberID),
	}, &out)
	if err != nil {
		return nil, err
	}
	debugf("rtc: token for %s from %s (sfu %s)", d.MemberID, d.URL, out.URL)
	return map[string]any{"jwt": out.JWT, "url": out.URL}, nil
}

// LiveKitDelayedLeave hands the call service the delayed event that ends the
// member's call when they drop, so the service restarts it while they are
// connected to the media room.
func (b *rtcBackend) LiveKitDelayedLeave(ctx context.Context, data json.RawMessage) (any, error) {
	var d struct {
		URL      string `json:"url"`
		RoomID   string `json:"room_id"`
		SlotID   string `json:"slot_id"`
		MemberID string `json:"member_id"`
		DelayID  string `json:"delay_id"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.URL == "" || d.DelayID == "" {
		return nil, errors.New("a delayed leave needs the service and the delayed event")
	}
	if id.RoomID(d.RoomID) != b.room {
		return nil, errors.New("the call page is limited to its own room")
	}
	var viaHS map[string]any
	if err := b.homeserverPost(ctx, "delegate_delayed_leave", data, &viaHS); err == nil {
		debugf("rtc: delayed leave %s handed over by the homeserver", d.DelayID)
		return map[string]any{}, nil
	} else {
		debugf("rtc: the homeserver's delegate_delayed_leave failed (%v): asking the service directly", err)
	}
	tok, err := b.s.cli.RequestOpenIDToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("openid: %w", err)
	}
	b.s.callMu.Lock()
	timeout := b.s.callDelays[d.DelayID]
	b.s.callMu.Unlock()
	if timeout == 0 {
		timeout = 8000
	}
	var out map[string]any
	if err := postService(ctx, d.URL, "/delegate_delayed_leave", map[string]any{
		"room_id": d.RoomID, "slot_id": d.SlotID, "openid_token": tok, "member": b.rtcMember(d.MemberID),
		"delay_id": d.DelayID, "delay_timeout": timeout,
	}, &out); err != nil {
		return nil, err
	}
	return map[string]any{}, nil
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

// callMemberTypes are the state events that say someone is in a call.
var callMemberTypes = []string{"m.rtc.member", "org.matrix.msc3401.call.member", "m.call.member"}

// ClearCallMembership removes this account's call memberships from a room:
// what a call that was never left properly (an app closed or crashed, a
// delayed leave that never ran) keeps showing as "in a call" on every client.
// It sets each of the user's memberships to empty, as leaving does. It says how
// many it cleared; with thisDeviceOnly, just this device's. Off the UI thread.
func (s *Source) ClearCallMembership(ctx context.Context, room string, thisDeviceOnly bool) (int, error) {
	if s.cli == nil {
		return 0, errors.New("not signed in")
	}
	state, err := s.cli.State(ctx, id.RoomID(room))
	if err != nil {
		return 0, err
	}
	cleared := 0
	for _, typ := range callMemberTypes {
		for key, evt := range state[event.Type{Type: typ, Class: event.StateEventType}] {
			if evt == nil || evt.Sender.String() != s.me.ID || len(evt.Content.VeryRaw) == 0 || string(evt.Content.VeryRaw) == "{}" {
				continue
			}
			// After a call, only this device's own membership is ended: another
			// device of the user may be in a call of its own.
			if thisDeviceOnly && !strings.Contains(key, string(s.cli.DeviceID)) {
				continue
			}
			if _, err := s.cli.SendStateEvent(ctx, id.RoomID(room), event.Type{Type: typ, Class: event.StateEventType}, key, map[string]any{}); err != nil {
				return cleared, err
			}
			cleared++
		}
	}
	return cleared, nil
}

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
func (s *Source) WatchCall(room string, fn func(toDevice bool, raw json.RawMessage)) (stop func()) {
	s.callMu.Lock()
	if s.callWatch == nil {
		s.callWatch = map[string]func(bool, json.RawMessage){}
	}
	s.callWatch[room] = fn
	s.callMu.Unlock()
	return func() {
		s.callMu.Lock()
		delete(s.callWatch, room)
		s.callMu.Unlock()
	}
}

// callPeople is how many different people are in the room's call.
func (r *room) callPeople() int {
	seen := map[string]bool{}
	for _, who := range r.callMembers {
		seen[who] = true
	}
	return len(seen)
}

// noteCallMember keeps the room's call memberships: one with content is
// someone in the call, an emptied one (what leaving does) is not.
func (s *Source) noteCallMember(evt *event.Event) {
	if evt.StateKey == nil {
		return
	}
	known := false
	for _, t := range callMemberTypes {
		known = known || t == evt.Type.Type
	}
	if !known {
		return
	}
	raw := strings.TrimSpace(string(evt.Content.VeryRaw))
	in := raw != "" && raw != "{}"
	room, key, who := evt.RoomID.String(), evt.Type.Type+"|"+*evt.StateKey, evt.Sender.String()
	fromTimeline := evt.Mautrix.EventSource&event.SourceTimeline != 0
	ts := eventTime(evt)
	s.post(func() {
		r := s.roomOf(room)
		wasIn := len(r.callMembers) > 0
		if in {
			if r.callMembers == nil {
				r.callMembers = map[string]string{}
			}
			r.callMembers[key] = who
			if !wasIn && r.callStart.IsZero() {
				r.callStart = ts // the first one in: the call begins
			}
		} else {
			delete(r.callMembers, key)
		}
		// The last person left a call that was going: say so, once, in the chat,
		// with how long it lasted.
		if wasIn && len(r.callMembers) == 0 {
			var length time.Duration
			if !r.callStart.IsZero() && ts.After(r.callStart) {
				length = ts.Sub(r.callStart)
			}
			r.callStart = time.Time{}
			if fromTimeline {
				r.appendEvent(model.Event{ID: "call-ended:" + room + ":" + strconv.FormatInt(ts.UnixMilli(), 10), Room: room, Sender: who,
					Time: ts, Kind: model.EventSystem, Body: "The call ended", Call: &model.CallLine{Ended: true, Duration: length}}, "", false)
			}
		}
	})
}

// RefreshCall reads who is in a room's call once a run (the sync does not
// resend state it has already given, so a call begun before the app started
// would otherwise go unseen). UI thread; the answer arrives through the update.
func (s *Source) RefreshCall(room string) {
	r, ok := s.rooms[room]
	if !ok || r.callAsked || s.cli == nil {
		return
	}
	r.callAsked = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		state, err := s.cli.State(ctx, id.RoomID(room))
		if err != nil {
			s.post(func() {
				if r, ok := s.rooms[room]; ok {
					r.callAsked = false
				}
			})
			return
		}
		found := map[string]string{}
		var began time.Time // the earliest of them: when the call going now began
		for _, typ := range callMemberTypes {
			for key, evt := range state[event.Type{Type: typ, Class: event.StateEventType}] {
				if evt == nil || len(evt.Content.VeryRaw) == 0 || strings.TrimSpace(string(evt.Content.VeryRaw)) == "{}" {
					continue
				}
				found[typ+"|"+key] = evt.Sender.String()
				if t := eventTime(evt); began.IsZero() || t.Before(began) {
					began = t
				}
			}
		}
		s.post(func() {
			if r, ok := s.rooms[room]; ok {
				r.callMembers = found
				if len(found) > 0 && r.callStart.IsZero() {
					r.callStart = began
				}
				s.changed()
			}
		})
	}()
}

func (s *Source) onCallEvent(_ context.Context, evt *event.Event) {
	s.noteCallMember(evt)
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
		fn(false, raw)
	}
}
