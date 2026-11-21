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
	Body      string
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
