package matrix

import (
	"context"
	"errors"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// RoomPermissions reads a room's power levels and says what the user may
// change: each of name, topic and picture is a state event with its own
// level to send it (the state default, 50, unless the room says).
func (s *Source) RoomPermissions(room string, done func(model.RoomPerms)) {
	if s.cli == nil {
		done(model.RoomPerms{})
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var pl event.PowerLevelsEventContent
		if err := s.cli.StateEvent(ctx, id.RoomID(room), event.StatePowerLevels, "", &pl); err != nil {
			s.post(func() { done(model.RoomPerms{}) }) // unknown: nothing offered
			return
		}
		me := pl.GetUserLevel(id.UserID(s.me.ID))
		can := func(t event.Type) bool { return me >= pl.GetEventLevel(t) }
		perms := model.RoomPerms{Name: can(event.StateRoomName), Topic: can(event.StateTopic), Avatar: can(event.StateRoomAvatar)}
		s.post(func() { done(perms) })
	}()
}

// sendRoomState sets one piece of a room's state, off the UI thread, and
// calls done with "" or a line saying why not; undo puts the shown value
// back on a refusal.
func (s *Source) sendRoomState(room string, typ event.Type, content any, what string, undo func(), done func(string)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := s.cli.SendStateEvent(ctx, id.RoomID(room), typ, "", content)
		s.post(func() {
			if err == nil {
				done("")
				return
			}
			undo()
			msg := "Could not change " + what + ": " + plain(err)
			var he mautrix.HTTPError
			if errors.As(err, &he) && he.RespError != nil && he.RespError.ErrCode == "M_FORBIDDEN" {
				msg = "You don't have permission to change " + what + "."
			}
			done(msg)
		})
	}()
}

// SetRoomName renames a room.
func (s *Source) SetRoomName(room, name string, done func(string)) {
	r, ok := s.rooms[room]
	name = strings.TrimSpace(name)
	if !ok || s.cli == nil || name == "" {
		done("A name can't be empty.")
		return
	}
	prev := r.name
	r.name = name
	s.markDirty(room)
	s.sendRoomState(room, event.StateRoomName, &event.RoomNameEventContent{Name: name}, "the name",
		func() { r.name = prev; s.markDirty(room) }, done)
}

// SetRoomTopic changes a room's topic (an empty one clears it).
func (s *Source) SetRoomTopic(room, topic string, done func(string)) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		done("Not signed in.")
		return
	}
	prev := r.topic
	r.topic = strings.TrimSpace(topic)
	s.markDirty(room)
	s.sendRoomState(room, event.StateTopic, &event.TopicEventContent{Topic: r.topic}, "the topic",
		func() { r.topic = prev; s.markDirty(room) }, done)
}

// SetRoomAvatar sets a room's picture: the middle square of the file,
// scaled down, uploaded and set.
func (s *Source) SetRoomAvatar(room string, f model.File, done func(string)) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		done("Not signed in.")
		return
	}
	go func() {
		data, mime, err := prepareAvatar(f.Data)
		if err != nil {
			s.post(func() { done("Could not use that picture: " + err.Error()) })
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		up, err := s.cli.UploadBytes(ctx, data, mime)
		if err != nil {
			s.post(func() { done("Could not upload the picture: " + plain(err)) })
			return
		}
		mxc := up.ContentURI.String()
		prev := r.avatar
		s.post(func() {
			r.avatar = mxc
			s.markDirty(room)
			s.sendRoomState(room, event.StateRoomAvatar, &event.RoomAvatarEventContent{URL: id.ContentURIString(mxc)}, "the picture",
				func() { r.avatar = prev; s.markDirty(room) }, done)
		})
	}()
}

// RemoveRoomAvatar takes a room's picture away.
func (s *Source) RemoveRoomAvatar(room string, done func(string)) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		done("Not signed in.")
		return
	}
	prev := r.avatar
	r.avatar = ""
	s.markDirty(room)
	s.sendRoomState(room, event.StateRoomAvatar, &event.RoomAvatarEventContent{}, "the picture",
		func() { r.avatar = prev; s.markDirty(room) }, done)
}

var _ model.RoomEditor = (*Source)(nil)
