package matrix

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// eventTime is when a Matrix event happened, from its server timestamp.
func eventTime(evt *event.Event) time.Time {
	return time.UnixMilli(evt.Timestamp)
}

// textEvent is an m.room.message as the model's timeline event: text,
// media, a reply, an edit's mention flags read out of it. Pure, for the
// tests.
//
// The body is the plain text one for now — the formatted body's rich
// Markdown round-trip lands with the media milestone.
func textEvent(me string, evt *event.Event) model.Event {
	c := evt.Content.AsMessage()
	e := model.Event{
		ID:     evt.ID.String(),
		Room:   evt.RoomID.String(),
		Sender: evt.Sender.String(),
		Time:   eventTime(evt),
		Body:   c.Body,
	}
	e.Formatted, e.Plain = formattedOf(c)
	if c.RelatesTo != nil && c.RelatesTo.GetNonFallbackReplyTo() != "" {
		e.Body = stripReplyFallback(e.Body) // an older client's "> <@who> quoted" lines
	}
	if c.Mentions != nil && c.Mentions.Has(id.UserID(me)) {
		e.Mentions = true
	}
	if rel := c.RelatesTo; rel != nil {
		// A reply in a thread points at the latest message of the thread as
		// a fallback for clients without threads: that is not a reply.
		if id := rel.GetNonFallbackReplyTo(); id != "" {
			e.ReplyTo = id.String()
		}
		if root := rel.GetThreadParent(); root != "" {
			e.ThreadRoot = root.String()
		}
	}
	switch {
	case evt.Type == event.EventSticker:
		// An m.sticker has no msgtype: its body is the words for those who
		// can't see it.
		e.Kind = model.EventMedia
		if m, ok := mediaOf(c); ok {
			m.Sticker = true
			if m.Mime == "image/gif" {
				m.Kind = model.MediaGIF
			}
			e.Media = []model.Media{m}
		}
	case c.MsgType == event.MsgImage, c.MsgType == event.MsgVideo, c.MsgType == event.MsgAudio, c.MsgType == event.MsgFile:
		e.Kind = model.EventMedia
		if m, ok := mediaOf(c); ok {
			e.Media = []model.Media{m}
		}
	case c.MsgType == event.MsgLocation:
		e.Kind = model.EventText
		if loc, ok := locationOf(evt, c); ok {
			e.Location = &loc
		}
	case c.MsgType == event.MsgEmote:
		e.Kind = model.EventEmote
	case c.MsgType == event.MsgNotice:
		e.Kind = model.EventNotice
	default:
		e.Kind = model.EventText
	}
	return e
}

// stripReplyFallback takes off the fallback a reply carried for clients that
// cannot show replies: lines that begin "> ", then one blank line.
func stripReplyFallback(body string) string {
	if !strings.HasPrefix(body, "> ") {
		return body
	}
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) && strings.HasPrefix(lines[i], ">") {
		i++
	}
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return body // nothing but quote: keep it, rather than show nothing
	}
	return strings.Join(lines[i:], "\n")
}

// locationOf reads a shared place out of an m.location: its geo_uri (or, in
// older clients, org.matrix.msc3488.location's uri) and the words with it.
// Pure, for the tests.
func locationOf(evt *event.Event, c *event.MessageEventContent) (model.Location, bool) {
	uri, words := c.GeoURI, ""
	if old, ok := contentMap(evt)["org.matrix.msc3488.location"].(map[string]any); ok {
		if uri == "" {
			uri, _ = old["uri"].(string)
		}
		words, _ = old["description"].(string)
	}
	lat, lon, ok := parseGeo(uri)
	if !ok {
		return model.Location{}, false
	}
	if strings.TrimSpace(words) == "" {
		words = locationWords(c.Body)
	}
	return model.Location{Lat: lat, Lon: lon, Description: strings.TrimSpace(words)}, true
}

// parseGeo reads "geo:48.2,16.3" (RFC 5870, with its ";u=" parameters and an
// optional altitude) into a latitude and a longitude.
func parseGeo(uri string) (lat, lon float64, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(uri), "geo:")
	if !found {
		return 0, 0, false
	}
	rest, _, _ = strings.Cut(rest, ";")
	parts := strings.Split(rest, ",")
	if len(parts) < 2 {
		return 0, 0, false
	}
	var err1, err2 error
	lat, err1 = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, err2 = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}

// locationWords is what the sender called the place: the body of an
// m.location, unless it is only the fallback clients write ("Location",
// "Location: 48.2,16.3", the coordinates, or a geo: link).
func locationWords(body string) string {
	b := strings.TrimSpace(body)
	if len(b) >= 8 && strings.EqualFold(b[:8], "location") {
		tail := b[8:]
		rest := strings.TrimSpace(strings.TrimLeft(tail, ": "))
		switch {
		case rest == "" || looksLikeCoords(rest):
			return ""
		case strings.HasPrefix(tail, ":"):
			return rest // "Location: Brandenburger Tor"
		}
		// otherwise the sender's own words that happen to start so
	}
	if strings.HasPrefix(b, "geo:") || looksLikeCoords(b) {
		return ""
	}
	return b
}

func looksLikeCoords(s string) bool {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts[:2] {
		if _, err := strconv.ParseFloat(p, 64); err != nil {
			return false
		}
	}
	return true
}

// mediaOf is one attachment of a message: its mxc URL and size, never
// cropped — the shape comes along.
func mediaOf(c *event.MessageEventContent) (model.Media, bool) {
	m := model.Media{Kind: model.MediaImage, Name: c.FileName}
	if m.Name == "" {
		m.Name = c.Body
	}
	if c.URL != "" {
		m.URL = string(c.URL)
	} else if c.File != nil {
		m.URL = string(c.File.URL) // an encrypted attachment: the key rides along
		if b, err := json.Marshal(c.File); err == nil {
			m.Secret = string(b)
		}
	} else {
		return model.Media{}, false
	}
	if c.Info != nil {
		m.Mime = c.Info.MimeType
		m.W, m.H = c.Info.Width, c.Info.Height
		m.Size = int64(c.Info.Size)
		m.Length = time.Duration(c.Info.Duration) * time.Millisecond
	}
	switch c.MsgType {
	case event.MsgImage:
		if m.Mime == "image/gif" {
			m.Kind = model.MediaGIF // it animates: the app plays it
		}
	case event.MsgVideo:
		m.Kind = model.MediaVideo
		if c.Info != nil && c.Info.MauGIF {
			m.Kind = model.MediaGIF
		}
	case event.MsgAudio:
		m.Kind, m.Audio = model.MediaFile, true
	case event.MsgFile:
		m.Kind = model.MediaFile
	}
	return m, true
}

// memberLine is a membership change as the model's quiet system line:
// "Mira joined", "You left". A join over a join is only a profile change:
// not a line. Pure, for the tests; me is the user's own MXID.
func memberLine(evt *event.Event, me string) (model.Event, bool) {
	c := evt.Content.AsMember()
	if c == nil || evt.StateKey == nil {
		return model.Event{}, false
	}
	var prev event.Membership
	if pc := evt.Unsigned.PrevContent; pc != nil {
		_ = pc.ParseRaw(event.StateMember)
		if parsed := pc.AsMember(); parsed != nil {
			prev = parsed.Membership
		}
	}
	if c.Membership == event.MembershipJoin && prev == event.MembershipJoin {
		return model.Event{}, false // only a profile change: nothing to say
	}
	you := *evt.StateKey == me
	who := c.Displayname
	if who == "" {
		who = mxidName(*evt.StateKey)
	}
	if you {
		who = "You"
	}
	var what string
	switch c.Membership {
	case event.MembershipJoin:
		what = "joined"
	case event.MembershipLeave:
		what = "left"
	case event.MembershipBan:
		what = "was banned"
		if you {
			what = "were banned"
		}
	case event.MembershipInvite:
		what = "was invited"
		if you {
			what = "were invited"
		}
	case event.MembershipKnock:
		what = "knocked"
	default:
		return model.Event{}, false
	}
	return model.Event{
		ID:     evt.ID.String(),
		Room:   evt.RoomID.String(),
		Sender: evt.Sender.String(),
		Time:   eventTime(evt),
		Kind:   model.EventSystem,
		Body:   who + " " + what,
		// join, leave, ban, invite or knock: the app can hide the first two.
		Membership: string(c.Membership),
	}, true
}

// undecryptable is an m.room.encrypted that would not decrypt, as the
// model's "unable to decrypt" event.
func undecryptable(evt *event.Event) model.Event {
	return model.Event{
		ID:     evt.ID.String(),
		Room:   evt.RoomID.String(),
		Sender: evt.Sender.String(),
		Time:   eventTime(evt),
		Kind:   model.EventUndecryptable,
	}
}

// formattedOf is how a message is to be shown: its HTML body when the
// sender gave one (org.matrix.custom.html), else it is plain text, to be
// shown as typed.
func formattedOf(c *event.MessageEventContent) (formatted string, plain bool) {
	if c.Format == event.FormatHTML && strings.TrimSpace(c.FormattedBody) != "" {
		return c.FormattedBody, false
	}
	return "", true
}

// editOf reports whether a message is an edit, and of which event, with
// its new body and the new formatted body, if any.
func editOf(c *event.MessageEventContent) (target, body, formatted string, ok bool) {
	if c.NewContent == nil || c.RelatesTo == nil || c.RelatesTo.Type != event.RelReplace {
		return "", "", "", false
	}
	formatted, _ = formattedOf(c.NewContent)
	return c.RelatesTo.EventID.String(), c.NewContent.Body, formatted, true
}

// reactionOf reports whether a message is a reaction, on which event and
// with which key.
func reactionOf(c *event.MessageEventContent) (target, key string, ok bool) {
	if c.RelatesTo == nil || c.RelatesTo.Type != event.RelAnnotation {
		return "", "", false
	}
	return c.RelatesTo.EventID.String(), c.RelatesTo.Key, true
}

// nameFor is what the lists call a sender: their display name, else the
// local part of their MXID.
func nameFor(name, mxid string) string {
	if name != "" {
		return name
	}
	return mxidName(mxid)
}
