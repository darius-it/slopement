package matrix

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// Forward sends a copy of a message to another room: its content as the
// sender wrote it (words, HTML, attachment — an attachment is the same
// uploaded file, so nothing is uploaded again), without what ties it to its
// old place: reply, thread, edit, mentions. The message is read from the
// server (decrypted if it is encrypted and this device has the key), and
// sending encrypts for the target room as any message.
func (s *Source) Forward(room, eventID, to string, done func(err string)) {
	if s.cli == nil {
		done("Not signed in.")
		return
	}
	if _, ok := s.rooms[to]; !ok {
		done("That chat is not available.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		fail := func(why string) { s.post(func() { done(why) }) }
		evt, err := s.cli.GetEvent(ctx, id.RoomID(room), id.EventID(eventID))
		if err != nil {
			fail("Could not read the message: " + plain(err))
			return
		}
		evt.RoomID = id.RoomID(room)
		_ = evt.Content.ParseRaw(evt.Type)
		if evt.Type == event.EventEncrypted {
			if s.helper == nil {
				fail("This session cannot read encrypted messages.")
				return
			}
			dec := s.decryptFetched(ctx, evt)
			if dec == nil {
				fail("This device has no key for that message yet.")
				return
			}
			evt = dec
		}
		if evt.Type != event.EventMessage && evt.Type != event.EventSticker {
			fail("That can't be forwarded.")
			return
		}
		content := forwardContent(contentMap(evt))
		if content == nil {
			fail("That message has nothing to forward.")
			return
		}
		if _, err := s.cli.SendMessageEvent(ctx, id.RoomID(to), evt.Type, content); err != nil {
			fail("Not sent: " + plain(err))
			return
		}
		s.post(func() { done("") })
	}()
}

// forwardContent is a message's content with what ties it to its place taken
// off: the reply or thread it was in, the edit it was, who it mentioned.
func forwardContent(c map[string]any) map[string]any {
	if len(c) == 0 {
		return nil
	}
	// A reply carries its quote as a fallback in its words: it goes with the reply.
	rel, _ := c["m.relates_to"].(map[string]any)
	_, wasReply := rel["m.in_reply_to"]
	out := make(map[string]any, len(c))
	for k, v := range c {
		switch k {
		case "m.relates_to", "m.new_content", "m.mentions":
			continue
		}
		out[k] = v
	}
	if wasReply {
		if f, ok := out["formatted_body"].(string); ok {
			out["formatted_body"] = stripMxReply(f)
		}
		if b, ok := out["body"].(string); ok {
			out["body"] = stripReplyFallback(b)
		}
	}
	if _, err := json.Marshal(out); err != nil {
		return nil
	}
	return out
}

// stripMxReply takes the <mx-reply>…</mx-reply> fallback off an HTML body.
func stripMxReply(html string) string {
	const open, close = "<mx-reply>", "</mx-reply>"
	for {
		i := strings.Index(html, open)
		if i < 0 {
			return html
		}
		j := strings.Index(html[i:], close)
		if j < 0 {
			return html
		}
		html = html[:i] + html[i+j+len(close):]
	}
}

var _ model.Forwarder = (*Source)(nil)
