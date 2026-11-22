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
	Reactions []Reaction
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
