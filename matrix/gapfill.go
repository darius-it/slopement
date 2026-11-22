package matrix

import (
	"context"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// A sync that comes back "limited" has more events since the last one than
// the filter's timeline holds: it gives the newest, and a token (prev_batch)
// to fetch the ones before. A client that was closed for a while resumes
// from its cache and is left with a hole between what it held and what the
// sync gave — messages missing in between. fillGap closes it.

const (
	gapPage  = 100 // events asked for at once
	gapPages = 5   // so a hole of up to 500 events is filled; a bigger one is dropped, below
)

// gapStart notes, for each room whose sync is limited and which already held
// events, where the hole is, and has it filled. UI thread (the sync's
// listener posts it ahead of the events, so the room still holds only what it
// held before this sync).
func (s *Source) gapStart(room string, limited bool, prevBatch string) {
	if !limited || prevBatch == "" || s.cli == nil {
		return
	}
	r, ok := s.rooms[room]
	if !ok {
		return
	}
	// The newest event the room held: the hole begins after it.
	var heldID string
	var heldTS int64
	for i := len(r.events) - 1; i >= 0; i-- {
		if e := r.events[i]; !strings.HasPrefix(e.ID, localPrefix) {
			heldID, heldTS = e.ID, e.Time.UnixMilli()
			break
		}
	}
	if heldID == "" {
		return // nothing held: no hole, only the first page of history
	}
	r.backBusy = true // paging back waits for the hole: it must not start above it
	go s.fillGap(room, prevBatch, heldID, heldTS)
}

// fillGap pages back from the sync's prev_batch until it meets what the room
// held (heldID, or an event older than heldTS), applying the events as
// history. If the hole is larger than gapPages, the old events are dropped
// instead and the room's history continues from where the fill stopped:
// better a timeline that is whole and shorter than one with a hole.
func (s *Source) fillGap(room, token, heldID string, heldTS int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	reached := false
	oldest := int64(0) // the oldest event the fill applied
	for page := 0; page < gapPages && token != "" && !reached; page++ {
		resp, err := s.cli.Messages(ctx, id.RoomID(room), token, "", mautrix.DirectionBackward,
			&mautrix.FilterPart{LazyLoadMembers: true}, gapPage)
		if err != nil {
			debugf("gap %s: page failed: %v", room, err)
			break
		}
		s.beginBatch()
		for _, evt := range resp.Chunk { // newest first
			if evt.ID.String() == heldID || evt.Timestamp < heldTS {
				reached = true
				break
			}
			s.historyEvent(ctx, room, evt)
			oldest = evt.Timestamp
		}
		s.endBatch("", nil)
		if resp.End == "" || resp.End == token {
			token = ""
			break
		}
		token = resp.End
	}
	debugf("gap %s: reached what was held: %v (filled down to %s)", room, reached,
		time.UnixMilli(oldest).Format(time.DateTime))
	s.post(func() {
		r, ok := s.rooms[room]
		if !ok {
			return
		}
		r.backBusy = false
		if reached || oldest == 0 {
			return // whole again (or nothing came: the next scroll up asks again)
		}
		// Too big a hole: drop what was held from before it, and let history
		// continue from the fill's token.
		kept := r.events[:0]
		for _, e := range r.events {
			if strings.HasPrefix(e.ID, localPrefix) || e.Time.UnixMilli() >= oldest {
				kept = append(kept, e)
			}
		}
		r.events = kept
		r.keep = len(r.events)
		r.back, r.backDone = token, token == ""
		s.markDirty(room)
	})
}
