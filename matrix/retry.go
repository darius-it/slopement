package matrix

import "slopement/model"

// A message that did not send can be sent again: the way it was sent is kept
// under its (local) ID while it sits there failed, and "Try again" runs it. The
// failed copy goes and the message comes back as a new one at the end of the
// room, sending, as if it had just been written.

// rememberRetry keeps how to send a failed message again. UI thread.
func (s *Source) rememberRetry(room, local string, again func()) {
	r := s.roomOf(room)
	if r.retries == nil {
		r.retries = map[string]func(){}
	}
	r.retries[local] = again
}

// CanRetry says a failed message can be sent again.
func (s *Source) CanRetry(room, id string) bool {
	r, ok := s.rooms[room]
	return ok && r.retries[id] != nil
}

// Retry sends a failed message again.
func (s *Source) Retry(room, id string) {
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	again := r.retries[id]
	if again == nil {
		return
	}
	delete(r.retries, id)
	if i := r.indexOf(id); i >= 0 {
		r.removeAt(i)
	}
	s.markDirty(room)
	again()
}

var _ model.Retrier = (*Source)(nil)
