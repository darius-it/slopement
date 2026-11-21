package matrix

import (
	"encoding/json"
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
	if c.Mentions != nil && c.Mentions.Has(id.UserID(me)) {
		e.Mentions = true
	}
	if rel := c.RelatesTo; rel != nil {
		if id := rel.GetReplyTo(); id != "" {
			e.ReplyTo = id.String()
		}
	}
	switch c.MsgType {
	case event.MsgImage, event.MsgVideo, event.MsgAudio, event.MsgFile:
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
		m.W, m.H = c.Info.Width, c.Info.Height
		m.Size = int64(c.Info.Size)
		m.Length = time.Duration(c.Info.Duration) * time.Millisecond
	}
	switch c.MsgType {
	case event.MsgVideo:
		m.Kind = model.MediaVideo
		if c.Info != nil && c.Info.MauGIF {
			m.Kind = model.MediaGIF
		}
	case event.MsgAudio:
		m.Kind = model.MediaFile
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

// editOf reports whether a message is an edit, and of which event, with
// its new body.
func editOf(c *event.MessageEventContent) (target, body string, ok bool) {
	if c.NewContent == nil || c.RelatesTo == nil || c.RelatesTo.Type != event.RelReplace {
		return "", "", false
	}
	return c.RelatesTo.EventID.String(), c.NewContent.Body, true
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
