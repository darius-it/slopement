package app

import (
	"testing"
	"time"

	"slopement/model"
	"slopement/model/mock"
	"slopement/picker"
)

// beforeJoinSource has locked messages from before the user's own join line,
// and one after it.
type beforeJoinSource struct {
	model.Source
	room string
}

func (s *beforeJoinSource) Timeline(room string) []model.Event {
	if room != s.room {
		return s.Source.Timeline(room)
	}
	t := time.Now().Add(-time.Hour)
	me := s.Source.Me().ID
	return []model.Event{
		{ID: "$u1", Room: room, Time: t, Sender: "@x:example.org", Kind: model.EventUndecryptable},
		{ID: "$u2", Room: room, Time: t.Add(time.Minute), Sender: "@x:example.org", Kind: model.EventUndecryptable},
		{ID: "$j", Room: room, Time: t.Add(2 * time.Minute), Sender: me, Kind: model.EventSystem, Body: "You joined", Membership: "join"},
		{ID: "$u3", Room: room, Time: t.Add(3 * time.Minute), Sender: "@x:example.org", Kind: model.EventUndecryptable},
	}
}

// Locked messages from before the user joined are one quiet line; one after
// the join still waits for its keys.
func TestMessagesBeforeJoinAreOneLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := mock.New(time.Now())
	m.Delay = 0
	a := New(&beforeJoinSource{Source: m, room: "!crit"}, picker.New(nil))
	locked, notes := 0, 0
	for _, r := range a.rows("!crit") {
		if r.ev == nil {
			continue
		}
		switch {
		case r.ev.Kind == model.EventUndecryptable:
			locked++
		case r.ev.Body == "Messages from before you joined can't be read here":
			notes++
		}
	}
	if locked != 1 || notes != 1 {
		t.Fatalf("locked %d, notes %d; want 1 and 1", locked, notes)
	}
}
