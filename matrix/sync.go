package matrix

import (
	"context"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/format"
	"slopement/model"
)

// NewSynced prepares a Source over a logged-in account's client: the
// state and crypto stores feed it, mautrix's olm machine decrypts
// (goolm, pure Go), and the handlers below turn /sync's events into the
// model's rooms and timeline. Start runs the loop; the client stays ours
// to send through.
//
// Sliding sync (MSC4186) comes later behind this same constructor; today
// every server gets the classic /v3/sync with a filter.
func NewSynced(ctx context.Context, acc *Account, cli *mautrix.Client) (*Source, error) {
	pickle, err := PickleKey(cli.UserID.String(), cli.DeviceID.String())
	if err != nil {
		return nil, err
	}

	// The stores: the room state the crypto machine trusts, and the sync
	// batch token, which the crypto store also keeps.
	cli.StateStore = acc.State
	cli.Store = acc.Crypto

	helper, err := cryptohelper.NewCryptoHelper(cli, pickle, acc.Crypto)
	if err != nil {
		return nil, err
	}

	syncer := mautrix.NewDefaultSyncer()
	syncer.FilterJSON = &mautrix.Filter{
		Room: &mautrix.RoomFilter{
			State:    &mautrix.FilterPart{LazyLoadMembers: true},
			Timeline: &mautrix.FilterPart{Limit: 30, UnreadThreadNotifications: true},
		},
	}
	cli.Syncer = syncer

	// The crypto machine first — it must see the sync before anything can
	// decrypt — then the member state store it reads, then us.
	if err := helper.Init(ctx); err != nil {
		return nil, err
	}
	syncer.OnEvent(cli.StateStoreSyncHandler)

	s := NewSource(&Session{UserID: cli.UserID, DeviceID: cli.DeviceID})
	s.cli, s.acc, s.helper = cli, acc, helper

	syncer.OnSync(func(ctx context.Context, resp *mautrix.RespSync, since string) bool {
		s.onSync(resp)
		return true
	})

	syncer.OnEventType(event.EventMessage, s.onMessage)
	syncer.OnEventType(event.EventReaction, s.onReaction)
	syncer.OnEventType(event.EventEncrypted, s.onEncrypted)
	syncer.OnEventType(event.StateMember, s.onMember)
	syncer.OnEventType(event.StateRoomName, s.onRoomName)
	syncer.OnEventType(event.StateTopic, s.onTopic)
	syncer.OnEventType(event.StateCanonicalAlias, s.onAlias)
	syncer.OnEventType(event.StateEncryption, s.onEncryption)
	syncer.OnEventType(event.EphemeralEventTyping, s.onTyping)
	syncer.OnEventType(event.EphemeralEventReceipt, s.onReceipt)
	syncer.OnEventType(event.EphemeralEventPresence, s.onPresence)
	syncer.OnEventType(event.AccountDataDirectChats, s.onDirect)
	syncer.OnEventType(event.AccountDataRoomTags, s.onTags)
	return s, nil
}

// Start runs the sync loop until ctx ends. Call it after SetUpdate, so
// the first sync's rooms reach the window.
func (s *Source) Start(ctx context.Context) {
	if s.started || s.cli == nil {
		return
	}
	s.started = true
	go func() {
		// SyncWithContext retries on its own, with the default syncer's
		// backoff; it returns when ctx ends.
		_ = s.cli.SyncWithContext(ctx)
	}()
}

// ---- the sync, into the model --------------------------------------------------------

// roomOf is the room of an event, made if the sync's envelope has not
// come first. Call on the UI thread.
func (s *Source) roomOf(roomID string) *room {
	r, ok := s.rooms[roomID]
	if !ok {
		r = &room{id: roomID}
		s.rooms[roomID] = r
	}
	return r
}

// onSync reads what only the whole response carries: the joined rooms'
// envelopes, with their server-counted unread.
func (s *Source) onSync(resp *mautrix.RespSync) {
	s.post(func() {
		for roomID, joined := range resp.Rooms.Join {
			r := s.roomOf(roomID.String())
			if joined.UnreadNotifications != nil {
				r.unread = joined.UnreadNotifications.NotificationCount
				r.mentions = joined.UnreadNotifications.HighlightCount
			}
		}
	})
}

// onMessage is a room message: text, media, a reply, an edit — what the
// timeline is made of.
func (s *Source) onMessage(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsMessage()
	if c == nil {
		return
	}

	// An edit rewrites its target; it is not an event of its own.
	if target, body, ok := editOf(c); ok {
		s.post(func() { s.roomOf(evt.RoomID.String()).editEvent(target, body) })
		return
	}

	e := textEvent(s.me.ID, evt)
	sender := evt.Sender.String()
	s.post(func() {
		r := s.roomOf(evt.RoomID.String())
		name := s.nameOf(sender)
		r.appendEvent(e, name, sender == s.me.ID)
		if r.dm && r.partner == "" && sender != s.me.ID {
			r.partner = sender // a DM's other person
			if r.name == "" {
				r.name = name
			}
		}
	})
}

// onReaction is one reaction: a key on an event, or its taking-back.
func (s *Source) onReaction(ctx context.Context, evt *event.Event) {
	target, key, ok := reactionOf(evt.Content.AsMessage())
	if !ok {
		return
	}
	s.post(func() { s.roomOf(evt.RoomID.String()).react(target, evt.Sender.String(), key) })
}

// onEncrypted is a message that did not decrypt: the crypto helper
// re-dispatches the ones that do, as plain m.room.message.
func (s *Source) onEncrypted(ctx context.Context, evt *event.Event) {
	if evt.Content.AsEncrypted() == nil {
		return // it decrypted and was dispatched as m.room.message
	}
	e := undecryptable(evt)
	s.post(func() { s.roomOf(evt.RoomID.String()).appendEvent(e, s.nameOf(evt.Sender.String()), false) })
}

// onMember is a membership change: who is in the room, who they are, and
// a quiet system line — from the timeline, never from the initial sync's
// state dump, which would flood every room with its whole history.
func (s *Source) onMember(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsMember()
	if c == nil || evt.StateKey == nil {
		return
	}
	who := *evt.StateKey
	name := nameFor(c.Displayname, who)
	line, isLine := memberLine(evt)
	fromTimeline := evt.Mautrix.EventSource&event.SourceTimeline != 0
	s.post(func() {
		if p, ok := s.people[who]; ok {
			p.Name, p.Avatar = name, string(c.AvatarURL)
		} else {
			s.people[who] = &model.Person{ID: who, Name: name, Avatar: string(c.AvatarURL), Contact: true}
		}
		r := s.roomOf(evt.RoomID.String())
		switch c.Membership {
		case event.MembershipJoin:
			r.members++
		case event.MembershipLeave, event.MembershipBan:
			r.members--
		}
		if isLine && fromTimeline && who != s.me.ID {
			r.appendEvent(line, name, false)
		}
	})
}

// onRoomName, onTopic, onAlias and onEncryption carry the room's shape.
func (s *Source) onRoomName(ctx context.Context, evt *event.Event) {
	name := evt.Content.AsRoomName().Name
	s.post(func() { s.roomOf(evt.RoomID.String()).name = name })
}

func (s *Source) onTopic(ctx context.Context, evt *event.Event) {
	topic := evt.Content.AsTopic().Topic
	s.post(func() { s.roomOf(evt.RoomID.String()).topic = topic })
}

func (s *Source) onAlias(ctx context.Context, evt *event.Event) {
	alias := string(evt.Content.AsCanonicalAlias().Alias)
	s.post(func() {
		r := s.roomOf(evt.RoomID.String())
		if r.name == "" {
			r.name = alias
		}
	})
}

func (s *Source) onEncryption(ctx context.Context, evt *event.Event) {
	s.post(func() { s.roomOf(evt.RoomID.String()).encrypted = true })
}

// onTyping is who is typing right now — the word expires, so it clears
// after half a minute without a fresher one.
func (s *Source) onTyping(ctx context.Context, evt *event.Event) {
	typing := evt.Content.AsTyping()
	if typing == nil {
		return
	}
	userIDs := typing.UserIDs
	s.post(func() {
		r, ok := s.rooms[evt.RoomID.String()]
		if !ok {
			return
		}
		names := make([]string, 0, len(userIDs))
		for _, u := range userIDs {
			if u.String() != s.me.ID {
				names = append(names, s.nameOf(u.String()))
			}
		}
		r.typing = names
		if len(names) > 0 && s.update != nil {
			time.AfterFunc(30*time.Second, func() {
				s.update(func() {
					r.typing = nil
					s.changed()
				})
			})
		}
	})
}

// onReceipt is a read marker; ours alone counts, clearing the room.
func (s *Source) onReceipt(ctx context.Context, evt *event.Event) {
	receipts := evt.Content.AsReceipt()
	if receipts == nil {
		return
	}
	for _, byType := range *receipts {
		if users, ok := byType[event.ReceiptTypeRead]; ok {
			if _, me := users[id.UserID(s.me.ID)]; me {
				s.post(func() {
					if r, ok := s.rooms[evt.RoomID.String()]; ok {
						r.unread, r.mentions = 0, 0
					}
				})
			}
		}
	}
}

// onPresence is who is around, the user included.
func (s *Source) onPresence(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsPresence()
	if c == nil {
		return
	}
	who := evt.Sender.String()
	presence := model.Offline
	switch c.Presence {
	case event.PresenceOnline:
		presence = model.Online
	case event.PresenceUnavailable:
		presence = model.Away
	}
	s.post(func() {
		if who == s.me.ID {
			s.me.Presence = presence
			return
		}
		if p, ok := s.people[who]; ok {
			p.Presence = presence
		} else {
			s.people[who] = &model.Person{ID: who, Name: nameFor(c.Displayname, who), Presence: presence, Contact: true}
		}
	})
}

// onDirect is the m.direct map: which rooms are DMs, with whom.
func (s *Source) onDirect(ctx context.Context, evt *event.Event) {
	parsed := evt.Content.AsDirectChats()
	if parsed == nil {
		return
	}
	type pair struct {
		room, partner string
	}
	var pairs []pair
	for partner, roomIDs := range *parsed {
		for _, roomID := range roomIDs {
			pairs = append(pairs, pair{roomID.String(), partner.String()})
		}
	}
	s.post(func() {
		for _, p := range pairs {
			s.direct[p.room] = true
			if r, ok := s.rooms[p.room]; ok {
				r.dm, r.partner = true, p.partner
				if r.name == "" {
					r.name = s.nameOf(p.partner)
				}
			}
		}
	})
}

// onTags is the room tags: favourites and low priority.
func (s *Source) onTags(ctx context.Context, evt *event.Event) {
	tags := evt.Content.AsTag().Tags
	s.post(func() {
		if r := s.roomOf(evt.RoomID.String()); r != nil {
			_, fav := tags["m.favourite"]
			_, low := tags["m.lowpriority"]
			r.favourite, r.lowPriority = fav, low
		}
	})
}

// nameOf is who a sender is, as the timeline shows them.
func (s *Source) nameOf(mxid string) string {
	if p, ok := s.people[mxid]; ok {
		return p.Name
	}
	return mxidName(mxid)
}

// ---- the writes -----------------------------------------------------------------------

// Send posts a message to a room: Markdown in, the homeserver's event
// out, encrypted where the room is. The sync loop echoes it back into
// the timeline.
func (s *Source) Send(room, markdown, replyTo string) {
	if s.cli == nil {
		return
	}
	// Read on this (UI) thread, so the goroutine never touches the maps.
	r, ok := s.rooms[room]
	encrypted := ok && r.encrypted
	go func() {
		content := contentOf(markdown)
		if replyTo != "" {
			content.RelatesTo = (&event.RelatesTo{}).SetReplyTo(id.EventID(replyTo))
		}
		ctx := context.Background()
		var err error
		if encrypted && s.helper != nil {
			var enc *event.EncryptedEventContent
			if enc, err = s.helper.Encrypt(ctx, id.RoomID(room), event.EventMessage, content); err == nil {
				_, err = s.cli.SendMessageEvent(ctx, id.RoomID(room), event.EventEncrypted, enc)
			}
		} else {
			_, err = s.cli.SendMessageEvent(ctx, id.RoomID(room), event.EventMessage, content)
		}
		if err != nil {
			// The sending milestone surfaces errors; a failed message is
			// simply not echoed today.
			_ = err
		}
	}()
}

// React puts a reaction on an event, or takes it back with an empty key.
func (s *Source) React(room, eventID, key string) {
	if s.cli == nil {
		return
	}
	go func() {
		_, _ = s.cli.SendReaction(context.Background(), id.RoomID(room), id.EventID(eventID), key)
	}()
}

// MarkRead reads a room to its end: the marker goes to the server, the
// room clears at once.
func (s *Source) MarkRead(room string) {
	if s.cli == nil {
		return
	}
	r, ok := s.rooms[room]
	if !ok || len(r.events) == 0 {
		return
	}
	last := r.events[len(r.events)-1].ID
	r.unread, r.mentions = 0, 0
	go func() {
		_ = s.cli.MarkRead(context.Background(), id.RoomID(room), id.EventID(last))
	}()
}

// contentOf is a Markdown message as Matrix sends it: the plain body and
// the formatted one when the Markdown means anything.
func contentOf(markdown string) *event.MessageEventContent {
	doc := format.Parse(markdown)
	content := event.MessageEventContent{MsgType: event.MsgText, Body: format.Plain(doc)}
	if format.NeedsHTML(doc) {
		content.FormattedBody, content.Format = format.HTML(doc), event.FormatHTML
	}
	return &content
}
