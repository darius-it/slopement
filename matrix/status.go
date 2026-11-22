package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// A person's status, in Matrix terms, is their presence's status message
// (status_msg): free text that FluffyChat and others set, with an emoji at
// its start when the person chose one. The app shows it as emoji and text;
// SetStatus writes it back the same way, so every client agrees.

// isEmojiRune reports a rune that can be part of an emoji: a pictographic
// symbol, a joiner, a variation selector, a skin tone, a flag half or the
// keycap mark.
func isEmojiRune(r rune) bool {
	switch {
	case r == 0x200D, r == 0xFE0F, r == 0x20E3:
		return true
	case r >= 0x1F1E6 && r <= 0x1F1FF, r >= 0x1F3FB && r <= 0x1F3FF:
		return true
	}
	return unicode.Is(unicode.So, r)
}

// parseStatus splits a status message into its emoji and its text: a first
// word made only of emoji is the emoji ("🎧 Focusing" → 🎧, "Focusing"), a
// message that is only emoji is only the emoji, and anything else is text.
func parseStatus(msg string) model.Status {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return model.Status{}
	}
	first, rest, _ := strings.Cut(msg, " ")
	for _, r := range first {
		if !isEmojiRune(r) {
			return model.Status{Text: msg}
		}
	}
	return model.Status{Emoji: first, Text: strings.TrimSpace(rest)}
}

// statusMessage is a status as the status message that carries it.
func statusMessage(st model.Status) string {
	return strings.TrimSpace(st.Emoji + " " + st.Text)
}

// setPresence records what is known of someone's presence and status. UI
// thread. A presence event carries the whole state, so no message means no
// status.
func (s *Source) setPresence(who, name string, presence model.Presence, msg string, lastActiveAgo int64) {
	status := parseStatus(msg)
	var seen time.Time
	if lastActiveAgo > 0 {
		seen = time.Now().Add(-time.Duration(lastActiveAgo) * time.Millisecond)
	}
	if who == s.me.ID {
		s.me.Presence = presence
		s.me.Status = status
		s.reconcileStatus() // the server's copy has no end time, and may have forgotten it
		return
	}
	p, ok := s.people[who]
	if !ok {
		p = &model.Person{ID: who, Name: name, Contact: true}
		s.people[who] = p
	}
	p.Presence, p.Status = presence, status
	if !seen.IsZero() {
		p.LastSeen = seen
	}
	s.dirtyMeta = true
	s.schedulePersist()
}

func presenceOf(p event.Presence) model.Presence {
	switch p {
	case event.PresenceOnline:
		return model.Online
	case event.PresenceUnavailable:
		return model.Away
	}
	return model.Offline
}

// fetchPresence asks the homeserver for the presence and status of the
// people who matter most — the other person of each DM, and the user — at
// the start. A resumed sync only sends what changed since, and a status that
// was set long ago would never arrive. Quiet when the server has presence
// off (it answers no).
func (s *Source) fetchPresence(ctx context.Context) {
	if s.cli == nil {
		return
	}
	done := make(chan []string, 1)
	s.update(func() {
		var ids []string
		seen := map[string]bool{s.me.ID: true}
		for _, r := range s.rooms {
			if r.dm && r.partner != "" && !seen[r.partner] {
				seen[r.partner] = true
				ids = append(ids, r.partner)
			}
		}
		done <- append(ids, s.me.ID)
	})
	var ids []string
	select {
	case ids = <-done:
	case <-ctx.Done():
		return
	}
	for _, who := range ids {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		resp, err := s.cli.GetPresence(c, id.UserID(who))
		cancel()
		if err != nil || resp == nil {
			if isForbidden(err) {
				return // presence is off on this server: no point asking for the rest
			}
			continue
		}
		who, resp := who, resp
		s.post(func() {
			name := ""
			if p, ok := s.people[who]; ok {
				name = p.Name
			}
			s.setPresence(who, name, presenceOf(resp.Presence), resp.StatusMsg, int64(resp.LastActiveAgo))
		})
	}
}

// isForbidden reports a homeserver refusing the request outright (403, or
// an M_FORBIDDEN answer).
func isForbidden(err error) bool {
	var h mautrix.HTTPError
	if errors.As(err, &h) && h.Response != nil {
		return h.Response.StatusCode == 403
	}
	return false
}

// SetStatus sets the user's own status: it shows at once and goes out as the
// presence's status message. One that ends (Until) clears itself, here and
// on the server.
func (s *Source) SetStatus(st model.Status) {
	s.me.Status = st
	s.savedStatus = st
	if s.cli != nil {
		go s.sendStatus(statusMessage(st))
		go s.saveStatus(st)
	}
	s.scheduleStatusEnd()
	s.changed()
}

// sendStatus puts the status message in the user's presence.
func (s *Source) sendStatus(msg string) {
	if s.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = s.cli.SetPresence(ctx, mautrix.ReqPresence{Presence: event.PresenceOnline, StatusMsg: msg})
}

// The user's own status is also kept in their account data, with the moment
// it ends: the homeserver's presence carries only the words, and may drop
// them (when the user goes offline, say), so a restart would otherwise lose
// both the status and its end — "Today" would be gone, or never end.

const statusData = "com.slopement.status"

var statusType = event.Type{Type: statusData, Class: event.AccountDataEventType}

type storedStatus struct {
	Emoji string `json:"emoji,omitempty"`
	Text  string `json:"text,omitempty"`
	Until int64  `json:"until,omitempty"` // Unix milliseconds; 0 for never
}

func (s *Source) saveStatus(st model.Status) {
	c := storedStatus{Emoji: st.Emoji, Text: st.Text}
	if !st.Until.IsZero() {
		c.Until = st.Until.UnixMilli()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = s.cli.SetAccountData(ctx, statusData, c)
}

// onStatusData is the saved status as the account has it.
func (s *Source) onStatusData(ctx context.Context, evt *event.Event) {
	var c storedStatus
	if json.Unmarshal(evt.Content.VeryRaw, &c) != nil {
		return
	}
	st := model.Status{Emoji: c.Emoji, Text: c.Text}
	if c.Until > 0 {
		st.Until = time.UnixMilli(c.Until)
	}
	s.post(func() {
		s.savedStatus = st
		s.reconcileStatus()
	})
}

// reconcileStatus makes the status shown agree with the one the user set: it
// is brought back when the server forgot it, takes its end time, and ends
// when that has passed (also across a restart). UI thread.
func (s *Source) reconcileStatus() {
	sv, cur := s.savedStatus, s.me.Status
	if sv.Emoji == "" && sv.Text == "" {
		return // none saved
	}
	same := statusMessage(sv) == statusMessage(cur)
	empty := cur.Emoji == "" && cur.Text == ""
	seen := s.statusSeen
	s.statusSeen = true // from now on, an empty status is the user's doing
	switch {
	case !sv.Until.IsZero() && !time.Now().Before(sv.Until):
		// Ended while the app was closed.
		if same || empty {
			s.me.Status, s.savedStatus = model.Status{}, model.Status{}
			if s.cli != nil {
				go s.sendStatus("")
				go s.saveStatus(model.Status{})
			}
			s.changed()
		}
		return
	case empty && seen:
		// Cleared by the user from another client while this one runs: theirs
		// is the choice.
		s.savedStatus = model.Status{}
		if s.cli != nil {
			go s.saveStatus(model.Status{})
		}
		return
	case empty:
		// Once at the start: the server forgot it. Put it back, and say it again.
		s.me.Status = sv
		if s.cli != nil {
			go s.sendStatus(statusMessage(sv))
		}
		s.changed()
	case same && s.me.Status.Until != sv.Until:
		s.me.Status.Until = sv.Until // the server's copy has no end
		s.changed()
	}
	s.scheduleStatusEnd()
}

// scheduleStatusEnd clears the status when its end comes (once at a time).
func (s *Source) scheduleStatusEnd() {
	if s.statusTimer != nil {
		s.statusTimer.Stop()
		s.statusTimer = nil
	}
	st := s.me.Status
	if st.Until.IsZero() || s.update == nil {
		return
	}
	d := time.Until(st.Until)
	if d <= 0 {
		return
	}
	s.statusTimer = time.AfterFunc(d, func() {
		s.update(func() {
			if s.me.Status == st { // not replaced by another since
				s.me.Status, s.savedStatus = model.Status{}, model.Status{}
				if s.cli != nil {
					go s.sendStatus("")
					go s.saveStatus(model.Status{})
				}
				s.changed()
			}
		})
	})
}
