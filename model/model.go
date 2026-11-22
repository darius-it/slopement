// Package model is what the app shows and does, independent of where it
// comes from: rooms, people, timelines and the actions on them, behind
// Source. The Matrix layer (to come) and the mock (package mock) both
// implement Source, so the app's UI never touches the protocol.
//
// Everything here is read and changed on the UI thread: a Source does its
// I/O elsewhere and applies the results through the update function it is
// given (Window.Update), then calls the change callback.
package model

import "time"

// Presence is whether someone is around.
type Presence int

const (
	Offline Presence = iota
	Online
	Away
)

// Status is a custom status: an emoji and words, until a time (zero: until
// cleared). Others never see Until; it only hides the status once past.
type Status struct {
	Emoji, Text string
	Until       time.Time
}

// Set reports whether the status shows at now.
func (s Status) Set(now time.Time) bool {
	return (s.Emoji != "" || s.Text != "") && (s.Until.IsZero() || now.Before(s.Until))
}

// Trust is how far someone's devices are verified.
type Trust int

const (
	TrustUnknown Trust = iota
	TrustVerified
	TrustWarning // a device of theirs isn't verified
)

// Person is a user.
type Person struct {
	ID       string // MXID
	Name     string
	Avatar   string // mxc:// URL; empty for initials
	Presence Presence
	Status   Status
	LastSeen time.Time
	Pronouns string
	Trust    Trust
	// Contact: someone the user has a DM with (listed in People).
	Contact bool
}

// RoomKind is how a room is shown.
type RoomKind int

const (
	KindDM    RoomKind = iota // m.direct: one other person
	KindGroup                 // a small group chat, in no community
	KindRoom                  // a room of a community, or a large room
)

// Room is a room (a DM, a group chat or a community's room).
type Room struct {
	ID        string
	Kind      RoomKind
	Name      string
	Topic     string
	Avatar    string
	Partner   string // a DM's other person (MXID)
	Alias     string // its address (#room:server), when it has one
	Community string // the community (space) it belongs to; "" for none
	Members   int
	Encrypted bool

	Unread, Mentions int
	Favourite        bool
	LowPriority      bool
	Muted            bool
	MutedUntil       time.Time // zero while Muted: until turned back on

	LastActivity time.Time
	Preview      string   // the last message, "Mira: hi"
	Typing       []string // the names of who is typing
	Call         bool     // a call is going on
}

// Community is a space: a home and its rooms.
type Community struct {
	ID, Name, About string
	Avatar          string
	Members, Online int
	Favourite       bool
	Rooms           []string // room IDs, in the space's order
	Parent          string   // the community this one is a sub-space of; "" at the top
	Spaces          []string // its sub-spaces (community IDs), in the space's order
}

// EventKind is what an event shows as.
type EventKind int

const (
	EventText EventKind = iota
	EventNotice
	EventEmote
	EventMedia
	EventPoll
	EventSystem        // joins, renames, topic changes… (one quiet line)
	EventUndecryptable // "unable to decrypt"
)

// Event is a timeline event.
type Event struct {
	ID     string
	Room   string
	Sender string // MXID
	Time   time.Time
	Kind   EventKind
	// Body is the text as Markdown (the formatted body converted), or the
	// system line.
	Body string
	// Formatted is the message's HTML body as the sender wrote it
	// (org.matrix.custom.html), shown in place of Body; Plain says there is
	// none and Body is plain text, shown as typed. With neither, Body is
	// Markdown (the mock's).
	Formatted string
	Plain     bool
	Media     []Media
	ReplyTo   string // the event replied to
	// ThreadRoot is the message a thread reply belongs to ("" for a message
	// of the room itself); Thread, on a thread's root, sums its replies up.
	ThreadRoot string
	Thread     *Thread
	Reactions  []Reaction
	Edited    bool
	Pending   bool
	// Failed: the user's message could not be sent; Error says why. Until
	// then a message shows as sent — only a failure is ever marked.
	Failed bool
	Error  string
	Poll      *Poll
	// Mentions is set when it mentions the user.
	Mentions bool
	// ReadBy are the others (MXIDs) whose read marker sits at this event:
	// each person is on the last event they have read, never on every
	// one before it.
	ReadBy []string
}

// Thread sums up a thread on its root message.
type Thread struct {
	Replies      int
	Last         time.Time
	LastSender   string   // the MXID of the latest reply's author
	Participants []string // who replied, the first to reply first
}

// Threads is what a Source may offer: replies in a thread. Replies show in
// their thread, not among the room's messages, and the root says how many
// there are. SendInThread sends a reply in the thread of root.
type Threads interface {
	SendInThread(room, root, markdown string)
}

// Preview is what a page says about itself (OpenGraph), as the
// homeserver's URL preview gives it.
type Preview struct {
	URL, Site, Title, Description string
	Image                         string // an mxc:// URL on the homeserver
	W, H                          int
}

// Previewer is what a Source may offer: asking the homeserver what a link
// says about itself. It does the fetching, so the user's own address is not
// shown to the site; done gets the answer on any goroutine.
type Previewer interface {
	LinkPreview(url string, done func(Preview, error))
}

// Actions is what a Source may offer on messages: editing one of the
// user's own, deleting one, pinning (and unpinning) one to its room. Each
// takes effect at once and is confirmed by the server after; Pinned lists a
// room's pinned events in the order they were pinned.
type Actions interface {
	Edit(room, event, markdown string)
	Redact(room, event string)
	Pin(room, event string, on bool)
	IsPinned(room, event string) bool
	Pinned(room string) []string
}

// Typist is what a Source may offer: telling a room the user is typing (or
// that they stopped), as the other people there see "… is typing".
type Typist interface {
	SetTyping(room string, typing bool)
}

// Notice is a message that may deserve a desktop notification: new (live,
// not history), from someone else, in a room that is not muted, and a chat
// with a person or a mention of the user. The app decides whether to show
// it (its setting, the window's focus, the room being open).
type Notice struct {
	Room, Event string
	Title       string // who or where: the person, or the room
	Body        string // what it said (a group's: "Name: …")
	Sender      string // the MXID
	DM          bool
	Mention     bool
}

// Notifier is what a Source may offer: telling the app of messages that
// may notify, on the UI thread.
type Notifier interface {
	OnNotify(fn func(Notice))
}

// RoomSpec is a room (or community) to make.
type RoomSpec struct {
	Name, Topic string
	Space       bool // a community
	Public      bool // open to anyone, with an address; else private and encrypted
	Invite      []string
}

// PublicRoom is a room of a homeserver's directory.
type PublicRoom struct {
	ID, Alias, Name, Topic, Avatar string
	Members                        int
	Space                          bool
	Joined                         bool // the user is in it already
}

// Invite is an invitation waiting.
type Invite struct {
	ID, Name, Avatar, Topic string
	Inviter                 string // the MXID of who asked
	DM, Space               bool
	Time                    time.Time
}

// RoomManager is what a Source may offer: making rooms and communities,
// joining by address or link, inviting people, the invitations received, and
// looking for people and public rooms. Results come on the UI thread: room
// IDs ("" with an error text when it did not work).
type RoomManager interface {
	CreateRoom(spec RoomSpec, done func(room, err string))
	JoinRoom(target string, done func(room, err string))
	Invite(room, person string, done func(err string))
	SearchPeople(query string, done func(people []Person, err string))
	PublicRooms(query, server string, done func(rooms []PublicRoom, err string))
	Invites() []Invite
	AcceptInvite(room string, done func(room, err string))
	DeclineInvite(room string)
}

// Profile is what a Source may offer: changing who the user is to others.
// Each shows at once where it can; done is called on the UI thread with ""
// or the reason it did not work.
type Profile interface {
	SetDisplayName(name string, done func(err string))
	// SetAvatar sets the picture from an image file (any common format; the
	// middle square of it, scaled down).
	SetAvatar(f File, done func(err string))
	RemoveAvatar(done func(err string))
}

// UserColors is what a Source may offer: the colors chosen for names (an
// index of the palette, by MXID), kept with the account so every session
// shows them. SetUserColor with -1 goes back to the automatic color.
type UserColors interface {
	UserColors() map[string]int
	SetUserColor(mxid string, index int)
}

// SessionManager is what a Source may offer: the account's devices.
// RefreshSessions asks for them (Sessions then has them); SignOutSession
// signs another out, or says (needsBrowser) the homeserver wants that
// confirmed on its account page, which AccountPage names ("" if none).
type SessionManager interface {
	RefreshSessions()
	SignOutSession(id string, done func(err string, needsBrowser bool))
	AccountPage() string
}

// DMCreator is what a Source may offer: opening a DM with someone who has
// none yet. OpenDM answers only for DMs that exist; this makes one (off the
// UI thread) and calls done on it with the room's ID ("" if it could not be
// made).
type DMCreator interface {
	OpenDMAsync(person string, done func(room string))
}

// Pins is what a Source may offer beyond Actions: the pinned messages
// themselves, including those older than what is held, which RefreshPins
// asks the server for (once per room per run).
type Pins interface {
	RefreshPins(room string)
	// PinnedEvents are the room's pinned messages, in the order pinned.
	PinnedEvents(room string) []Event
}

// File is something the user sends: its name, its media type ("" to
// detect it) and its bytes.
type File struct {
	Name string
	Mime string
	Data []byte
}

// Uploader is what a Source may offer: sending a file or picture to a room
// (as a reply when replyTo is set). It shows at once, and a failure marks
// the message as the failure of any send.
type Uploader interface {
	SendFile(room string, f File, replyTo string)
}

// History is what a Source may offer: paging a room's older messages in
// above what it holds. HasOlder says there may be more; LoadOlder asks for
// a page (one at a time per room, a no-op when there is none), whose
// events arrive as the room's timeline changing.
type History interface {
	HasOlder(room string) bool
	LoadOlder(room string)
}

// HistorySeek is what a History may offer for a jump far back (an old
// pin): LoadOlder with a page of n events, so that months of history come
// in a few requests rather than hundreds.
type HistorySeek interface {
	LoadOlderBy(room string, n int)
}

// ReceiptPrivacy is what a Source may offer: whether the read receipts
// the user sends are public (others see them) or private (only the
// server's own unread counts and the user's other devices do).
type ReceiptPrivacy interface {
	SetPublicReceipts(public bool)
}

// Media is an attachment.
type Media struct {
	ID      string
	Kind    MediaKind
	URL     string // mxc:// (or https:// for a GIF by link)
	Thumb   string
	W, H    int
	Name    string
	Size    int64
	Length  time.Duration // video and audio
	Spoiler bool
	Secret  string // the source's own key material for an encrypted attachment; opaque
	Mime    string // its media type, as the sender said ("image/gif")
	Audio   bool   // an audio or voice message: it plays (Kind is still MediaFile, for a download)
	// Local is the picture's own bytes while it is being sent: it shows at
	// once, before the server has it. Never stored.
	Local []byte `json:"-"`
	// Square, when set, asks for a thumbnail of that many pixels a side
	// (cropped) rather than the whole file: a profile picture.
	Square int
}

// MediaLoader is what a Source may offer: the bytes of an attachment
// (downloaded and decrypted), delivered to done on any goroutine.
type MediaLoader interface {
	LoadMedia(m Media, done func(data []byte, err error))
}

// MediaKind is an attachment's kind.
type MediaKind int

const (
	MediaImage MediaKind = iota
	MediaVideo
	MediaGIF
	MediaFile
	MediaSticker
)

// Reaction is a reaction key with its count.
type Reaction struct {
	Key   string
	Count int
	Mine  bool
}

// Poll is an m.poll.start with its votes.
type Poll struct {
	Question string
	Answers  []PollAnswer
	Multiple bool // max_selections > 1
	Hidden   bool // undisclosed until ended
	Ends     time.Time
	Ended    bool
}

// PollAnswer is an answer and who voted for it (MXIDs, the user's own
// included).
type PollAnswer struct {
	ID, Text string
	Voters   []string
}

// Session is one of the user's devices.
type Session struct {
	ID, Name   string
	LastSeen   time.Time
	IP, Client string
	Verified   bool
	Current    bool
}

// Source is where the app's data comes from and where its actions go.
type Source interface {
	// SetUpdate gives the source the window's update function: work done
	// off the UI thread comes back through it.
	SetUpdate(update func(func()))
	// OnChange is called (on the UI thread) after anything changed.
	OnChange(fn func())

	Me() Person
	Rooms() []Room
	Room(id string) (Room, bool)
	Communities() []Community
	People() []Person
	Person(id string) (Person, bool)
	Timeline(room string) []Event
	Sessions() []Session

	Send(room, markdown, replyTo string)
	React(room, event, key string)
	Vote(room, event string, answer int)
	EndPoll(room, event string)
	CreatePoll(room string, p Poll)
	SetStatus(s Status)
	SetFavourite(room string, on bool)
	SetCommunityFavourite(community string, on bool)
	SetLowPriority(room string, on bool)
	// Mute mutes a room until a time (zero: until unmuted).
	Mute(room string, until time.Time)
	Unmute(room string)
	MarkRead(room string)
	MarkUnread(room string)
	Leave(room string)
	// OpenDM returns the DM with someone, creating it if needed.
	OpenDM(person string) string
	// Verify is this device's verification, or nil when the source has
	// none (the crypto layer's comes with its sync).
	Verify() Verify
}
