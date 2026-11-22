package matrix

import (
	"context"
	"time"

	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// typingTimeout is how long the server shows the user as typing without
// another notice: the app sends one again before it runs out, and one to
// stop when they stop.
const typingTimeout = 30 * time.Second

// SetTyping tells a room the user is typing, or has stopped. Fire and
// forget: a typing notice is worth nothing late, and nothing is said of one
// that failed.
func (s *Source) SetTyping(room string, typing bool) {
	if s.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = s.cli.UserTyping(ctx, id.RoomID(room), typing, typingTimeout)
	}()
}

var _ model.Typist = (*Source)(nil)
