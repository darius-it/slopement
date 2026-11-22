package matrix

import (
	"context"
	"sort"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"slopement/format"
	"slopement/model"
)

// Searching messages is two searches. The app's own: every message it holds
// — the cached timelines and what was paged in, decrypted, so the rooms that
// are encrypted are searched too — matched in memory, at once, never leaving
// the device. And the server's, for the rooms that are not encrypted (it can
// read those), which reaches back past what is held. A hit found by both is
// one hit.
//
// The index is the timelines themselves: the cache keeps them sealed on
// disk, which a searchable copy of the plain text would undo.

// searchPage is how many events a page of history asks for when searching
// back: bigger than scrolling's, since a search wants many.
const searchPage = 100

// searchLimit is how many hits a search gives.
const searchLimit = 200

// plainBody is what a message says, as plain text.
func plainBody(e model.Event) string {
	if e.Formatted != "" {
		return format.Plain(format.ParseHTML(e.Formatted))
	}
	return e.Body
}

// searchText is what a search reads of a message: its words and the names
// of the files it carries (searching "report.pdf" finds it).
func searchText(e model.Event) string {
	text := plainBody(e)
	for _, m := range e.Media {
		if m.Name != "" && !strings.Contains(text, m.Name) {
			text += " " + m.Name
		}
	}
	return text
}

// searchable says an event is a message a search looks at.
func searchable(e model.Event) bool {
	switch e.Kind {
	case model.EventSystem, model.EventUndecryptable:
		return false
	}
	return !strings.HasPrefix(e.ID, localPrefix) // not one still being sent
}

// matches says a message fits a query: its filters, and — when words is set
// — every word of it. roomName and alias are the room's, for in:.
func (s *Source) matches(q model.SearchQuery, roomName, alias string, e model.Event, words bool) bool {
	return q.Match(searchText(e), roomName, alias, e.Sender, s.nameOf(e.Sender), e, words)
}

// localHits are the messages the app holds that fit, newest first.
func (s *Source) localHits(q model.SearchQuery) []model.SearchHit {
	var out []model.SearchHit
	for rid, r := range s.rooms {
		if r.space || (q.Room != "" && rid != q.Room) {
			continue
		}
		for _, e := range r.events {
			if searchable(e) && s.matches(q, r.name, r.alias, e, true) {
				out = append(out, model.SearchHit{Room: rid, Event: e.ID, Sender: e.Sender, Time: e.Time, Body: plainBody(e)})
			}
		}
	}
	sortHits(out)
	return out
}

func sortHits(h []model.SearchHit) {
	sort.SliceStable(h, func(i, j int) bool {
		if !h[i].Time.Equal(h[j].Time) {
			return h[i].Time.After(h[j].Time)
		}
		return h[i].Event < h[j].Event
	})
}

// Search looks for messages: what the app holds comes at once, and the
// server's answer for the rooms that are not encrypted follows. UI thread.
func (s *Source) Search(q model.SearchQuery, done func(hits []model.SearchHit, final bool, err string)) {
	if q.Empty() {
		done(nil, true, "")
		return
	}
	local := s.localHits(q)
	if len(local) > searchLimit {
		local = local[:searchLimit]
	}
	words := strings.TrimSpace(q.Words)
	var rooms []id.RoomID
	for rid, r := range s.rooms {
		if r.space || r.encrypted || (q.Room != "" && rid != q.Room) {
			continue
		}
		if q.In != "" {
			in := strings.ToLower(q.In)
			if !strings.Contains(strings.ToLower(r.name), in) && !strings.Contains(strings.ToLower(r.alias), in) {
				continue
			}
		}
		rooms = append(rooms, id.RoomID(rid))
	}
	if s.cli == nil || words == "" || len(rooms) == 0 {
		done(local, true, "") // nothing the server could add
		return
	}
	done(local, false, "")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := s.cli.Search(ctx, &mautrix.ReqSearch{
			SearchTerm: words, OrderBy: "recent", Keys: []string{"content.body"},
			Filter: &mautrix.FilterPart{Rooms: rooms, Limit: searchLimit},
		})
		s.post(func() {
			if err != nil || resp == nil {
				done(local, true, "The server could not be searched: "+plain(err))
				return
			}
			have := map[string]bool{}
			for _, h := range local {
				have[h.Event] = true
			}
			all := append([]model.SearchHit(nil), local...)
			for _, res := range resp.Results {
				if res == nil || res.Event == nil || res.Event.Type != event.EventMessage {
					continue
				}
				evt := res.Event
				if have[evt.ID.String()] {
					continue
				}
				_ = evt.Content.ParseRaw(evt.Type)
				if c := evt.Content.AsMessage(); c == nil || c.NewContent != nil {
					continue // an edit is not a message of its own
				}
				e := textEvent(s.me.ID, evt)
				r := s.rooms[evt.RoomID.String()]
				if r == nil || !s.matches(q, r.name, r.alias, e, false) {
					continue
				}
				have[e.ID] = true
				all = append(all, model.SearchHit{Room: r.id, Event: e.ID, Sender: e.Sender, Time: e.Time, Body: plainBody(e)})
			}
			sortHits(all)
			if len(all) > searchLimit {
				all = all[:searchLimit]
			}
			done(all, true, "")
		})
	}()
}

var _ model.Searcher = (*Source)(nil)

// PageBack pages a room's older history in, a page at a time, up to pages
// pages: each is fetched, decrypted with the keys there are (or looked for in
// the backup, as any waiting message is) and placed in the timeline, where a
// search then finds it. It stops at the room's start, and when another page
// of the room is already on its way it waits for that one.
func (s *Source) PageBack(room string, pages int, progress func(done, more bool)) {
	if progress == nil {
		progress = func(bool, bool) {}
	}
	if s.cli == nil {
		progress(true, false)
		return
	}
	go func() {
		more := true
		for i := 0; i < pages && more; i++ {
			type start struct {
				oldest, token string
				stop, busy    bool
			}
			var st start
			for try := 0; try < 25; try++ { // a page already on its way: wait for it
				ch := make(chan start, 1)
				s.post(func() {
					r, ok := s.rooms[room]
					switch {
					case !ok || r.backDone:
						ch <- start{stop: true}
						return
					case r.backBusy:
						ch <- start{busy: true}
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
						ch <- start{stop: true}
						return
					}
					r.backBusy = true
					ch <- start{oldest: oldest, token: r.back}
				})
				st = <-ch
				if !st.busy {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if st.stop || st.busy {
				more = !st.stop
				break
			}
			s.pageBack(room, st.oldest, st.token, min(searchPage, seekPage)) // returns when the page is in
			res := make(chan bool, 1)
			s.post(func() {
				r, ok := s.rooms[room]
				m := ok && !r.backDone
				res <- m
				progress(false, m)
			})
			more = <-res
		}
		s.post(func() { progress(true, more) })
	}()
}

var _ model.DeepSearch = (*Source)(nil)
