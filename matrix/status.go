package matrix

import (
	"context"
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
	if s.cli == nil {
		return
	}
	msg := statusMessage(st)
	send := func(msg string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = s.cli.SetPresence(ctx, mautrix.ReqPresence{Presence: event.PresenceOnline, StatusMsg: msg})
	}
	go send(msg)
	if !st.Until.IsZero() && s.update != nil {
		if d := time.Until(st.Until); d > 0 {
			time.AfterFunc(d, func() {
				s.update(func() {
					if s.me.Status == st { // not replaced by another since
						s.me.Status = model.Status{}
						go send("")
						s.changed()
					}
				})
			})
		}
	}
	s.changed()
}
