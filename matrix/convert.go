package matrix

import (
	"encoding/json"
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
	default:
		e.Kind = model.EventText
	}
	return e
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
// "Mira joined", "Alex left". Pure, for the tests.
func memberLine(evt *event.Event) (model.Event, bool) {
	c := evt.Content.AsMember()
	if c == nil || evt.StateKey == nil {
		return model.Event{}, false
	}
	who := c.Displayname
	if who == "" && evt.StateKey != nil {
		who = mxidName(*evt.StateKey)
	}
	var what string
	switch c.Membership {
	case event.MembershipJoin:
		what = "joined"
	case event.MembershipLeave:
		what = "left"
	case event.MembershipBan:
		what = "was banned"
	case event.MembershipInvite:
		what = "was invited"
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
