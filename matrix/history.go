package matrix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/model"
)

// historyPage is how many events one page of history asks for.
const historyPage = 40

// HasOlder says whether the room has older messages than the app holds
// (or may have: not known until they are asked for). UI thread.
func (s *Source) HasOlder(room string) bool {
	r, ok := s.rooms[room]
	return ok && !r.backDone && len(r.events) > 0
}

// LoadOlder pages one more stretch of a room's history in, above what the
// app holds: the events are placed by time (they are not news: no unread,
// no preview), decrypted if their keys are there, and shown as waiting for
// them if not. One page at a time per room; nothing when the room has no
// older events. UI thread; the work is off it.
func (s *Source) LoadOlder(room string) { s.LoadOlderBy(room, historyPage) }

// seekPage is the most a seek asks for at once (servers cap /messages; 1000
// on Synapse).
const seekPage = 500

// LoadOlderBy is LoadOlder with a page of n events: a jump to a message
// months back (an old pin) asks for big pages, not hundreds of small ones.
func (s *Source) LoadOlderBy(room string, n int) {
	if s.cli == nil {
		return
	}
	n = min(max(n, 1), seekPage)
	r, ok := s.rooms[room]
	if !ok || r.backDone || r.backBusy {
		return
	}
	oldest := ""
	for _, e := range r.events {
		if !strings.HasPrefix(e.ID, localPrefix) {
			oldest = e.ID
			break
		}
	}
	if oldest == "" && r.back == "" {
		return
	}
	r.backBusy = true
	token := r.back
	go s.pageBack(room, oldest, token, n)
}

// debugOn: SLOPEMENT_DEBUG=1 logs history paging and key lookups to stderr.
var debugOn = os.Getenv("SLOPEMENT_DEBUG") != ""

func debugf(format string, args ...any) {
	if debugOn {
		fmt.Fprintf(os.Stderr, "slopement: "+format+"\n", args...)
	}
}

// pageBack fetches and applies one page. The token to page from is the
// room's (kept from the last page), or — the first time, or after the cache
// started over — the point just before the oldest event the app holds.
func (s *Source) pageBack(room, oldest, token string, n int) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	began := time.Now()
	failed := func(err error) {
		debugf("history %s: page failed: %v", room, err)
		s.post(func() {
			if r, ok := s.rooms[room]; ok {
				r.backBusy = false // the next scroll to the top tries again
			}
		})
	}
	if token == "" {
		around, err := s.cli.Context(ctx, id.RoomID(room), id.EventID(oldest), nil, 0)
		if err == nil && around.Start == "" {
			err = errors.New("no token before " + oldest)
		}
		if err != nil {
			failed(err)
			return
		}
		token = around.Start
	}
	resp, err := s.cli.Messages(ctx, id.RoomID(room), token, "", mautrix.DirectionBackward,
		&mautrix.FilterPart{LazyLoadMembers: true}, n)
	if err != nil {
		failed(err)
		return
	}
	if debugOn {
		var from, to time.Time
		if k := len(resp.Chunk); k > 0 {
			from, to = time.UnixMilli(resp.Chunk[k-1].Timestamp), time.UnixMilli(resp.Chunk[0].Timestamp)
		}
		debugf("history %s: %d events asked, %d came (%s … %s) in %s, more: %v",
			room, n, len(resp.Chunk), from.Format(time.DateTime), to.Format(time.DateTime),
			time.Since(began).Round(time.Millisecond), resp.End != "")
	}

	// The whole page lands as one update.
	s.beginBatch()
	for _, evt := range resp.Chunk {
		s.historyEvent(ctx, room, evt)
	}
	// The history ends where the server gives no token to go on from. A
	// page can come back empty with one (what it covered was filtered out,
	// or hidden by the room's history visibility): there is more before it.
	end := resp.End
	s.post(func() {
		r, ok := s.rooms[room]
		if !ok {
			return
		}
		r.backBusy = false
		r.back = end
		if end == "" || end == token { // nothing further back
			r.backDone = true
		}
		s.markDirty(room)
	})
	s.endBatch("", nil)
}

// historyEvent applies one event of a page through the handlers a live
// one takes, marked as history. Only what shows in a timeline: messages
// (and their edits), reactions, and what could not be decrypted; state and
// the rest stay as the sync left them — an older member event must not
// undo a newer one.
func (s *Source) historyEvent(ctx context.Context, room string, evt *event.Event) {
	evt.RoomID = id.RoomID(room)
	if err := evt.Content.ParseRaw(evt.Type); err != nil && evt.Type != event.EventEncrypted {
		return
	}
	evt.Mautrix.EventSource = event.SourceJoin | event.SourceTimeline | sourceHistory
	if evt.Type == event.EventEncrypted {
		if s.helper != nil {
			if dec, err := s.helper.Decrypt(ctx, evt); err == nil {
				dec.Mautrix.EventSource |= sourceHistory
				s.historyEvent2(ctx, dec)
				return
			}
		}
		s.onEncrypted(ctx, evt) // waits for its keys, like any other
		return
	}
	s.historyEvent2(ctx, evt)
}

func (s *Source) historyEvent2(ctx context.Context, evt *event.Event) {
	switch evt.Type {
	case event.EventMessage:
		s.onMessage(ctx, evt)
	case event.EventReaction:
		s.onReaction(ctx, evt)
	}
}

var _ model.History = (*Source)(nil)
