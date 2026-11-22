package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// ---- Invitations ----------------------------------------------------------

// An invitation is not a room yet: it has a name and who asked, from the
// stripped state the sync sends, and nothing else until it is accepted.
// The sync would hand those events to the room handlers, which would make a
// room of it; takeInvites keeps them out and keeps them here instead (in the
// cache too: a resumed sync never repeats an invitation).

// Invites lists the invitations waiting, the newest first. UI thread.
func (s *Source) Invites() []model.Invite {
	out := make([]model.Invite, 0, len(s.invites))
	for _, inv := range s.invites {
		out = append(out, *inv)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Time.Equal(out[j].Time) {
			return out[i].Time.After(out[j].Time)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// takeInvites reads a sync response's invitations out of it (so the room
// handlers never see them), and forgets those that were answered
// elsewhere: joined, or no longer invited. Called before the response is
// processed; the changes land with the batch.
func (s *Source) takeInvites(resp *mautrix.RespSync) {
	invited := resp.Rooms.Invite
	resp.Rooms.Invite = nil
	var answered []string
	for rid := range resp.Rooms.Join {
		answered = append(answered, rid.String())
	}
	for rid := range resp.Rooms.Leave {
		answered = append(answered, rid.String())
	}
	if len(invited) == 0 && len(answered) == 0 {
		return
	}
	got := map[string]*model.Invite{}
	for rid, room := range invited {
		if inv := inviteFrom(s.me.ID, rid.String(), room.State.Events); inv != nil {
			got[rid.String()] = inv
		}
	}
	s.post(func() {
		changed := false
		for _, rid := range answered {
			if _, ok := s.invites[rid]; ok {
				delete(s.invites, rid)
				changed = true
			}
		}
		for rid, inv := range got {
			if s.invites == nil {
				s.invites = map[string]*model.Invite{}
			}
			s.invites[rid] = inv
			changed = true
		}
		if changed {
			s.dirtyMeta = true
			s.schedulePersist()
		}
	})
}

// inviteFrom reads an invitation from its stripped state: the room's name
// (or its alias, or who invited), its picture, whether it is a direct chat,
// and who asked.
func inviteFrom(me, room string, state []*event.Event) *model.Invite {
	inv := &model.Invite{ID: room, Time: time.Now()}
	var alias string
	for _, evt := range state {
		if evt == nil {
			continue
		}
		_ = evt.Content.ParseRaw(evt.Type)
		switch evt.Type {
		case event.StateRoomName:
			if c := evt.Content.AsRoomName(); c != nil {
				inv.Name = c.Name
			}
		case event.StateCanonicalAlias:
			if c := evt.Content.AsCanonicalAlias(); c != nil {
				alias = c.Alias.String()
			}
		case event.StateRoomAvatar:
			if c := evt.Content.AsRoomAvatar(); c != nil && c.URL != "" {
				inv.Avatar = string(c.URL)
			}
		case event.StateTopic:
			if c := evt.Content.AsTopic(); c != nil {
				inv.Topic = c.Topic
			}
		case event.StateCreate:
			if c := evt.Content.AsCreate(); c != nil && c.Type == event.RoomTypeSpace {
				inv.Space = true
			}
		case event.StateMember:
			if evt.StateKey != nil && *evt.StateKey == me {
				if c := evt.Content.AsMember(); c != nil && c.Membership == event.MembershipInvite {
					inv.Inviter = evt.Sender.String()
					inv.DM = c.IsDirect
					if evt.Timestamp > 0 {
						inv.Time = time.UnixMilli(evt.Timestamp)
					}
				}
			}
		}
	}
	if inv.Name == "" {
		inv.Name = alias
	}
	if inv.Name == "" && inv.Inviter != "" {
		inv.Name = mxidName(inv.Inviter)
	}
	if inv.Name == "" {
		inv.Name = room
	}
	return inv
}

// AcceptInvite joins an invited room: done gets its ID (the room itself
// arrives with the next sync) or why not.
func (s *Source) AcceptInvite(room string, done func(room, err string)) {
	inv := s.invites[room]
	s.JoinRoom(room, func(joined, err string) {
		if err == "" {
			delete(s.invites, room)
			s.dirtyMeta = true
			s.schedulePersist()
			if inv != nil && inv.DM && inv.Inviter != "" {
				s.markDirectLater(joined, inv.Inviter)
			}
		}
		done(joined, err)
	})
}

// DeclineInvite turns an invitation down: it goes at once.
func (s *Source) DeclineInvite(room string) {
	if _, ok := s.invites[room]; !ok {
		return
	}
	delete(s.invites, room)
	s.dirtyMeta = true
	s.schedulePersist()
	if s.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.cli.LeaveRoom(ctx, id.RoomID(room)); err == nil {
			_, _ = s.cli.ForgetRoom(ctx, id.RoomID(room))
		}
	}()
}

// markDirectLater records an accepted direct-chat invitation in m.direct,
// so it is listed as a DM here and in other clients.
func (s *Source) markDirectLater(room, person string) {
	if s.cli == nil || room == "" {
		return
	}
	r := s.roomOf(room)
	r.dm, r.partner = true, person
	s.direct[room] = true
	s.dirtyMeta = true
	s.markDirty(room)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var direct map[string][]string
		if err := s.cli.GetAccountData(ctx, event.AccountDataDirectChats.Type, &direct); err != nil || direct == nil {
			direct = map[string][]string{}
		}
		for _, existing := range direct[person] {
			if existing == room {
				return
			}
		}
		direct[person] = append(direct[person], room)
		_ = s.cli.SetAccountData(ctx, event.AccountDataDirectChats.Type, direct)
	}()
}

func (s *Source) invitesRecord() []byte {
	list := make([]*model.Invite, 0, len(s.invites))
	for _, inv := range s.invites {
		list = append(list, inv)
	}
	b, _ := json.Marshal(list)
	return b
}

func (s *Source) loadInvites(b []byte) {
	var list []*model.Invite
	if json.Unmarshal(b, &list) != nil {
		return
	}
	for _, inv := range list {
		if inv != nil && inv.ID != "" {
			if s.invites == nil {
				s.invites = map[string]*model.Invite{}
			}
			s.invites[inv.ID] = inv
		}
	}
}

// ---- Joining and creating --------------------------------------------------

// parseTarget reads what the user typed to join: a room ID (!id:server), an
// alias (#room:server), a matrix.to link or a matrix: URI to either. It
// returns the ID or alias and the servers to try ("via").
func parseTarget(in string) (target string, via []string, err error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", nil, errors.New("Type a room address, like #room:server.org")
	}
	switch {
	case strings.HasPrefix(in, "https://matrix.to/#/"), strings.HasPrefix(in, "http://matrix.to/#/"):
		rest := in[strings.Index(in, "#/")+2:]
		if i := strings.IndexAny(rest, "?"); i >= 0 {
			if q, qerr := url.ParseQuery(rest[i+1:]); qerr == nil {
				via = q["via"]
			}
			rest = rest[:i]
		}
		if u, uerr := url.PathUnescape(rest); uerr == nil {
			rest = u
		}
		in = rest
	case strings.HasPrefix(in, "matrix:"):
		rest := strings.TrimPrefix(in, "matrix:")
		rest = strings.TrimPrefix(rest, "//")
		if i := strings.Index(rest, "?"); i >= 0 {
			if q, qerr := url.ParseQuery(rest[i+1:]); qerr == nil {
				via = q["via"]
			}
			rest = rest[:i]
		}
		switch {
		case strings.HasPrefix(rest, "r/"):
			in = "#" + strings.TrimPrefix(rest, "r/")
		case strings.HasPrefix(rest, "roomid/"):
			in = "!" + strings.TrimPrefix(rest, "roomid/")
		default:
			return "", nil, errors.New("That is not a room link")
		}
	}
	if (strings.HasPrefix(in, "#") || strings.HasPrefix(in, "!")) && strings.Contains(in, ":") {
		return in, via, nil
	}
	return "", nil, errors.New("A room address looks like #room:server.org")
}

// JoinRoom joins a room by address, ID or link. done gets the room's ID once
// joined (it shows in the lists with the next sync), or why not.
func (s *Source) JoinRoom(target string, done func(room, err string)) {
	if s.cli == nil {
		done("", "Not signed in.")
		return
	}
	t, via, err := parseTarget(target)
	if err != nil {
		done("", err.Error())
		return
	}
	if strings.HasPrefix(t, "!") {
		if _, ok := s.rooms[t]; ok {
			done(t, "") // already in it
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		resp, err := s.cli.JoinRoom(ctx, t, &mautrix.ReqJoinRoom{Via: via})
		if err != nil {
			s.post(func() { done("", joinFailure(err)) })
			return
		}
		room := resp.RoomID.String()
		s.post(func() {
			delete(s.invites, room)
			done(room, "")
		})
	}()
}

// joinFailure says in a line why a join did not work.
func joinFailure(err error) string {
	var he mautrix.HTTPError
	if errors.As(err, &he) && he.RespError != nil {
		switch he.RespError.ErrCode {
		case "M_NOT_FOUND":
			return "No room with that address."
		case "M_FORBIDDEN":
			return "That room needs an invitation."
		case "M_LIMIT_EXCEEDED":
			return "Too many tries. Wait a moment."
		}
	}
	return "Could not join: " + plain(err)
}

// CreateRoom makes a room — or a community (a space) — and gives its ID. A
// private room is encrypted and invite-only; a public one is open and gets
// an address from its name.
func (s *Source) CreateRoom(spec model.RoomSpec, done func(room, err string)) {
	if s.cli == nil {
		done("", "Not signed in.")
		return
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		done("", "Give it a name.")
		return
	}
	req := &mautrix.ReqCreateRoom{Name: name, Topic: strings.TrimSpace(spec.Topic)}
	for _, who := range spec.Invite {
		req.Invite = append(req.Invite, id.UserID(who))
	}
	if spec.Public {
		req.Visibility = "public"
		req.Preset = "public_chat"
		if alias := aliasFor(name); alias != "" {
			req.RoomAliasName = alias
		}
	} else {
		req.Preset = "private_chat"
		if !spec.Space {
			req.InitialState = []*event.Event{{
				Type:    event.StateEncryption,
				Content: event.Content{Parsed: &event.EncryptionEventContent{Algorithm: id.AlgorithmMegolmV1}},
			}}
		}
	}
	if spec.Space {
		req.CreationContent = map[string]any{"type": string(event.RoomTypeSpace)}
		// Only admins write in a community's own room.
		req.PowerLevelOverride = &event.PowerLevelsEventContent{EventsDefault: 100}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		resp, err := s.cli.CreateRoom(ctx, req)
		if err != nil {
			msg := "Could not create it: " + plain(err)
			var he mautrix.HTTPError
			if errors.As(err, &he) && he.RespError != nil && he.RespError.ErrCode == "M_ROOM_IN_USE" {
				msg = "That address is taken. Try another name."
			}
			s.post(func() { done("", msg) })
			return
		}
		room := resp.RoomID.String()
		s.post(func() {
			r := s.roomOf(room)
			r.name, r.space, r.encrypted = name, spec.Space, !spec.Public && !spec.Space
			r.topic = spec.Topic
			s.markDirty(room)
			done(room, "")
		})
	}()
}

// aliasFor makes a room's address from its name: lowercase letters and
// digits, dashes between words.
func aliasFor(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// Invite asks someone into a room.
func (s *Source) Invite(room, person string, done func(err string)) {
	if s.cli == nil {
		done("Not signed in.")
		return
	}
	person = strings.TrimSpace(person)
	if !strings.HasPrefix(person, "@") || !strings.Contains(person, ":") {
		done("A person's address looks like @name:server.org")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := s.cli.InviteUser(ctx, id.RoomID(room), &mautrix.ReqInviteUser{UserID: id.UserID(person)})
		msg := ""
		if err != nil {
			msg = "Could not invite them: " + plain(err)
			var he mautrix.HTTPError
			if errors.As(err, &he) && he.RespError != nil {
				switch he.RespError.ErrCode {
				case "M_FORBIDDEN":
					msg = "You can't invite people to this room."
				case "M_NOT_FOUND":
					msg = "No one with that address."
				}
			}
		}
		s.post(func() { done(msg) })
	}()
}

// ---- Looking for people and rooms -------------------------------------------

// SearchPeople looks in the homeserver's user directory. A full address
// (@name:server) is offered as typed even when the directory does not know it.
func (s *Source) SearchPeople(query string, done func(people []model.Person, err string)) {
	query = strings.TrimSpace(query)
	if s.cli == nil || query == "" {
		done(nil, "")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var out []model.Person
		resp, err := s.cli.SearchUserDirectory(ctx, query, 12)
		if err == nil && resp != nil {
			for _, u := range resp.Results {
				p := model.Person{ID: u.UserID.String(), Name: u.DisplayName}
				if p.Name == "" {
					// The directory's results say "display_name"; the profile
					// shape mautrix reads says "displayname".
					p.Name, _ = u.Extra["display_name"].(string)
				}
				if p.Name == "" {
					p.Name = mxidName(p.ID)
				}
				if !u.AvatarURL.IsEmpty() {
					p.Avatar = u.AvatarURL.String()
				}
				out = append(out, p)
			}
		}
		if strings.HasPrefix(query, "@") && strings.Contains(query, ":") {
			found := false
			for _, p := range out {
				found = found || p.ID == query
			}
			if !found {
				p := model.Person{ID: query, Name: mxidName(query)}
				if prof, perr := s.cli.GetProfile(ctx, id.UserID(query)); perr == nil && prof != nil {
					if prof.DisplayName != "" {
						p.Name = prof.DisplayName
					}
					if !prof.AvatarURL.IsEmpty() {
						p.Avatar = prof.AvatarURL.String()
					}
				}
				out = append([]model.Person{p}, out...)
			}
		}
		msg := ""
		if err != nil && len(out) == 0 {
			msg = "Could not search: " + plain(err)
		}
		s.post(func() { done(out, msg) })
	}()
}

// PublicRooms searches the homeserver's room directory (or another
// server's, for "term server.org" is not parsed: pass the server apart).
func (s *Source) PublicRooms(query, server string, done func(rooms []model.PublicRoom, err string)) {
	if s.cli == nil {
		done(nil, "Not signed in.")
		return
	}
	req := &mautrix.ReqPublicRooms{Limit: 30, Server: strings.TrimSpace(server)}
	if q := strings.TrimSpace(query); q != "" {
		req.Filter = &mautrix.PublicRoomsFilter{GenericSearchTerm: q}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := s.cli.PublicRooms(ctx, req)
		if err != nil {
			s.post(func() { done(nil, "Could not load the directory: "+plain(err)) })
			return
		}
		var out []model.PublicRoom
		for _, info := range resp.Chunk {
			pr := model.PublicRoom{
				ID: info.RoomID.String(), Alias: info.CanonicalAlias.String(), Name: info.Name,
				Topic: info.Topic, Members: info.NumJoinedMembers, Avatar: string(info.AvatarURL),
				Space: info.RoomType == event.RoomTypeSpace,
			}
			if pr.Name == "" {
				pr.Name = pr.Alias
			}
			if pr.Name == "" {
				pr.Name = pr.ID
			}
			out = append(out, pr)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Members > out[j].Members })
		s.post(func() {
			for i := range out {
				_, out[i].Joined = s.rooms[out[i].ID]
			}
			done(out, "")
		})
	}()
}

var _ model.RoomManager = (*Source)(nil)
