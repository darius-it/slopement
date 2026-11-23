package matrix

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// Private nicknames and notes about people are one account data event,
// com.slopement.person_notes: MXID → {nick, note}. Only the user's own
// sessions read it: nobody else sees what they call someone. (Account data is
// not end-to-end encrypted: the user's homeserver can read it.)

const personNotesData = "com.slopement.person_notes"

var personNotesType = event.Type{Type: personNotesData, Class: event.AccountDataEventType}

type storedNote struct {
	Nick string `json:"nick,omitempty"`
	Note string `json:"note,omitempty"`
}

// PersonNotes is the nicknames and notes the user gave, by MXID. UI thread;
// read only.
func (s *Source) PersonNotes() map[string]model.PersonNote { return s.personNotes }

// SetPersonNote gives someone a nickname and a note (both empty takes them
// away). It shows at once and is saved to the account.
func (s *Source) SetPersonNote(mxid string, n model.PersonNote) {
	n.Nick, n.Note = strings.TrimSpace(n.Nick), strings.TrimSpace(n.Note)
	// A new map each time: the app holds the old one, read only.
	next := make(map[string]model.PersonNote, len(s.personNotes)+1)
	for k, v := range s.personNotes {
		next[k] = v
	}
	if n.Nick == "" && n.Note == "" {
		delete(next, mxid)
	} else {
		next[mxid] = n
	}
	s.personNotes = next
	s.changed()
	if s.cli == nil {
		return
	}
	snapshot := make(map[string]storedNote, len(next))
	for k, v := range next {
		snapshot[k] = storedNote{v.Nick, v.Note}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.cli.SetAccountData(ctx, personNotesData, snapshot)
	}()
}

// onPersonNotes is the nicknames and notes as the account has them.
func (s *Source) onPersonNotes(ctx context.Context, evt *event.Event) {
	var m map[string]storedNote
	if json.Unmarshal(evt.Content.VeryRaw, &m) != nil {
		return
	}
	notes := make(map[string]model.PersonNote, len(m))
	for k, v := range m {
		notes[k] = model.PersonNote{Nick: v.Nick, Note: v.Note}
	}
	s.post(func() { s.personNotes = notes })
}

var _ model.PrivateNotes = (*Source)(nil)
