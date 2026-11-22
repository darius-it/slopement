package matrix

import (
	"time"

	"slopement/model"
)

// notifyWindow is how recent a message must be to notify: what a sync
// brings after the app was shut for a while is unread, but not news.
const notifyWindow = 2 * time.Minute

// OnNotify gives the source the function that hears of messages that may
// notify (UI thread).
func (s *Source) OnNotify(fn func(model.Notice)) { s.notify = fn }

var _ model.Notifier = (*Source)(nil)

// noticeOf says whether a message just added to a room is one to notify of
// and what it says: live, from someone else, readable, in a room that is
// not muted, and a chat with a person or a mention of the user. A message
// that is told of once (an echo, or a late decrypt arriving again) is not
// told of twice. UI thread.
func (s *Source) noticeOf(r *room, e model.Event, senderName string) {
	if s.notify == nil || e.Sender == s.me.ID || r.muted || s.ignored[e.Sender] || e.Kind == model.EventUndecryptable ||
		e.Kind == model.EventSystem || time.Since(e.Time) > notifyWindow {
		return
	}
	// The room's level: the user's own choice, else a chat with a person
	// notifies for everything and a room for mentions.
	switch r.notify {
	case model.NotifyAll:
	case model.NotifyMentions:
		if !e.Mentions {
			return
		}
	default:
		if !r.dm && !e.Mentions {
			return
		}
	}
	if s.notified[e.ID] {
		return
	}
	if s.notified == nil {
		s.notified = map[string]bool{}
	}
	if len(s.notified) > 2000 {
		s.notified = map[string]bool{} // enough memory of what was told
	}
	s.notified[e.ID] = true
	title, body := r.name, previewText(e)
	if r.dm {
		title = senderName
	} else {
		body = senderName + ": " + body
	}
	if title == "" {
		title = senderName
	}
	s.notify(model.Notice{Room: r.id, Event: e.ID, Title: title, Body: body, Sender: e.Sender, DM: r.dm, Mention: e.Mentions})
}
