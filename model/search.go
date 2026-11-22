package model

import (
	"strings"
	"time"
)

// SearchQuery is a message search: words, and the filters that narrow them.
// It is typed as one line — `lunch in:design from:kai has:image
// before:2026-03-01 after:2026-01-01 mentions:me` — which ParseSearch reads
// and Text writes back, so the filter chips of the search screen and the
// typed tokens are the same thing.
type SearchQuery struct {
	Words    string    // what is searched for (every word must be in the message)
	In       string    // part of a room's name or address
	From     string    // part of a person's name or address
	Has      []string  // image, file, link, gif, video
	Before   time.Time // sent before the start of this day
	After    time.Time // sent on or after the start of this day
	Mentions bool      // mentions the user
	// Room, when set, limits the search to that one room (by ID): the
	// in-chat find. It is not one of the typed tokens.
	Room string
}

// Empty says there is nothing to search for.
func (q SearchQuery) Empty() bool {
	return strings.TrimSpace(q.Words) == "" && q.In == "" && q.From == "" && len(q.Has) == 0 &&
		q.Before.IsZero() && q.After.IsZero() && !q.Mentions
}

// searchDay is the date format of before: and after:.
const searchDay = "2006-01-02"

// hasKinds are what has: takes.
var hasKinds = map[string]bool{"image": true, "file": true, "link": true, "gif": true, "video": true}

// ParseSearch reads a typed search. A token it does not know (an unknown
// has: kind, a date that is not a date) is kept as a word, so what was typed
// is never silently dropped.
func ParseSearch(text string) SearchQuery {
	var q SearchQuery
	var words []string
	for _, tok := range strings.Fields(text) {
		key, val, ok := strings.Cut(tok, ":")
		low := strings.ToLower(key)
		if !ok || val == "" {
			words = append(words, tok)
			continue
		}
		switch low {
		case "in":
			q.In = val
		case "from":
			q.From = val
		case "has":
			if kind := strings.ToLower(val); hasKinds[kind] {
				q.Has = appendUnique(q.Has, kind)
			} else {
				words = append(words, tok)
			}
		case "before", "after":
			d, err := time.ParseInLocation(searchDay, val, time.Local)
			switch {
			case err != nil:
				words = append(words, tok)
			case low == "before":
				q.Before = d
			default:
				q.After = d
			}
		case "mentions":
			if strings.EqualFold(val, "me") {
				q.Mentions = true
			} else {
				words = append(words, tok)
			}
		default:
			words = append(words, tok) // "10:30" and the like are words
		}
	}
	q.Words = strings.Join(words, " ")
	return q
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// Text is the query as one typed line.
func (q SearchQuery) Text() string {
	var parts []string
	if w := strings.TrimSpace(q.Words); w != "" {
		parts = append(parts, w)
	}
	if q.In != "" {
		parts = append(parts, "in:"+q.In)
	}
	if q.From != "" {
		parts = append(parts, "from:"+q.From)
	}
	for _, h := range q.Has {
		parts = append(parts, "has:"+h)
	}
	if !q.After.IsZero() {
		parts = append(parts, "after:"+q.After.Format(searchDay))
	}
	if !q.Before.IsZero() {
		parts = append(parts, "before:"+q.Before.Format(searchDay))
	}
	if q.Mentions {
		parts = append(parts, "mentions:me")
	}
	return strings.Join(parts, " ")
}

// HasKind says the query asks for a kind (has:image).
func (q SearchQuery) HasKind(kind string) bool {
	for _, h := range q.Has {
		if h == kind {
			return true
		}
	}
	return false
}

// Toggle returns the query with a has: kind switched on or off (what a chip
// does).
func (q SearchQuery) Toggle(kind string) SearchQuery {
	if q.HasKind(kind) {
		var kept []string
		for _, h := range q.Has {
			if h != kind {
				kept = append(kept, h)
			}
		}
		q.Has = kept
	} else {
		q.Has = append(append([]string(nil), q.Has...), kind)
	}
	return q
}

// Match says a message fits the query's filters and — when words is true —
// every one of its words. text is what the message says as plain text (with
// its files' names); the room's name and address and the sender's address and
// name are for in: and from:.
func (q SearchQuery) Match(text, roomName, roomAlias, sender, senderName string, e Event, words bool) bool {
	body := strings.ToLower(text)
	if words {
		for _, w := range strings.Fields(strings.ToLower(q.Words)) {
			if !strings.Contains(body, w) {
				return false
			}
		}
	}
	if q.In != "" {
		in := strings.ToLower(q.In)
		if !strings.Contains(strings.ToLower(roomName), in) && !strings.Contains(strings.ToLower(roomAlias), in) {
			return false
		}
	}
	if q.From != "" {
		from := strings.ToLower(q.From)
		if !strings.Contains(strings.ToLower(sender), from) && !strings.Contains(strings.ToLower(senderName), from) {
			return false
		}
	}
	if !q.Before.IsZero() && !e.Time.Before(q.Before) {
		return false
	}
	if !q.After.IsZero() && e.Time.Before(q.After) {
		return false
	}
	if q.Mentions && !e.Mentions {
		return false
	}
	for _, h := range q.Has {
		if !hasKind(e, body, h) {
			return false
		}
	}
	return true
}

// hasKind says a message has a kind of thing in it (has:image).
func hasKind(e Event, lowerBody, kind string) bool {
	media := func(k MediaKind) bool {
		for _, m := range e.Media {
			if m.Kind == k {
				return true
			}
		}
		return false
	}
	switch kind {
	case "image":
		return media(MediaImage) || media(MediaSticker)
	case "gif":
		return media(MediaGIF) || strings.Contains(lowerBody, ".gif")
	case "video":
		return media(MediaVideo)
	case "file":
		return media(MediaFile)
	case "link":
		return strings.Contains(lowerBody, "http://") || strings.Contains(lowerBody, "https://")
	}
	return false
}

// SearchHit is a message found.
type SearchHit struct {
	Room, Event string
	Sender      string // the MXID
	Time        time.Time
	Body        string // what it says, as plain text
}

// DeepSearch is what a Source may offer to search further back than it
// holds: PageBack pages a room's older messages in (decrypting them: it is how
// an encrypted room's old messages become searchable at all) up to a number
// of pages, calling progress on the UI thread after each (done false) and once
// at the end (done true); more says the room has older messages still.
type DeepSearch interface {
	PageBack(room string, pages int, progress func(done, more bool))
}

// Searcher is what a Source may offer: searching messages. done is called on
// the UI thread, first with what the app already holds (at once, final
// false), then — for a source that can ask its server about the rooms that
// are not encrypted — with everything found (final true). err is a quiet
// line when the server could not be asked; the local hits stand.
type Searcher interface {
	Search(q SearchQuery, done func(hits []SearchHit, final bool, err string))
}
