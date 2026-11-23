package matrix

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"

	"slopement/model"
)

// Private room names are one account data event, com.slopement.room_names:
// room ID → name. Only the user's own sessions read it: nobody else sees what
// they call a room. (Account data is not end-to-end encrypted: the user's
// homeserver can read it.)

const roomNamesData = "com.slopement.room_names"

var roomNamesType = event.Type{Type: roomNamesData, Class: event.AccountDataEventType}

// RoomNicknames is the private names by room ID. UI thread; read only.
func (s *Source) RoomNicknames() map[string]string { return s.roomNicks }

// SetRoomNickname gives a room a private name ("" takes it away). It shows at
// once and is saved to the account.
func (s *Source) SetRoomNickname(room, name string) {
	name = strings.TrimSpace(name)
	// A new map each time: the app holds the old one, read only.
	next := make(map[string]string, len(s.roomNicks)+1)
	for k, v := range s.roomNicks {
		next[k] = v
	}
	if name == "" {
		delete(next, room)
	} else {
		next[room] = name
	}
	s.roomNicks = next
	s.changed()
	if s.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.cli.SetAccountData(ctx, roomNamesData, next)
	}()
}

// onRoomNames is the private names as the account has them.
func (s *Source) onRoomNames(ctx context.Context, evt *event.Event) {
	var m map[string]string
	if json.Unmarshal(evt.Content.VeryRaw, &m) != nil {
		return
	}
	s.post(func() { s.roomNicks = m; s.changed() })
}

var _ model.PrivateRoomNames = (*Source)(nil)
