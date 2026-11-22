package matrix

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"
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

	s := NewSource(&Session{UserID: cli.UserID, DeviceID: cli.DeviceID})
	s.cli, s.acc, s.cache = cli, acc, acc.Cache

	// The stores: the room state the crypto machine trusts, and the sync
	// token — which the room cache owns: it resumes from it with the rooms
	// the cache holds, or starts from nothing when there is no cache to
	// trust.
	cli.StateStore = acc.State
	if acc.Cache != nil {
		acc.Cache.setKey(pickle)
	}
	s.resumed = s.loadCache(ctx)
	cli.Store = &tokenStore{SyncStore: acc.Crypto, token: s.token}

	helper, err := cryptohelper.NewCryptoHelper(cli, pickle, acc.Crypto)
	if err != nil {
		return nil, err
	}

	syncer := &batchSyncer{DefaultSyncer: mautrix.NewDefaultSyncer()}
	syncer.FilterJSON = &mautrix.Filter{
		Room: &mautrix.RoomFilter{
			State:    &mautrix.FilterPart{LazyLoadMembers: true},
			Timeline: &mautrix.FilterPart{Limit: 30, UnreadThreadNotifications: true},
		},
	}
	cli.Syncer = syncer

	// The crypto machine first — it must see the sync before anything can
	// decrypt — then the member state store it reads, then us. The
	// client's Crypto hook makes sending encrypt automatically, and the
	// verification helper requires it.
	if err := helper.Init(ctx); err != nil {
		return nil, err
	}
	cli.Crypto = helper
	syncer.OnEvent(cli.StateStoreSyncHandler)

	s.helper = helper
	syncer.src = s

	// The verification: the emoji with another device, or the Security
	// Key with the key backup behind it.
	driver, err := newVerify(ctx, s, cli, helper.Machine(), helper)
	if err != nil {
		return nil, err
	}
	s.verify = driver

	syncer.OnSync(func(ctx context.Context, resp *mautrix.RespSync, since string) bool {
		s.onSync(resp)
		return true
	})

	syncer.OnEventType(event.EventMessage, s.onMessage)
	syncer.OnEventType(event.EventSticker, s.onMessage)
	syncer.OnEventType(event.EventReaction, s.onReaction)
	syncer.OnEventType(event.EventRedaction, s.onRedaction)
	syncer.OnEventType(event.EventEncrypted, s.onEncrypted)
	syncer.OnEventType(event.StateMember, s.onMember)
	syncer.OnEventType(event.StatePowerLevels, s.onPowerLevels)
	syncer.OnEventType(event.StateRoomName, s.onRoomName)
	syncer.OnEventType(event.StateTopic, s.onTopic)
	syncer.OnEventType(event.StateRoomAvatar, s.onRoomAvatar)
	syncer.OnEventType(event.StatePinnedEvents, s.onPinned)
	syncer.OnEventType(event.StateCanonicalAlias, s.onAlias)
	syncer.OnEventType(event.StateEncryption, s.onEncryption)
	syncer.OnEventType(event.StateCreate, s.onCreate)
	syncer.OnEventType(event.StateSpaceChild, s.onSpaceChild)
	syncer.OnEventType(event.StateSpaceParent, s.onSpaceParent)
	syncer.OnEventType(event.EphemeralEventTyping, s.onTyping)
	syncer.OnEventType(event.EphemeralEventReceipt, s.onReceipt)
	syncer.OnEventType(event.EphemeralEventPresence, s.onPresence)
	syncer.OnEventType(event.AccountDataDirectChats, s.onDirect)
	syncer.OnEventType(event.AccountDataRoomTags, s.onTags)
	syncer.OnEventType(event.AccountDataMarkedUnread, s.onMarkedUnread)
	syncer.OnEventType(event.AccountDataPushRules, s.onPushRules)
	syncer.OnEventType(mutesType, s.onMutes)
	syncer.OnEventType(userColorsType, s.onUserColors)
	syncer.OnEventType(event.AccountDataIgnoredUserList, s.onIgnored)
	syncer.OnEventType(railOrderType, s.onRailOrder)
	syncer.OnEventType(statusType, s.onStatusData)
	syncer.OnEventType(userEmotesType, s.onUserEmotes)
	syncer.OnEventType(emoteRoomsType, s.onEmoteRooms)
	syncer.OnEventType(roomPackType, s.onRoomEmotes)
	for _, t := range pollStartTypes {
		syncer.OnEventType(t, s.onPollStart)
	}
	for _, t := range pollResponseTypes {
		syncer.OnEventType(t, s.onPollResponse)
	}
	for _, t := range pollEndTypes {
		syncer.OnEventType(t, s.onPollEnd)
	}
	return s, nil
}

// batchSyncer applies a whole sync response to the model as one UI
// update (see Source.beginBatch).
type batchSyncer struct {
	*mautrix.DefaultSyncer
	src *Source
}

func (b *batchSyncer) ProcessResponse(ctx context.Context, resp *mautrix.RespSync, since string) error {
	if b.src != nil {
		b.src.beginBatch()
		b.src.takeInvites(resp) // invitations are not rooms: kept apart, out of the handlers' way
		var left []string
		for id := range resp.Rooms.Leave {
			left = append(left, id.String())
		}
		defer b.src.endBatch(resp.NextBatch, left)
	}
	return b.DefaultSyncer.ProcessResponse(ctx, resp, since)
}

// OnFailedSync: a resumed sync whose token the server refuses (400)
// means the cache is stale — stop, so Start can drop it and begin again.
// Anything else keeps the default retry.
func (b *batchSyncer) OnFailedSync(res *mautrix.RespSync, err error) (time.Duration, error) {
	if b.src != nil && b.src.resumed && badSyncToken(err) {
		return 0, err
	}
	return b.DefaultSyncer.OnFailedSync(res, err)
}

// badSyncToken reports whether the server rejected the request's since
// token (not the login: an unknown access token is its own error).
func badSyncToken(err error) bool {
	var h mautrix.HTTPError
	if errors.As(err, &h) && h.Response != nil && h.Response.StatusCode == http.StatusBadRequest {
		return true
	}
	return false
}

// setMe is the user's own name and picture, from their profile or their
// member events. Empty values leave what is there: a member event of a
// room where they set no picture of their own must not blank the account's.
// UI thread.
func (s *Source) setMe(name, avatar string) {
	if name != "" && name != mxidName(s.me.ID) {
		s.me.Name = name
	}
	if avatar != "" {
		s.me.Avatar = avatar
	}
}

// fetchMe reads the user's own profile: the sync's member events may never
// mention them (lazy loading sends only some members), but the profile
// always has their name and picture.
func (s *Source) fetchMe(ctx context.Context) {
	prof, err := s.cli.GetProfile(ctx, s.cli.UserID)
	if err != nil || prof == nil {
		return
	}
	name, avatar := prof.DisplayName, ""
	if !prof.AvatarURL.IsEmpty() {
		avatar = prof.AvatarURL.String()
	}
	s.post(func() { s.setMe(name, avatar) })
}

// Start runs the sync loop until ctx ends. Call it after SetUpdate, so
// the first sync's rooms reach the window.
func (s *Source) Start(ctx context.Context) {
	if s.started || s.cli == nil {
		return
	}
	s.started = true
	go s.fetchMe(ctx)
	go s.fetchAccountData(ctx)
	go func() {
		// The sync's first answer brings the DMs; their presence and
		// status are then asked for (a resumed sync would never say).
		time.Sleep(3 * time.Second)
		s.fetchPresence(ctx)
	}()
	if s.verify != nil {
		go s.verify.restoreSaved(ctx)
		go s.keyWatch(ctx)
	}
	go func() {
		// SyncWithContext retries on its own, with the default syncer's
		// backoff; it returns when ctx ends — or when the server refused
		// the token the cache resumed from: then the cache goes, and the
		// sync starts over from nothing.
		for {
			err := s.cli.SyncWithContext(ctx)
			if ctx.Err() != nil || !(s.resumed && badSyncToken(err)) {
				return
			}
			s.resumed = false
			if ts, ok := s.cli.Store.(*tokenStore); ok {
				ts.token = ""
			}
			if s.cache != nil {
				s.cache.reset(ctx)
			}
		}
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
		s.spaceCache = nil
	}
	s.markDirty(roomID)
	return r
}

// onSync reads what only the whole response carries: the joined rooms'
// envelopes, with their server-counted unread.
func (s *Source) onSync(resp *mautrix.RespSync) {
	s.post(func() {
		for roomID, joined := range resp.Rooms.Join {
			s.gapStart(roomID.String(), joined.Timeline.Limited, joined.Timeline.PrevBatch)
			r := s.roomOf(roomID.String())
			if joined.UnreadNotifications != nil {
				r.unread = joined.UnreadNotifications.NotificationCount
				r.mentions = joined.UnreadNotifications.HighlightCount
			}
			if sum := joined.Summary; sum.JoinedMemberCount != nil {
				// The server's count: the member events a lazy-loading
				// sync sends are only some of the room.
				r.members, r.hasSummary = *sum.JoinedMemberCount, true
			}
		}
	})
}

// sourceHistory marks an event that came from paging a room's history
// back, not from the sync: the handlers put it in its place by time and do
// not count it as news.
const sourceHistory event.Source = 1 << 30

// onMessage is a room message: text, media, a reply, an edit — what the
// timeline is made of.
func (s *Source) onMessage(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsMessage()
	if c == nil {
		return
	}
	if c.MsgType == "" && c.Body == "" && c.NewContent == nil {
		return // a redacted message: nothing of it is left to show
	}

	// An edit rewrites its target; it is not an event of its own.
	if target, body, formatted, ok := editOf(c); ok {
		id, ts := evt.ID.String(), evt.Timestamp
		s.post(func() {
			r := s.roomOf(evt.RoomID.String())
			r.editEvent(target, body, formatted, ts)
			s.clearLock(r, id) // the edit itself may have shown as waiting for keys
		})
		return
	}

	e := textEvent(s.me.ID, evt)
	sender := evt.Sender.String()
	s.post(func() {
		r := s.roomOf(evt.RoomID.String())
		name := s.nameOf(sender)
		if txn := evt.Unsigned.TransactionID; sender == s.me.ID && txn != "" {
			r.adoptEcho(txn, e.ID) // the message the user sent: same place, server's ID
		}
		if evt.Mautrix.EventSource&sourceHistory != 0 {
			r.insertOlder(e) // a page of history: in its place, and not news
		} else {
			r.appendEvent(e, name, sender == s.me.ID)
			s.noticeOf(r, e, name)
		}
		// A decrypted event may be one that had waited for keys
		// re-dispatched: its raw copy goes with its tile.
		if _, raw := r.raw[e.ID]; raw {
			delete(r.raw, e.ID)
			s.encrypted--
		}
		if r.dm && r.partner == "" && sender != s.me.ID {
			r.partner = sender // a DM's other person
			if r.name == "" {
				r.name = name
			}
		}
	})
}

// onReaction is one reaction: a key put on an event. (It is taken back by
// redacting the reaction event: onRedaction.)
func (s *Source) onReaction(ctx context.Context, evt *event.Event) {
	rel := evt.Content.AsReaction().RelatesTo
	if rel.Type != event.RelAnnotation || rel.EventID == "" || rel.Key == "" {
		return
	}
	target, key := rel.EventID.String(), rel.Key
	id, sender, txn := evt.ID.String(), evt.Sender.String(), evt.Unsigned.TransactionID
	s.post(func() {
		r := s.roomOf(evt.RoomID.String())
		// The echo of a reaction the user made from here: it is already
		// shown, under a local ID, which this one takes over.
		if local, mine := r.reactTxn[txn]; mine && sender == s.me.ID {
			r.unreact(local)
			delete(r.reactTxn, txn)
		}
		r.react(target, id, sender, key)
		s.clearLock(r, id) // a reaction that was waiting for its keys is no tile
	})
}

// onRedaction is an event taken back (deleted): it goes from the timeline,
// with its reactions and edit — and, if it was a lock tile or waiting for
// keys, from the count of those.
func (s *Source) onRedaction(ctx context.Context, evt *event.Event) {
	target := evt.Redacts
	if c := evt.Content.AsRedaction(); c != nil && c.Redacts != "" {
		target = c.Redacts
	}
	if target == "" {
		return
	}
	id := target.String()
	s.post(func() {
		r := s.roomOf(evt.RoomID.String())
		if i := r.indexOf(id); i >= 0 {
			r.removeAt(i)
		}
		if _, raw := r.raw[id]; raw {
			delete(r.raw, id)
			s.encrypted--
		}
		delete(r.reactions, id) // the reactions on the event that went
		r.unreact(id)           // or the event was a reaction, taken back
		delete(r.edits, id)
	})
}

// clearLock removes what a room kept of an event that was waiting for its
// keys — the "waiting" tile and the raw copy — when it turns out to be
// something that is not a tile of its own (an edit, a reaction). UI thread.
func (s *Source) clearLock(r *room, id string) {
	if _, raw := r.raw[id]; raw {
		delete(r.raw, id)
		s.encrypted--
	}
	if i := r.indexOf(id); i >= 0 && r.events[i].Kind == model.EventUndecryptable {
		r.removeAt(i)
	}
}

// onEncrypted is a message that did not decrypt: the crypto helper
// re-dispatches the ones that do, as plain m.room.message. The raw
// event is kept for the retry once verification or the key backup
// brings the session.
func (s *Source) onEncrypted(ctx context.Context, evt *event.Event) {
	if evt.Content.AsEncrypted() == nil {
		return // it decrypted and was dispatched as m.room.message
	}
	e := undecryptable(evt)
	txn := evt.Unsigned.TransactionID
	s.post(func() {
		r := s.roomOf(evt.RoomID.String())
		// The echo of what the user just sent from here: the text is
		// already on screen; this tile would only say it is waiting for
		// keys for words they typed a moment ago.
		if evt.Sender.String() == s.me.ID && txn != "" && r.adoptEcho(txn, e.ID) {
			return
		}
		if _, reaction := r.reactTxn[txn]; reaction && evt.Sender.String() == s.me.ID {
			return // the echo of a reaction made here: shown already, no tile for it
		}
		// Already shown readable (a sync that repeats an event): nothing.
		if i := r.indexOf(e.ID); i >= 0 && r.events[i].Kind != model.EventUndecryptable {
			return
		}
		if evt.Mautrix.EventSource&sourceHistory != 0 {
			r.insertOlder(e)
		} else {
			r.appendEvent(e, s.nameOf(evt.Sender.String()), false)
		}
		if age := time.Since(e.Time); age < unreadableGrace && s.update != nil {
			// Hidden for now (Timeline): show it, as waiting, if its key
			// has not come by then.
			time.AfterFunc(unreadableGrace-age+200*time.Millisecond, func() {
				s.update(s.changed)
			})
		}
		if r.raw == nil {
			r.raw = map[string]*rawEvent{}
		}
		if _, dup := r.raw[e.ID]; !dup {
			s.encrypted++
		}
		raw := &rawEvent{evt: evt, tile: e}
		if evt.Mautrix.EventSource&sourceHistory != 0 {
			raw.since = time.Now() // paged in just now: give the key backup a look first
		}
		r.raw[e.ID] = raw
		select {
		case s.keyKick <- struct{}{}:
		default:
		}
	})
}

// onMember is a membership change: who is in the room, who they are, and
// a quiet system line — the user's own joins and leaves say so too — from
// the timeline, never from the initial sync's state dump, which would flood
// every room with its whole history.
func (s *Source) onMember(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsMember()
	if c == nil || evt.StateKey == nil {
		return
	}
	who := *evt.StateKey
	name := nameFor(c.Displayname, who)
	line, isLine := memberLine(evt, s.me.ID)
	fromTimeline := evt.Mautrix.EventSource&event.SourceTimeline != 0
	s.post(func() {
		if who == s.me.ID {
			s.setMe(name, string(c.AvatarURL))
		}
		if p, ok := s.people[who]; ok {
			p.Name, p.Avatar = name, string(c.AvatarURL)
		} else {
			s.people[who] = &model.Person{ID: who, Name: name, Avatar: string(c.AvatarURL), Contact: true}
		}
		r := s.roomOf(evt.RoomID.String())
		if !r.hasSummary { // with the server's count, the events only repeat it
			switch c.Membership {
			case event.MembershipJoin:
				r.members++
			case event.MembershipLeave, event.MembershipBan:
				r.members--
			}
		}
		if isLine && fromTimeline {
			r.appendEvent(line, name, false)
		}
	})
}

// onRoomName, onTopic, onAlias and onEncryption carry the room's shape.
func (s *Source) onRoomName(ctx context.Context, evt *event.Event) {
	name := evt.Content.AsRoomName().Name
	s.post(func() { s.roomOf(evt.RoomID.String()).name = name })
}

// onRoomAvatar is the room's (or space's) picture, an mxc:// URL.
func (s *Source) onRoomAvatar(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsRoomAvatar()
	if c == nil {
		return
	}
	url := string(c.URL)
	s.post(func() { s.roomOf(evt.RoomID.String()).avatar = url })
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

// onCreate carries a room's birth: a space (a community) is a room of
// type m.space.
func (s *Source) onCreate(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsCreate()
	if c == nil || c.Type != event.RoomTypeSpace {
		return
	}
	s.post(func() { s.roomOf(evt.RoomID.String()).space = true })
}

// onSpaceChild is a room listed in a space. A repeat updates the entry —
// its order may have changed — without duplicating it.
func (s *Source) onSpaceChild(ctx context.Context, evt *event.Event) {
	if evt.StateKey == nil {
		return
	}
	c := evt.Content.AsSpaceChild()
	child := spaceChild{room: *evt.StateKey}
	if c != nil {
		child.order = c.Order
	}
	space := evt.RoomID.String()
	s.post(func() {
		r := s.roomOf(space)
		for i := range r.children {
			if r.children[i].room == child.room {
				r.children[i].order = child.order
				spaceSort(r.children)
				return
			}
		}
		r.children = append(r.children, child)
		spaceSort(r.children)
	})
}

// onSpaceParent is a room saying which space it belongs to: the parent's
// ID is the state key. A canonical claim wins over the spaces' own
// lists, which keeps a room from flickering between two spaces that
// both list it.
func (s *Source) onSpaceParent(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsSpaceParent()
	if evt.StateKey == nil || c == nil || *evt.StateKey == "" {
		return
	}
	room, parent := evt.RoomID.String(), *evt.StateKey
	canonical := c.Canonical
	s.post(func() {
		r := s.roomOf(room)
		if canonical || r.parent == "" {
			r.parent = parent
		}
	})
}

// spaceSort orders a space's children by their order field, the ones
// without one after those with.
func spaceSort(children []spaceChild) {
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].order != children[j].order {
			if children[i].order == "" {
				return false
			}
			if children[j].order == "" {
				return true
			}
			return children[i].order < children[j].order
		}
		return false
	})
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

// onReceipt is a read marker: the user's own clears the room, and
// everyone else's public one says how far they have read.
func (s *Source) onReceipt(ctx context.Context, evt *event.Event) {
	receipts := evt.Content.AsReceipt()
	if receipts == nil {
		return
	}
	type mark struct {
		user string
		at   readMark
	}
	var others []mark
	mine := false
	for eventID, byType := range *receipts {
		for typ, users := range byType {
			for user, rr := range users {
				if rr.ThreadID != "" && rr.ThreadID != event.ReadReceiptThreadMain {
					continue // a thread's own marker: the room's timeline is the main one
				}
				switch {
				case user.String() == s.me.ID:
					// The user's own, public or private (from here or
					// another device): the room is read.
					mine = mine || typ == event.ReceiptTypeRead || typ == event.ReceiptTypeReadPrivate
				case typ == event.ReceiptTypeRead: // others' private ones never reach us
					others = append(others, mark{user.String(), readMark{Event: eventID.String(), TS: rr.Timestamp.UnixMilli()}})
				}
			}
		}
	}
	if !mine && len(others) == 0 {
		return
	}
	room := evt.RoomID.String()
	s.post(func() {
		r, ok := s.rooms[room]
		if !ok {
			return
		}
		if mine {
			r.unread, r.mentions = 0, 0
		}
		for _, m := range others {
			if old, had := r.reads[m.user]; had && old.TS > m.at.TS {
				continue // an older receipt arriving late
			}
			if r.reads == nil {
				r.reads = map[string]readMark{}
			}
			r.reads[m.user] = m.at
		}
		s.markDirty(room)
	})
}

// onPresence is who is around, the user included.
func (s *Source) onPresence(ctx context.Context, evt *event.Event) {
	c := evt.Content.AsPresence()
	if c == nil {
		return
	}
	who := evt.Sender.String()
	presence, msg, name, ago := presenceOf(c.Presence), c.StatusMessage, nameFor(c.Displayname, who), c.LastActiveAgo
	s.post(func() { s.setPresence(who, name, presence, msg, ago) })
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
			s.dirtyMeta = true
			if r, ok := s.rooms[p.room]; ok {
				s.markDirty(p.room)
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

// redecryptAll retries every still-encrypted event — run after
// verification or a key restore, when the sessions may just have
// arrived. It runs off the UI thread; what decrypts replaces its
// "waiting" tile.
func (s *Source) redecryptAll() {
	if s.helper == nil {
		return
	}
	ctx := context.Background()
	for _, snap := range s.snapshotRaw() {
		for evtID, raw := range snap.raw {
			decrypted, err := s.helper.Decrypt(ctx, raw.evt)
			if err != nil {
				continue // still waiting for keys
			}
			s.applyDecrypted(snap, evtID, decrypted)
		}
	}
}

// applyDecrypted puts a late-decrypted event in place of its "waiting"
// tile. Off the UI thread.
func (s *Source) applyDecrypted(snap *rawSnapshot, evtID string, decrypted *event.Event) {
	// Through the same handlers a live event takes, as the crypto helper
	// dispatches one it decrypts on arrival: whatever it turns out to be —
	// a message, an edit of one, a reaction — is handled as that, and not
	// shown as a message of its own.
	if d, ok := s.cli.Syncer.(mautrix.DispatchableSyncer); ok {
		d.Dispatch(context.Background(), decrypted)
	}
}

// rawSnapshot is one room's still-encrypted events with the names the
// tiles need, taken on the UI thread for the goroutine's loop.
type rawSnapshot struct {
	id    string
	raw   map[string]*rawEvent
	names map[string]string
}

// snapshotRaw copies the encrypted events for redecryptAll's goroutine,
// reading the rooms on the UI thread alone.
func (s *Source) snapshotRaw() []*rawSnapshot {
	if s.update == nil {
		return s.collectRaw()
	}
	done := make(chan []*rawSnapshot, 1)
	s.update(func() { done <- s.collectRaw() })
	return <-done
}

func (s *Source) collectRaw() []*rawSnapshot {
	var out []*rawSnapshot
	for _, r := range s.rooms {
		if len(r.raw) == 0 {
			continue
		}
		// The maps are written on the UI thread alone; the goroutine
		// reads only the copies taken here.
		raw := make(map[string]*rawEvent, len(r.raw))
		names := map[string]string{}
		for k, v := range r.raw {
			raw[k] = v
			names[v.evt.Sender.String()] = s.nameOf(v.evt.Sender.String())
		}
		out = append(out, &rawSnapshot{id: r.id, raw: raw, names: names})
	}
	return out
}

// ---- the writes -----------------------------------------------------------------------

// Send posts a message to a room: Markdown in, the homeserver's event
// out. The client's Crypto hook encrypts it where the room is; the sync
// loop echoes it back into the timeline.
func (s *Source) Send(room, markdown, replyTo string) { s.sendMessage(room, markdown, replyTo, "") }

// SendInThread sends a reply in the thread of root (a thread starts with its
// first reply). Clients without threads see it as a reply to the thread's
// latest message.
func (s *Source) SendInThread(room, root, markdown string) {
	s.sendMessage(room, markdown, "", root)
}

var _ model.Threads = (*Source)(nil)

// sendMessage sends a Markdown message: a reply to one (replyTo), in a thread
// (threadRoot), or plain.
func (s *Source) sendMessage(room, markdown, replyTo, threadRoot string) {
	if s.cli == nil {
		return
	}
	content := contentOf(markdown)
	if shortcodeRE.MatchString(content.Body) {
		// ":wave:" of a custom emoji pack goes as its picture.
		html := content.FormattedBody
		if html == "" {
			html = format.HTML(format.Parse(markdown))
		}
		if withEmoji := s.withCustomEmoji(room, html); withEmoji != html {
			content.FormattedBody, content.Format = withEmoji, event.FormatHTML
		}
	}
	if threadRoot != "" {
		fallback := threadRoot
		if r, ok := s.rooms[room]; ok {
			for i := len(r.events) - 1; i >= 0; i-- { // the thread's latest, for clients without threads
				if e := r.events[i]; e.ThreadRoot == threadRoot && !strings.HasPrefix(e.ID, localPrefix) {
					fallback = e.ID
					break
				}
			}
		}
		content.RelatesTo = (&event.RelatesTo{}).SetThread(id.EventID(threadRoot), id.EventID(fallback))
		if replyTo != "" {
			content.RelatesTo.SetReplyTo(id.EventID(replyTo))
		}
	} else if replyTo != "" {
		content.RelatesTo = (&event.RelatesTo{}).SetReplyTo(id.EventID(replyTo))
	}
	// The message shows at once, pending, under a local ID; it takes the
	// server's ID when the send returns, and the sync's echo of it then
	// lands on the same event. Sending itself (finding the room's devices,
	// sharing the key) can take seconds the first time in a room.
	txn := s.cli.TxnID()
	local := localPrefix + txn
	pending := model.Event{ID: local, Room: room, Sender: s.me.ID, Time: time.Now(), Kind: model.EventText,
		Body: content.Body, Formatted: content.FormattedBody, Plain: content.FormattedBody == "",
		ReplyTo: replyTo, ThreadRoot: threadRoot, Pending: true}
	s.post(func() { s.roomOf(room).appendEvent(pending, "You", true) })
	go func() {
		resp, err := s.cli.SendMessageEvent(context.Background(), id.RoomID(room), event.EventMessage, content,
			mautrix.ReqSendEvent{TransactionID: txn})
		s.post(func() {
			r := s.roomOf(room)
			if err != nil {
				r.failLocal(local, "Not sent: "+plain(err))
			} else {
				r.settleLocal(local, resp.EventID.String())
			}
		})
	}()
}

// React puts a reaction on an event, or takes it back with an empty key.
//
// It toggles: the user's own reaction with that key is taken back (the
// reaction event is redacted), otherwise it is made. Each shows at once; the
// server's answer settles it, and a refusal undoes it with a quiet line in
// the room.
func (s *Source) React(room, eventID, key string) {
	if s.cli == nil || key == "" || eventID == "" {
		return
	}
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	me := s.me.ID
	if mine := r.myReaction(eventID, me, key); mine != "" {
		r.unreact(mine)
		s.markDirty(room)
		// One still being sent has no ID on the server yet: its answer
		// finds it gone and takes it back then.
		if !strings.HasPrefix(mine, localPrefix) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if _, err := s.cli.RedactEvent(ctx, id.RoomID(room), id.EventID(mine)); err != nil {
					s.notice(room, "Could not take the reaction back: "+plain(err))
				}
			}()
		}
		return
	}

	txn := s.cli.TxnID()
	local := localPrefix + txn
	if r.reactTxn == nil {
		r.reactTxn = map[string]string{}
	}
	r.reactTxn[txn] = local
	r.react(eventID, local, me, key)
	s.markDirty(room)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		content := &event.ReactionEventContent{RelatesTo: event.RelatesTo{Type: event.RelAnnotation, EventID: id.EventID(eventID), Key: key}}
		resp, err := s.cli.SendMessageEvent(ctx, id.RoomID(room), event.EventReaction, content, mautrix.ReqSendEvent{TransactionID: txn})
		s.post(func() {
			rr := s.roomOf(room)
			delete(rr.reactTxn, txn)
			switch {
			case err != nil:
				rr.unreact(local)
				s.notice(room, "Could not react: "+plain(err))
			case rr.hasReaction(resp.EventID.String()):
				rr.unreact(local) // the echo got here first and replaced it
			case rr.hasReaction(local):
				rr.renameReaction(local, resp.EventID.String())
			default:
				// The user took it back while it was being sent.
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_, _ = s.cli.RedactEvent(ctx, id.RoomID(room), resp.EventID)
				}()
			}
		})
	}()
}

// MarkRead reads a room to its end: the marker goes to the server, the
// room clears at once.
func (s *Source) MarkRead(room string) {
	if s.cli == nil {
		return
	}
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	last := ""
	for i := len(r.events) - 1; i >= 0; i-- {
		if !strings.HasPrefix(r.events[i].ID, localPrefix) { // not one still being sent
			last = r.events[i].ID
			break
		}
	}
	if last == "" {
		return
	}
	r.unread, r.mentions = 0, 0
	if r.markedUnread {
		r.markedUnread = false
		go s.setMarkedUnread(room, false) // read: the mark goes, on every device
	}
	s.markDirty(room)
	receipt := event.ReceiptTypeReadPrivate // nobody sees it, unless the user chose so
	if s.publicReceipts {
		receipt = event.ReceiptTypeRead
	}
	go func() {
		// One request: the user's own place (what the unread counts and
		// their other devices follow), and the receipt of the chosen
		// kind.
		_ = s.cli.SetReadMarkers(context.Background(), id.RoomID(room), map[string]id.EventID{
			"m.fully_read":  id.EventID(last),
			string(receipt): id.EventID(last),
		})
	}()
}

// SetPublicReceipts chooses whether the read receipts the user sends are
// seen by others (off by default: private receipts keep the unread counts
// right on their devices and show nothing to anyone).
func (s *Source) SetPublicReceipts(public bool) { s.publicReceipts = public }

var _ model.ReceiptPrivacy = (*Source)(nil)

// contentOf is a Markdown message as Matrix sends it: the formatted body
// when the Markdown means anything, and the body, which a client without
// formatting shows — the Markdown as typed (as Element sends it, so "- a" is
// still a list in the body and "**x**" still says so), with a mention
// chosen from the suggestions reduced to its name. A message with nothing to
// format has the plain text as its body, and no formatted one.
func contentOf(markdown string) *event.MessageEventContent {
	doc := format.Parse(markdown)
	content := event.MessageEventContent{MsgType: event.MsgText, Body: format.Plain(doc)}
	if format.NeedsHTML(doc) {
		content.FormattedBody, content.Format = format.HTML(doc), event.FormatHTML
		content.Body = mentionLinkRE.ReplaceAllStringFunc(markdown, func(m string) string {
			label := mentionLinkRE.FindStringSubmatch(m)[1]
			return strings.NewReplacer(`\[`, "[", `\]`, "]").Replace(label)
		})
	}
	if ids := mentionedUsers(doc); len(ids) > 0 {
		content.Mentions = &event.Mentions{UserIDs: ids} // they are notified, as the spec has it
	}
	return &content
}

// mentionLinkRE is a mention as the composer writes it: [@Name](matrix.to
// link of a user) or [#room](matrix.to link of a room), the label possibly
// with escaped brackets.
var mentionLinkRE = regexp.MustCompile(`\[((?:[^\]\\]|\\.)*)\]\(https://matrix\.to/#/[@#!][^)\s]*\)`)

// mentionedUsers are the people a message links to as matrix.to user
// links (what the composer makes of a chosen @mention).
func mentionedUsers(n *format.Node) []id.UserID {
	var out []id.UserID
	seen := map[id.UserID]bool{}
	var walk func(*format.Node)
	walk = func(n *format.Node) {
		if n.Kind == format.Link || n.RealMention() {
			if who, ok := strings.CutPrefix(n.Href, "https://matrix.to/#/@"); ok && who != "" {
				uid := id.UserID("@" + strings.SplitN(who, "?", 2)[0])
				if !seen[uid] {
					seen[uid] = true
					out = append(out, uid)
				}
			}
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	walk(n)
	return out
}
