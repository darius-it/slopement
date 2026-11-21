package matrix

import (
	"strings"
	"time"

	"slopement/model"
)

// Source is one signed-in account's data, before the sync loop: it knows
// who is signed in and nothing else — the app shows its empty states,
// and the sync milestone fills in the rooms, the timeline and sending.
type Source struct {
	me       model.Person
	update   func(func())
	onChange []func()
}

// NewSource is the app's view of a signed-in account.
func NewSource(s *Session) *Source {
	return &Source{me: model.Person{ID: string(s.UserID), Name: mxidName(string(s.UserID))}}
}

// mxidName is the local part of an MXID, until the profile (display
// name, avatar) arrives with the sync.
func mxidName(mxid string) string {
	local, _, _ := strings.Cut(strings.TrimPrefix(mxid, "@"), ":")
	return local
}

// SetUpdate gives the source the window's update function: work done
// off the UI thread comes back through it.
func (s *Source) SetUpdate(update func(func())) { s.update = update }

// OnChange is called (on the UI thread) after anything changed.
func (s *Source) OnChange(fn func()) { s.onChange = append(s.onChange, fn) }

func (s *Source) emit() {
	for _, fn := range s.onChange {
		fn()
	}
}

func (s *Source) Me() model.Person    { return s.me }
func (s *Source) Rooms() []model.Room { return nil }
func (s *Source) Room(string) (model.Room, bool) {
	return model.Room{}, false
}
func (s *Source) Communities() []model.Community { return nil }
func (s *Source) People() []model.Person         { return nil }
func (s *Source) Person(id string) (model.Person, bool) {
	if id == s.me.ID {
		return s.me, true
	}
	return model.Person{}, false
}
func (s *Source) Timeline(string) []model.Event { return nil }
func (s *Source) Sessions() []model.Session     { return nil }

// The writes are no-ops until sync lands: nothing to write them to yet.
func (s *Source) Send(room, markdown, replyTo string)             {}
func (s *Source) React(room, event, key string)                   {}
func (s *Source) Vote(room, event string, answer int)             {}
func (s *Source) EndPoll(room, event string)                      {}
func (s *Source) CreatePoll(room string, p model.Poll)            {}
func (s *Source) SetStatus(st model.Status)                       {}
func (s *Source) SetFavourite(room string, on bool)               {}
func (s *Source) SetCommunityFavourite(community string, on bool) {}
func (s *Source) SetLowPriority(room string, on bool)             {}
func (s *Source) Mute(room string, until time.Time)               {}
func (s *Source) Unmute(room string)                              {}
func (s *Source) MarkRead(room string)                            {}
func (s *Source) MarkUnread(room string)                          {}
func (s *Source) Leave(room string)                               {}
func (s *Source) OpenDM(person string) string                     { return "" }
