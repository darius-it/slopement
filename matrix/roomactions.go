package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
	"maunium.net/go/mautrix/pushrules"

	"slopement/model"
)

// The room actions of the menus, on a real account. Each shows at once (the
// room list changes before the server answers) and goes to the server after;
// a refusal is said as a quiet line in the room, and the sync's account
// data then confirms what was shown. They all run on the UI thread, the
// work off it.

const (
	tagFavourite   = event.RoomTag("m.favourite")
	tagLowPriority = event.RoomTag("m.lowpriority")
)

// SetFavourite adds a room to the favourites, or takes it out. A room is
// never a favourite and low priority at once (as in Element): one tag
// takes the other's away.
func (s *Source) SetFavourite(room string, on bool) { s.tag(room, tagFavourite, tagLowPriority, on) }

// SetLowPriority moves a room down to Low priority, or back.
func (s *Source) SetLowPriority(room string, on bool) { s.tag(room, tagLowPriority, tagFavourite, on) }

// SetCommunityFavourite favourites a community: its space room carries the
// tag, as any room does.
func (s *Source) SetCommunityFavourite(community string, on bool) {
	s.tag(community, tagFavourite, tagLowPriority, on)
}

// tag sets or clears one of the two list tags on a room (other is the tag
// that cannot be there together with it).
func (s *Source) tag(room string, tag, other event.RoomTag, on bool) {
	if s.cli == nil {
		return
	}
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	set := func(t event.RoomTag, v bool) {
		switch t {
		case tagFavourite:
			r.favourite = v
		case tagLowPriority:
			r.lowPriority = v
		}
	}
	has := func(t event.RoomTag) bool {
		if t == tagFavourite {
			return r.favourite
		}
		return r.lowPriority
	}
	wasOther := has(other)
	prev := has(tag)
	set(tag, on)
	if on {
		set(other, false)
	}
	s.markDirty(room)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if on {
			if wasOther {
				_ = s.cli.RemoveTag(ctx, id.RoomID(room), other)
			}
			err = s.cli.AddTag(ctx, id.RoomID(room), tag, 0.5)
		} else {
			err = s.cli.RemoveTag(ctx, id.RoomID(room), tag)
		}
		if err != nil {
			s.post(func() {
				if _, ok := s.rooms[room]; ok { // not applied: back as it was
					set(tag, prev)
					if on {
						set(other, wasOther)
					}
					s.markDirty(room)
				}
			})
			s.notice(room, "Could not change the room's list: "+plain(err))
		}
	}()
}

// MarkUnread flags a room as unread again (m.marked_unread, which other
// clients follow too): it shows as unread until it is opened.
func (s *Source) MarkUnread(room string) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		return
	}
	r.markedUnread = true
	s.markDirty(room)
	go s.setMarkedUnread(room, true)
}

func (s *Source) setMarkedUnread(room string, unread bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = s.cli.SetRoomAccountData(ctx, id.RoomID(room), event.AccountDataMarkedUnread.Type, &event.MarkedUnreadEventContent{Unread: unread})
}

// onMarkedUnread is the room's m.marked_unread, as the server has it.
func (s *Source) onMarkedUnread(ctx context.Context, evt *event.Event) {
	if evt.RoomID == "" {
		return
	}
	c := evt.Content.AsMarkedUnread()
	room := evt.RoomID.String()
	s.post(func() {
		if r, ok := s.rooms[room]; ok && r.markedUnread != c.Unread {
			r.markedUnread = c.Unread
			s.markDirty(room)
		}
	})
}

// Leave leaves a room: it goes from the lists at once, and the server is
// told (and told to forget it, so it is not offered again).
func (s *Source) Leave(room string) {
	if s.cli == nil {
		return
	}
	if _, ok := s.rooms[room]; !ok {
		return
	}
	delete(s.rooms, room)
	s.gone = append(s.gone, room)
	delete(s.dirty, room)
	s.dirtyMeta = true
	s.schedulePersist()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.cli.LeaveRoom(ctx, id.RoomID(room)); err != nil {
			s.reportLeaveFailure(room, err)
			return
		}
		_, _ = s.cli.ForgetRoom(ctx, id.RoomID(room))
	}()
}

// reportLeaveFailure brings a room back that could not be left: the next
// sync shows it again; until then it says why it did not go.
func (s *Source) reportLeaveFailure(room string, err error) {
	s.post(func() {
		s.roomOf(room) // back, empty until the sync fills it again
		s.notice(room, "Could not leave the room: "+plain(err))
	})
}

// ---- Mute -------------------------------------------------------------

// A muted room is a room push rule with no actions: the server stops
// notifying, as every client does for it (Element's "Mute"). A timed mute
// also writes its end to account data (com.slopement.mutes), which this
// client honors when it is open, on any session.

const mutesData = "com.slopement.mutes"

// Mute mutes a room until a time (zero: until turned back on).
func (s *Source) Mute(room string, until time.Time) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		return
	}
	r.muted, r.mutedUntil = true, until
	s.markDirty(room)
	s.watchMute(room, until)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := s.cli.PutPushRule(ctx, "global", pushrules.RoomRule, room, &mautrix.ReqPutPushRule{Actions: []*pushrules.PushAction{}})
		if err != nil {
			s.post(func() {
				if r, ok := s.rooms[room]; ok {
					r.muted, r.mutedUntil = false, time.Time{}
					s.markDirty(room)
				}
			})
			s.notice(room, "Could not mute the room: "+plain(err))
			return
		}
		s.writeMute(ctx, room, until)
	}()
}

// Unmute turns a room's notifications back on.
func (s *Source) Unmute(room string) {
	r, ok := s.rooms[room]
	if !ok || s.cli == nil {
		return
	}
	r.muted, r.mutedUntil = false, time.Time{}
	s.markDirty(room)
	go s.unmuteRemote(room)
}

// unmuteRemote deletes the room's rule and forgets its expiry.
func (s *Source) unmuteRemote(room string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.cli.DeletePushRule(ctx, "global", pushrules.RoomRule, room); err != nil {
		var he mautrix.HTTPError
		if !errors.As(err, &he) || he.RespError == nil || he.RespError.ErrCode != "M_NOT_FOUND" {
			s.notice(room, "Could not unmute the room: "+plain(err))
		}
	}
	s.writeMute(ctx, room, time.Time{})
}

// writeMute records (or, for the zero time, forgets) when a room's mute
// ends, in account data every session reads. Read-modify-write of one small
// map: concurrent edits from two sessions are rare and the last one wins.
func (s *Source) writeMute(ctx context.Context, room string, until time.Time) {
	var all map[string]int64
	if err := s.cli.GetAccountData(ctx, mutesData, &all); err != nil || all == nil {
		all = map[string]int64{}
	}
	if until.IsZero() {
		delete(all, room)
	} else {
		all[room] = until.Unix()
	}
	_ = s.cli.SetAccountData(ctx, mutesData, all)
}

// watchMute unmutes a room when its timed mute ends: a timer while the app
// is open (a mute that ended while it was closed is found at the next sync).
func (s *Source) watchMute(room string, until time.Time) {
	if until.IsZero() || s.update == nil {
		return
	}
	d := time.Until(until)
	if d < 0 {
		d = 0
	}
	time.AfterFunc(d+500*time.Millisecond, func() {
		s.update(func() {
			if r, ok := s.rooms[room]; ok && r.muted && !r.mutedUntil.IsZero() && !time.Now().Before(r.mutedUntil) {
				s.Unmute(room)
			}
		})
	})
}

// onPushRules is the account's push rules: a room rule with no actions is a
// muted room.
func (s *Source) onPushRules(ctx context.Context, evt *event.Event) {
	var content struct {
		Global struct {
			Room []struct {
				RuleID  string            `json:"rule_id"`
				Enabled bool              `json:"enabled"`
				Actions []json.RawMessage `json:"actions"`
			} `json:"room"`
		} `json:"global"`
	}
	if json.Unmarshal(evt.Content.VeryRaw, &content) != nil {
		return
	}
	muted := map[string]bool{}
	for _, rule := range content.Global.Room {
		if rule.Enabled && len(rule.Actions) == 0 {
			muted[rule.RuleID] = true
		}
	}
	s.post(func() {
		for id, r := range s.rooms {
			m := muted[id]
			if r.muted != m {
				r.muted = m
				if !m {
					r.mutedUntil = time.Time{}
				}
				s.markDirty(id)
			}
		}
	})
}

// onMutes is where timed mutes end (com.slopement.mutes): kept on the
// rooms for display and the timer, and one that has ended is unmuted.
func (s *Source) onMutes(ctx context.Context, evt *event.Event) {
	var all map[string]int64
	if json.Unmarshal(evt.Content.VeryRaw, &all) != nil {
		return
	}
	s.post(func() {
		for rid, r := range s.rooms {
			end, ok := all[rid]
			if !ok {
				continue
			}
			until := time.Unix(end, 0)
			if r.muted {
				r.mutedUntil = until
				s.markDirty(rid)
				if !time.Now().Before(until) {
					s.Unmute(rid)
				} else {
					s.watchMute(rid, until)
				}
			}
		}
	})
}

// ---- Direct messages --------------------------------------------------

// OpenDM returns the DM with someone, when there is one already. Without,
// it returns "" and OpenDMAsync is the way to make one.
func (s *Source) OpenDM(person string) string {
	for id, r := range s.rooms {
		if r.dm && r.partner == person && !r.space {
			return id
		}
	}
	return ""
}

// OpenDMAsync finds or creates the DM with someone and calls done on the UI
// thread with its room ID ("" when it could not be made). A new one is an
// encrypted private chat with the person invited, recorded in m.direct so
// every client lists it as a DM.
func (s *Source) OpenDMAsync(person string, done func(room string)) {
	if room := s.OpenDM(person); room != "" {
		done(room)
		return
	}
	if s.cli == nil || person == "" {
		done("")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		resp, err := s.cli.CreateRoom(ctx, &mautrix.ReqCreateRoom{
			Invite:   []id.UserID{id.UserID(person)},
			IsDirect: true,
			Preset:   "trusted_private_chat",
			InitialState: []*event.Event{{
				Type:    event.StateEncryption,
				Content: event.Content{Parsed: &event.EncryptionEventContent{Algorithm: id.AlgorithmMegolmV1}},
			}},
		})
		if err != nil {
			s.post(func() { done("") })
			return
		}
		room := resp.RoomID.String()
		// m.direct: add this room under the person, keeping every other entry.
		var direct map[string][]string
		if err := s.cli.GetAccountData(ctx, event.AccountDataDirectChats.Type, &direct); err != nil || direct == nil {
			direct = map[string][]string{}
		}
		direct[person] = append(direct[person], room)
		_ = s.cli.SetAccountData(ctx, event.AccountDataDirectChats.Type, direct)
		s.post(func() {
			r := s.roomOf(room)
			r.dm, r.partner, r.encrypted = true, person, true
			if r.name == "" {
				r.name = s.nameOf(person)
			}
			s.direct[room] = true
			s.dirtyMeta = true
			s.markDirty(room)
			done(room)
		})
	}()
}

var _ model.DMCreator = (*Source)(nil)
