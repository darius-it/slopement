package matrix

import (
	"encoding/json"
	"fmt"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// Converted is what one raw event comes to in the timeline, as the sync
// handlers would make it: an event of its own, or a change to another one.
type Converted struct {
	Event model.Event // the timeline event ("" ID when the raw event is only a change)
	// EditOf is the event an edit rewrites, with its new body and HTML.
	EditOf, EditBody, EditFormatted string
	// ReactionTo and ReactionKey, for a reaction.
	ReactionTo, ReactionKey string
	Poll                    *model.Poll
	// Skipped says why the event shows as nothing (a redaction, a type not shown).
	Skipped string
}

// ConvertEvent runs one event, as the server sends it (JSON), through the
// conversions the sync handlers use. It is how the compatibility fixtures
// (testdata/events) are rendered, with no server.
func ConvertEvent(me string, raw []byte) (Converted, error) {
	var evt event.Event
	if err := json.Unmarshal(raw, &evt); err != nil {
		return Converted{}, fmt.Errorf("not an event: %w", err)
	}
	// Poll events have no parsed form: they are read from the raw content.
	if err := evt.Content.ParseRaw(evt.Type); err != nil && !isPollType(evt.Type) {
		return Converted{}, fmt.Errorf("content of %s: %w", evt.Type.Type, err)
	}
	switch {
	case evt.Type == event.EventMessage || evt.Type == event.EventSticker:
		c := evt.Content.AsMessage()
		if c == nil || (c.MsgType == "" && c.Body == "" && c.NewContent == nil && evt.Type != event.EventSticker) {
			return Converted{Skipped: "redacted"}, nil
		}
		if target, body, formatted, ok := editOf(c); ok {
			return Converted{EditOf: target, EditBody: body, EditFormatted: formatted}, nil
		}
		return Converted{Event: textEvent(me, &evt)}, nil
	case evt.Type == event.EventReaction:
		rel := evt.Content.AsReaction().RelatesTo
		return Converted{ReactionTo: rel.EventID.String(), ReactionKey: rel.Key}, nil
	case evt.Type == event.StateMember:
		if e, ok := memberLine(&evt, me); ok {
			return Converted{Event: e}, nil
		}
		return Converted{Skipped: "no line"}, nil
	case isPollType(evt.Type):
		p, _, ok := parsePollStart(contentMap(&evt))
		if !ok {
			return Converted{Skipped: "not a poll start"}, nil
		}
		e := model.Event{ID: evt.ID.String(), Room: evt.RoomID.String(), Sender: evt.Sender.String(),
			Time: eventTime(&evt), Kind: model.EventPoll, Poll: &p}
		return Converted{Event: e, Poll: &p}, nil
	}
	return Converted{Skipped: "type " + evt.Type.Type + " is not shown"}, nil
}
